#!/usr/bin/env bash
# Network e2e: SSH, telnet, TCP and UDP through a gateway pool to targets the
# server cannot reach, directly and through `werbot connect`; then the werbot
# login, ls, ssh and ssh-config commands (cli.sh).
# Expects postgres, server, target, router, echo and dns already up (see
# `make e2e-network`). Requires docker, curl, jq, ssh, ssh-keygen, go, nc, dig.
set -euo pipefail

cd "$(dirname "$0")/../.."
compose() { docker compose -p werbot-e2e -f deploy/e2e/docker-compose.yaml "$@"; }

API=http://localhost:8080
TMP=$(mktemp -d)
connect_pid=
login_pid=
cleanup() {
  local rc=$?
  set +e # a failing teardown must not skip the rest or replace the exit code
  [[ -z "$connect_pid" ]] || kill "$connect_pid"
  [[ -z "$login_pid" ]] || kill "$login_pid"
  ((rc == 0)) || { cat "$TMP/connect.log" 2>/dev/null >&2; compose --profile gateway logs --tail 100 >&2; }
  compose --profile gateway down -v
  rm -rf "$TMP"
  exit "$rc"
}
trap cleanup EXIT

# rpc Service/Method JSON: Connect unary call with the session cookie.
rpc() {
  local body
  body=$(curl -sS --fail-with-body -b "$TMP/cookies" -c "$TMP/cookies" -H 'Content-Type: application/json' \
    -d "$2" "$API/werbot.v1.$1") || {
    echo "rpc $1 failed: $body" >&2
    return 1
  }
  printf '%s\n' "$body"
}

# wait_for SECONDS CMD...: retry CMD once a second until it succeeds.
wait_for() {
  local deadline=$((SECONDS + $1))
  shift
  until "$@" >/dev/null 2>&1; do
    if ((SECONDS >= deadline)); then
      echo "timeout: $*" >&2
      return 1
    fi
    sleep 1
  done
}

# pool_online: the e2e pool and its single instance are online.
pool_online() {
  rpc GatewayService/ListGateways '{}' | jq -e '.gateways[] | select(.name == "e2e")
    | .online == true and (.instances | length) == 1 and .instances[0].online == true'
}

instance_id() {
  rpc GatewayService/ListGateways '{}' | jq -r '.gateways[] | select(.name == "e2e") | .instances[0].id'
}

# gw_ssh ARGS...: ssh to the gateway's SSH port with the e2e key.
gw_ssh() {
  ssh -i "$TMP/id" -p 2222 -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=5 \
    -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR "$@"
}

ssh_target() { gw_ssh e2e+target@localhost hostname; }

# create_bound NAME TYPE CONFIG: a resource bound to the e2e user; prints its id.
create_bound() {
  local id
  # || return: errexit is off inside $(create_bound ...)
  id=$(rpc ResourceService/CreateResource "$(jq -nc --arg n "$1" --arg t "$2" --argjson c "$3" '{name: $n, type: $t, config: $c}')" | jq -er .resource.id) || return
  rpc AccessService/CreateBinding "$(jq -nc --arg u "$user" --arg r "$id" '{subjectType: "user", subjectId: $u, resourceId: $r}')" >/dev/null || return
  echo "$id"
}

# set_password ID: the resource's credential is the password "e2e-password".
set_password() {
  # value is bytes: base64 of "e2e-password"
  rpc ResourceService/SetCredential "$(jq -nc --arg id "$1" '{resourceId: $id, kind: "password", value: "ZTJlLXBhc3N3b3Jk"}')" >/dev/null
}

# check_ssh: the hostname seen through the gateway is the target's.
check_ssh() {
  wait_for 60 ssh_target
  local got
  got=$(ssh_target)
  echo "target hostname via gateway: $got"
  [[ "$got" == "$want" ]] || { echo "want hostname $want, got $got" >&2; exit 1; }
}

gateway_ready() { [[ "$(curl -fsS localhost:8081/readyz)" == ok ]]; }

# tunnel_up: captured first, so grep -q cannot SIGPIPE curl under pipefail.
tunnel_up() {
  local m
  m=$(curl -fsS localhost:8081/metrics)
  grep -qx 'werbot_gateway_tunnel_connected 1' <<<"$m"
}

# want_networks SERVICE JSON: the container joins exactly these networks.
want_networks() {
  local got
  got=$(networks "$1")
  [[ "$got" == "$2" ]] || { echo "$1 networks: want $2, got $got" >&2; exit 1; }
}

networks() { docker inspect -f '{{json .NetworkSettings.Networks}}' "$(compose ps -q "$1")" | jq -c 'keys'; }

# OpenBSD nc (CI) needs -N to close its write side at stdin EOF; macOS nc does it by default.
NC_EOF=$(nc -h 2>&1 | grep -q -- '-N.*[Ss]hutdown' && echo -N || true)

# type_probe EOL: types the probe and exit after the autologin, then keeps stdin
# open for the reply: the user's EOF closes a telnet session at once.
type_probe() {
  sleep 2
  printf '%s%sexit%s' "$probe" "$1" "$1"
  sleep 5
}

# router_shell: runs the probe in a telnet session through the SSH bridge; the output goes to $out.
router_shell() { out=$(type_probe $'\n' | gw_ssh -tt e2e+router@localhost 2>&1); }

# router_recording: the only router session is a terminal session whose recording verifies.
router_recording() {
  local sid
  sid=$(rpc SessionService/ListSessions "$(jq -nc --arg r "$router" '{resourceId: $r}')" |
    jq -er '.sessions | select(length == 1) | .[0] | select(.kind == "terminal") | .id') || return
  rpc SessionService/VerifyRecording "$(jq -nc --arg id "$sid" '{sessionId: $id}')" | jq -e .verified
}

# target_denied: the refused echo:8 forward is audited as target_not_allowed.
target_denied() {
  rpc AuditService/ListEvents '{"action":"session.deny"}' | jq -e --arg r "$echo_id" \
    'any(.events[]; .targetId == $r and .data.reason == "target_not_allowed" and .data.target == "echo:8")'
}

# health_settled: the TCP and telnet targets are healthy; udp has no health check.
health_settled() {
  rpc ResourceService/ListResources '{}' | jq -e '[.resources[] | {(.name): .health.state}] | add
    | .echo == "STATE_HEALTHY" and .router == "STATE_HEALTHY" and .dns == "STATE_UNKNOWN"'
}

echo "== server ready"
wait_for 120 curl -fsS "$API/readyz"

echo "== setup admin and SSH key"
rpc AuthService/Setup '{"email":"e2e@example.com","login":"e2e","name":"E2E","password":"e2e-password-123","organizationName":"E2E"}' >/dev/null
ssh-keygen -q -t ed25519 -N '' -f "$TMP/id"
rpc SSHKeyService/AddSSHKey "$(jq -nc --arg k "$(cat "$TMP/id.pub")" '{name: "e2e", publicKey: $k}')" >/dev/null

echo "== gateway pool and join token"
gw=$(rpc GatewayService/CreateGateway '{"name":"e2e"}' | jq -r .gateway.id)
token=$(rpc GatewayService/CreateJoinToken "$(jq -nc --arg id "$gw" '{gatewayId: $id, maxUses: 1}')" | jq -r .token)
JOIN_TOKEN=$token compose --profile gateway up -d gateway
wait_for 60 pool_online
first=$(instance_id)

echo "== gateway readyz and metrics"
wait_for 30 gateway_ready
wait_for 30 tunnel_up

echo "== resource, credential, binding"
user=$(rpc AuthService/GetSession '{}' | jq -r .user.id)
res=$(create_bound target ssh '{"@type":"type.googleapis.com/werbot.connector.ssh.v1.Config","host":"target","port":2222,"login":"werbot"}')
set_password "$res"

echo "== SSH through the gateway"
want=$(compose exec -T target hostname)
check_ssh

echo "== gateway restart keeps its identity"
compose restart gateway
# SSH first: it needs the new process's live tunnel, so the pool state
# read after it is fresh, not the pre-restart online flag.
check_ssh
wait_for 60 pool_online
[[ "$(instance_id)" == "$first" ]] || { echo "instance changed: $first -> $(instance_id)" >&2; exit 1; }

echo "== topology"
want_networks server '["werbot-e2e_public"]'
want_networks target '["werbot-e2e_private"]'
for svc in router echo dns; do want_networks "$svc" '["werbot-e2e_private"]'; done

echo "== telnet, tcp and udp resources"
created=$SECONDS
router=$(create_bound router telnet '{"@type":"type.googleapis.com/werbot.connector.telnet.v1.Config","host":"router","login":"werbot"}')
set_password "$router"
echo_id=$(create_bound echo tcp '{"@type":"type.googleapis.com/werbot.connector.tcp.v1.Config","host":"echo","ports":[{"port":7}]}')
create_bound dns udp '{"@type":"type.googleapis.com/werbot.connector.udp.v1.Config","host":"dns","ports":[{"port":53}]}' >/dev/null

# Shell probes print a computed marker, so the echo of the typed command cannot match.
# shellcheck disable=SC2016 # $((6*7)) is expanded by the router's shell
probe='echo werbot-$((6*7))'

echo "== telnet through the SSH bridge"
# the gateway refuses the router until the new policy snapshot reaches it
wait_for 60 router_shell || { echo "ssh e2e+router failed: $out" >&2; exit 1; }
grep -q werbot-42 <<<"$out" || { echo "no werbot-42 in the router session: $out" >&2; exit 1; }
! grep -q e2e-password <<<"$out" || { echo "the router password reached the user: $out" >&2; exit 1; }
wait_for 60 router_recording

echo "== werbot connect"
go build -o "$TMP/werbot" ./cmd/werbot
"$TMP/werbot" connect --gateway localhost:2222 --login e2e -i "$TMP/id" --known-hosts "$TMP/known_hosts" \
  echo router dns >"$TMP/connect.log" 2>&1 &
connect_pid=$!
wait_for 30 grep -qxF 'echo → 127.0.0.1:10007 (tcp)' "$TMP/connect.log"
wait_for 30 grep -qxF 'router → 127.0.0.1:10023 (tcp)' "$TMP/connect.log"
wait_for 30 grep -qxF 'dns → 127.0.0.1:10053 (udp)' "$TMP/connect.log"

echo "== tcp and telnet through werbot connect"
# shellcheck disable=SC2086 # NC_EOF is empty or one flag
got=$(printf werbot-probe | nc $NC_EOF 127.0.0.1 10007)
[[ "$got" == werbot-probe ]] || { echo "tcp echo: want werbot-probe, got $got" >&2; exit 1; }
out=$(type_probe $'\r\n' | nc 127.0.0.1 10023)
grep -aq werbot-42 <<<"$out" || { echo "no werbot-42 in the telnet stream: $out" >&2; exit 1; }

echo "== udp through werbot connect"
for _ in 1 2; do # each dig sends from a new client port: a new flow
  got=$(dig +short @127.0.0.1 -p 10053 probe.werbot.test)
  [[ "$got" == 10.0.0.1 ]] || { echo "dns: want 10.0.0.1, got $got" >&2; exit 1; }
done

echo "== port outside the resource is refused"
# -v: ssh logs the channel refusal at INFO, below gw_ssh's LogLevel=ERROR
if out=$(gw_ssh -v -W echo:8 e2e+echo@localhost </dev/null 2>&1); then
  echo "ssh -W echo:8 succeeded: $out" >&2
  exit 1
fi
grep -q 'open failed: administratively prohibited' <<<"$out" || { echo "ssh -W echo:8: $out" >&2; exit 1; }
wait_for 60 target_denied

echo "== resource health"
wait_for $((created + 180 - SECONDS)) health_settled || {
  rpc ResourceService/ListResources '{}' | jq -c '.resources[] | {name, health}' >&2
  exit 1
}

# shellcheck source=deploy/e2e/cli.sh
source deploy/e2e/cli.sh

echo "PASS"
