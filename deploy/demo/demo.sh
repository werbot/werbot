#!/usr/bin/env bash
# Demo: an AI agent works on "production" with no key and no password.
# Usage: demo.sh [--ci]   (--ci does not wait for Enter between steps)
# Needs docker compose, curl, jq, ssh and a werbot CLI (WERBOT_BIN, default "werbot").
# Set WERBOT_VERSION to a published image tag (default v0.2.0).
# The stack is throwaway: every secret is generated here, `down -v` runs on exit.
set -euo pipefail

cd "$(dirname "$0")"
CI=false
[[ "${1:-}" != --ci ]] || CI=true
BIN=${WERBOT_BIN:-werbot}
API=http://localhost:18090
TMP=$(mktemp -d)
login_pid=
wait_pid=

export WERBOT_VERSION=${WERBOT_VERSION:-v0.2.0}
WERBOT_SECRET_KEY=$(openssl rand -base64 32)
POSTGRES_PASSWORD=$(openssl rand -hex 24)
DEMO_TARGET_PASSWORD=$(openssl rand -hex 12)
OWNER_PASSWORD=$(openssl rand -hex 16)
export WERBOT_SECRET_KEY POSTGRES_PASSWORD DEMO_TARGET_PASSWORD

compose() { docker compose --env-file /dev/null -f ../docker-compose.yaml -f docker-compose.yaml "$@"; }

cleanup() {
  local rc=$?
  set +e # a failing teardown must not skip the rest or replace the exit code
  [[ -z "$login_pid" ]] || kill "$login_pid"
  [[ -z "$wait_pid" ]] || kill "$wait_pid"
  ((rc == 0)) || compose logs --tail 50 werbot >&2
  compose down -v --remove-orphans
  rm -rf "$TMP"
  exit "$rc"
}
trap cleanup EXIT

# rpc Service/Method JSON: Connect unary call with the session cookie.
rpc() {
  local body
  body=$(curl -sS --fail-with-body -b "$TMP/cookies" -c "$TMP/cookies" -H 'Content-Type: application/json' \
    -d "$2" "$API/werbot.v1.$1") || { echo "rpc $1 failed: $body" >&2; return 1; }
  printf '%s\n' "$body"
}

wait_for() {
  local deadline=$((SECONDS + $1))
  shift
  until "$@" >/dev/null 2>&1; do
    ((SECONDS < deadline)) || { echo "timeout: $*" >&2; return 1; }
    sleep 1
  done
}

step() {
  printf '\n== %s\n' "$1"
  $CI || read -r -p "Enter to run... " _
}

# agent CMD...: the command runs as the AI agent "demo", with its own key and certificate.
# sed drops the run banner and the profile warning of werbot (stderr); the rest is the command's own output.
agent() {
  WERBOT_HOME="$TMP/home" "$BIN" agent run --agent demo --reason "demo: deploy a fix" -- "$@" 2>&1 |
    sed -E '/^(warning: the agent |Agent run |MCP: )/d'
}
human() { WERBOT_HOME="$TMP/home" "$BIN" "$@"; }

login_done() { ! kill -0 "$login_pid"; }
pending_request() { human requests --to-approve | awk '$2 == "pending" {print $1; exit}' | grep .; }

step "Start the stack (werbot-demo)"
compose up -d --wait

step "Set up the organization: owner, SSH pool, two targets"
wait_for 60 curl -fsS "$API/readyz"
rpc AuthService/Setup "$(jq -nc --arg p "$OWNER_PASSWORD" '{email: "demo@example.com", login: "demo", name: "Demo", password: $p, organizationName: "Demo"}')" >/dev/null
pool_online() { rpc GatewayService/ListGateways '{}' | jq -e '.gateways[0].online'; } # the embedded gateway enrols after start
wait_for 60 pool_online
pool=$(rpc GatewayService/ListGateways '{}' | jq -er '.gateways[0].id')
rpc GatewayService/UpdateGateway "$(jq -nc --arg id "$pool" '{id: $id, sshAddress: "localhost:12222", wsUrl: ""}')" >/dev/null
user=$(rpc AuthService/GetSession '{}' | jq -er .user.id)
pass64=$(printf %s "$DEMO_TARGET_PASSWORD" | base64)
for env in prod stage; do
  res=$(rpc ResourceService/CreateResource "$(jq -nc --arg n "$env-web" --arg e "$env" \
    '{name: $n, type: "ssh", labels: {env: $e}, config: {"@type": "type.googleapis.com/werbot.connector.ssh.v1.Config", host: $n, port: 2222, login: "werbot"}}')" | jq -er .resource.id)
  rpc ResourceService/SetCredential "$(jq -nc --arg id "$res" --arg v "$pass64" '{resourceId: $id, kind: "password", value: $v}')" >/dev/null
  rpc AccessService/CreateBinding "$(jq -nc --arg u "$user" --arg r "$res" '{subjectType: "user", subjectId: $u, resourceId: $r}')" >/dev/null
done
rpc PolicyService/CreateCommandRule '{"name":"prod-write-approval","subjects":{"allAgents":true},"resourceSelector":{"labels":{"env":"prod"}},"classes":["write"],"effect":"approval"}' >/dev/null

step "Log in the CLI (device flow, approved here)"
human login --device --server "$API" >"$TMP/login.log" 2>&1 &
login_pid=$!
wait_for 30 grep -q 'enter code ' "$TMP/login.log"
code=$(sed -n 's/.*enter code \([^ ]*\).*/\1/p' "$TMP/login.log")
rpc OAuthConsentService/ApproveDeviceCode "$(jq -nc --arg c "$code" '{userCode: $c}')" >/dev/null
wait_for 30 login_done
wait "$login_pid"
login_pid=

step "1. The agent reads prod-web: allowed"
agent ssh prod-web 'ls /etc' | sed -n 1,3p

step "2. The agent deletes the site: refused by the baseline"
if out=$(agent ssh prod-web 'rm -rf ~/site'); then echo "unexpected success: $out" >&2; exit 1; fi
echo "$out"

step "3. The agent changes prod-web: waits for approval, a person approves"
agent ssh prod-web 'touch ~/deployed' &
wait_pid=$!
wait_for 60 pending_request
id=$(pending_request)
human approve "$id"
wait "$wait_pid"
wait_pid=

step "4. The agent reads a private key: refused, the sponsor gets an alert"
if out=$(agent ssh prod-web 'cat ~/.ssh/id_ed25519'); then echo "unexpected success: $out" >&2; exit 1; fi
echo "$out"

step "5. The transcript"
human agents
run=$(rpc AgentService/ListRuns '{}' | jq -er '.runs[0].id')
echo "Open $API/agents/runs/$run (owner login: demo, password: $OWNER_PASSWORD)"
$CI || read -r -p "Enter to stop the stack... " _
