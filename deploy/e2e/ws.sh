# shellcheck shell=bash disable=SC2154 # gw, res, want and the helpers come from run.sh
# WebSocket input and ssh -R e2e (W1–W4): OpenSSH and werbot connect through
# the gateway's WebSocket input behind nginx (edge), then a remote forward.
# Sourced by run.sh after cli.sh: it uses run.sh's helpers (rpc, wait_for,
# compose, gw_ssh), $TMP, $gw, $res, $want, $NC_EOF and $TMP/werbot. vnet.sh
# uses its bg list, vnet_exec and the vnet container it starts.

TLS=deploy/e2e/.bin/tls # mounted into edge and vnet
bg=()                   # host processes; the EXIT trap ends them

# e2e_down ends the host processes and the vnet container, then returns the
# exit code, so run.sh's cleanup tears the stack down and exits with it.
e2e_down() {
  local rc=$?
  set +e
  ((${#bg[@]} == 0)) || kill "${bg[@]}" 2>/dev/null
  ((rc == 0)) || {
    compose --profile vnet exec -T vnet sh -c 'tail -n 50 /tmp/*.log' >&2
    compose --profile vnet logs --tail 100 vnet edge >&2
  }
  compose --profile vnet rm -fsv vnet
  rm -rf "$TLS"
  return "$rc"
}
trap 'e2e_down; cleanup' EXIT

# vnet_exec ARGS...: a command in the vnet container.
vnet_exec() { compose --profile vnet exec -T vnet "$@"; }

update_gateway() {
  rpc GatewayService/UpdateGateway "$(jq -nc --arg id "$gw" --arg s "$1" --arg w "$2" '{id: $id, sshAddress: $s, wsUrl: $w}')" >/dev/null
}

echo "== W1: edge (nginx) in front of the WebSocket input"
mkdir -p "$TLS"
openssl req -x509 -newkey ed25519 -nodes -days 1 -subj /CN=localhost \
  -addext subjectAltName=DNS:localhost,DNS:edge -keyout "$TLS/key.pem" -out "$TLS/cert.pem" 2>/dev/null
compose up -d --wait edge
update_gateway localhost:2222 wss://localhost:8448/v1/ssh
rpc GatewayService/ListGateways '{}' | jq -e '.gateways[] | select(.name == "e2e") | .wsUrl == "wss://localhost:8448/v1/ssh"' >/dev/null

echo "== W2: OpenSSH through werbot proxy over ws://"
ws_ssh() {
  ssh -o ProxyCommand="$TMP/werbot proxy ws://localhost:8088/v1/ssh" -o UserKnownHostsFile="$TMP/kh_ws" \
    -o StrictHostKeyChecking=accept-new -o IdentitiesOnly=yes -o BatchMode=yes -i "$TMP/id" e2e+target@werbot-pool hostname </dev/null
}
got=$(ws_ssh)
[[ "$got" == "$want" ]] || { echo "ssh over ws://: want hostname $want, got $got" >&2; exit 1; }

echo "== W3: werbot connect over wss:// from the vnet container"
compose --profile vnet up -d --build --wait vnet
compose --profile vnet cp "$TMP/id" vnet:/root/id
compose --profile vnet exec -d vnet sh -c 'SSL_CERT_FILE=/tls/cert.pem werbot connect --gateway wss://edge:8448/v1/ssh \
  --login e2e -i /root/id --known-hosts /root/kh_ws echo dns >/tmp/connect.log 2>&1'
wait_for 30 vnet_exec grep -qxF 'echo → 127.0.0.1:10007 (tcp)' /tmp/connect.log
wait_for 30 vnet_exec grep -qxF 'dns → 127.0.0.1:10053 (udp)' /tmp/connect.log
got=$(vnet_exec sh -c 'printf werbot-probe | nc -N 127.0.0.1 10007')
[[ "$got" == werbot-probe ]] || { echo "tcp echo over wss://: want werbot-probe, got $got" >&2; exit 1; }
got=$(vnet_exec dig +short @127.0.0.1 -p 10053 probe.werbot.test)
[[ "$got" == 10.0.0.1 ]] || { echo "dns over wss://: want 10.0.0.1, got $got" >&2; exit 1; }
# the gateway trusts edge's X-Forwarded-For: the client is the vnet container
vnet_ip=$(docker inspect -f '{{(index .NetworkSettings.Networks "werbot-e2e_public").IPAddress}}' "$(compose --profile vnet ps -q vnet)")
rpc SessionService/ListSessions "$(jq -nc --arg r "$echo_id" '{resourceId: $r}')" |
  jq -e --arg ip "$vnet_ip" 'any(.sessions[]; .clientIp == $ip) and all(.sessions[]; .clientIp != "172.30.0.10")' >/dev/null ||
  { echo "no echo session from $vnet_ip" >&2; exit 1; }
rpc AuditService/ListEvents '{"action":"session.start"}' | jq -e 'any(.events[]; .data.transport == "ws")' >/dev/null

echo "== W4: ssh -R"
fwd_ssh() { gw_ssh -N -o ExitOnForwardFailure=yes -R 18080:127.0.0.1:17007 e2e+target@localhost </dev/null; }
fwd_done() { ! kill -0 "$fwd_pid"; }
fwd_ssh &
fwd_pid=$!
bg+=("$fwd_pid")
wait_for 30 fwd_done
if wait "$fwd_pid"; then echo "ssh -R without remote_forwarding succeeded" >&2; exit 1; fi
# denials reach the server with the next heartbeat
fwd_denied() {
  rpc AuditService/ListEvents '{"action":"session.deny"}' | jq -e --arg r "$res" \
    'any(.events[]; .targetId == $r and .data.reason == "target_not_allowed")'
}
wait_for 60 fwd_denied

rpc ResourceService/UpdateResource "$(jq -nc --arg id "$res" '{id: $id, name: "target",
  config: {"@type": "type.googleapis.com/werbot.connector.ssh.v1.Config", host: "target", port: 2222, login: "werbot", remoteForwarding: true}}')" >/dev/null
# shellcheck disable=SC2086 # NC_EOF is empty or one flag
printf werbot-probe-r | nc $NC_EOF -l 127.0.0.1 17007 &
bg+=("$!")
# fwd_up (re)starts ssh -R until the gateway has the new snapshot, then reads
# the probe through the forward from the target.
fwd_up() {
  fwd_done && {
    fwd_ssh &
    fwd_pid=$!
    bg+=("$fwd_pid")
  }
  [[ "$(compose exec -T target nc -w 5 127.0.0.1 18080 </dev/null)" == werbot-probe-r ]]
}
wait_for 60 fwd_up
rpc SessionService/ListSessions "$(jq -nc --arg r "$res" '{resourceId: $r}')" |
  jq -e 'any(.sessions[]; .kind == "stream" and .target == "localhost:18080")' >/dev/null
