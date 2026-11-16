#!/usr/bin/env bash
# Network e2e: SSH through a gateway pool to a target the server cannot reach.
# Expects postgres, server and target already up (see `make e2e-network`).
# Requires docker, curl, jq, ssh, ssh-keygen.
set -euo pipefail

cd "$(dirname "$0")/../.."
compose() { docker compose -p werbot-e2e -f deploy/e2e/docker-compose.yaml "$@"; }

API=http://localhost:8080
TMP=$(mktemp -d)
trap 'compose --profile gateway down -v; rm -rf "$TMP"' EXIT

# rpc Service/Method JSON: Connect unary call with the session cookie.
rpc() {
  curl -fsS -b "$TMP/cookies" -c "$TMP/cookies" -H 'Content-Type: application/json' \
    -d "$2" "$API/werbot.v1.$1"
}

# wait_for SECONDS CMD...: retry CMD once a second until it succeeds.
wait_for() {
  local deadline=$((SECONDS + $1))
  shift
  until "$@" >/dev/null 2>&1; do
    if ((SECONDS >= deadline)); then
      echo "timeout: $*" >&2
      compose --profile gateway logs --tail 50 >&2
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

ssh_target() {
  ssh -i "$TMP/id" -p 2222 -o BatchMode=yes -o ConnectTimeout=5 \
    -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR \
    e2e+target@localhost hostname
}

# check_ssh: the hostname seen through the gateway is the target's.
check_ssh() {
  wait_for 60 ssh_target
  local got
  got=$(ssh_target)
  echo "target hostname via gateway: $got"
  [[ "$got" == "$want" ]] || { echo "want hostname $want, got $got" >&2; exit 1; }
}

networks() { docker inspect -f '{{json .NetworkSettings.Networks}}' "$(compose ps -q "$1")" | jq -c 'keys'; }

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
[[ "$(curl -fsS localhost:8081/readyz)" == ok ]]
curl -fsS localhost:8081/metrics | grep -qx 'werbot_gateway_tunnel_connected 1'

echo "== resource, credential, binding"
res=$(rpc ResourceService/CreateResource '{"name":"target","type":"ssh","config":{"@type":"type.googleapis.com/werbot.connector.ssh.v1.Config","host":"target","port":2222,"login":"werbot"}}' | jq -r .resource.id)
# value is bytes: base64 of "e2e-password"
rpc ResourceService/SetCredential "$(jq -nc --arg id "$res" '{resourceId: $id, kind: "password", value: "ZTJlLXBhc3N3b3Jk"}')" >/dev/null
user=$(rpc AuthService/GetSession '{}' | jq -r .user.id)
rpc AccessService/CreateBinding "$(jq -nc --arg u "$user" --arg r "$res" '{subjectType: "user", subjectId: $u, resourceId: $r}')" >/dev/null

echo "== SSH through the gateway"
want=$(compose exec -T target hostname)
check_ssh

echo "== gateway restart keeps its identity"
compose restart gateway
wait_for 60 pool_online
[[ "$(instance_id)" == "$first" ]] || { echo "instance changed: $first -> $(instance_id)" >&2; exit 1; }
check_ssh

echo "== topology"
[[ "$(networks server)" == '["werbot-e2e_public"]' ]]
[[ "$(networks target)" == '["werbot-e2e_private"]' ]]

echo "PASS"
