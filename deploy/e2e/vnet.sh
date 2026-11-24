# shellcheck shell=bash disable=SC2154,SC2016 # helpers come from run.sh and ws.sh; $ in single quotes expands in the container
# Preset and --vnet e2e (W5–W6): a resource made from a preset, then
# werbot connect --vnet in the Linux vnet container, as root under sudo's
# environment (W6) and as a user with CAP_NET_ADMIN from setcap (W6b).
# Sourced by run.sh after ws.sh: it uses run.sh's helpers (rpc, wait_for,
# gw_ssh), ws.sh's bg list, update_gateway and vnet_exec, and $TMP/werbot.

# web_up: the gateway has the snapshot with the web resource.
web_up() {
  local out
  out=$(printf 'GET / HTTP/1.0\r\n\r\n' | gw_ssh -W web:80 e2e+web@localhost)
  [[ "$out" == *werbot-probe ]]
}

echo "== W5: resource from a preset"
preset=$(rpc ResourcePresetService/CreateResourcePreset '{"name": "http", "baseType": "tcp", "displayName": "HTTP",
  "defaultPorts": [{"port": 80}], "category": "applications", "launchHint": "curl http://{host}:{port}"}' | jq -er .preset.id)
web=$(rpc ResourceService/CreateResource "$(jq -nc --arg p "$preset" '{name: "web", type: "tcp", presetId: $p,
  config: {"@type": "type.googleapis.com/werbot.connector.tcp.v1.Config", host: "echo", ports: [{port: 80}]}}')" |
  jq -er --arg p "$preset" 'select(.resource.presetId == $p) | .resource.id')
rpc AccessService/CreateBinding "$(jq -nc --arg u "$user" --arg r "$web" '{subjectType: "user", subjectId: $u, resourceId: $r}')" >/dev/null
wait_for 60 web_up
"$TMP/werbot" connect --gateway localhost:2222 --login e2e -i "$TMP/id" --known-hosts "$TMP/known_hosts" \
  web >"$TMP/connect_web.log" 2>&1 &
bg+=("$!")
wait_for 30 grep -qxF 'web → 127.0.0.1:10080 (tcp)' "$TMP/connect_web.log"
got=$(curl -fsS http://127.0.0.1:10080/)
[[ "$got" == werbot-probe ]] || { echo "preset resource: want werbot-probe, got $got" >&2; exit 1; }

# vnet_start LOG CMD: runs CMD (werbot connect --vnet) in the vnet container
# in the background; its pid goes to LOG.pid, its exit code to LOG.rc.
vnet_start() {
  compose --profile vnet exec -d vnet sh -c "$2 >$1 2>&1 & echo \$! >$1.pid; wait \$!; echo \$? >$1.rc"
  wait_for 60 vnet_exec grep -q '\.werbot → ' "$1" || { vnet_exec cat "$1" >&2; return 1; }
}

# vnet_stop LOG: Ctrl+C ends it with exit code 0 and removes werbot0.
vnet_stop() {
  vnet_exec sh -c "kill -INT \$(cat $1.pid)"
  wait_for 30 vnet_exec test -s "$1.rc"
  [[ "$(vnet_exec cat "$1.rc")" == 0 ]] || { echo "$1: exit code $(vnet_exec cat "$1.rc")" >&2; exit 1; }
  if vnet_exec ip link show werbot0 >/dev/null 2>&1; then echo "werbot0 left after $1" >&2; exit 1; fi
}

# in_prefix ADDR: ADDR is inside the default --vnet prefix 100.64.0.0/10.
in_prefix() { [[ "$1" =~ ^100\.([0-9]+)\.[0-9]+\.[0-9]+$ ]] && ((BASH_REMATCH[1] >= 64 && BASH_REMATCH[1] <= 127)); }

echo "== W6: werbot connect --vnet as root under sudo's environment"
vnet_exec test -c /dev/net/tun || { echo "W6: no /dev/net/tun in the vnet container; --vnet needs a TUN device (on OrbStack run W6 on CI)" >&2; exit 1; }
update_gateway gateway:2222 ""
vnet_exec su werbot -c 'werbot login --device --server http://server:8080' >"$TMP/vnet_login.log" 2>&1 &
login_pid=$!
wait_for 30 grep -q 'enter code ' "$TMP/vnet_login.log" || { cat "$TMP/vnet_login.log" >&2; exit 1; }
user_code=$(sed -n 's/.*enter code \([^ ]*\).*/\1/p' "$TMP/vnet_login.log")
rpc OAuthConsentService/ApproveDeviceCode "$(jq -nc --arg c "$user_code" '{userCode: $c}')" >/dev/null
wait_for 30 login_done
wait "$login_pid" || { echo "werbot login in vnet failed: $(cat "$TMP/vnet_login.log")" >&2; exit 1; }
login_pid=
# renewed under --vnet with root dropped: the new certificate must be the user's
vnet_exec sh -c 'rm /home/werbot/.werbot/*/id_ed25519-cert.pub'
vnet_start /tmp/vnet.log 'SUDO_USER=werbot SUDO_UID=$(id -u werbot) SUDO_GID=$(id -g werbot) werbot connect --vnet'
WEB=$(vnet_exec awk '$1 ~ /^web\./ {print $1}' /tmp/vnet.log)
DNS=$(vnet_exec awk '$1 ~ /^dns\./ {print $1}' /tmp/vnet.log)
addr=$(vnet_exec dig +short @100.64.0.2 "$WEB")
in_prefix "$addr" || { echo "dig $WEB: want an address in 100.64.0.0/10, got $addr" >&2; exit 1; }
got=$(vnet_exec dig +tcp +short @100.64.0.2 "$WEB")
[[ "$got" == "$addr" ]] || { echo "dig +tcp $WEB: want $addr, got $got" >&2; exit 1; }
out=$(vnet_exec dig @100.64.0.2 example.com)
grep -q 'status: REFUSED' <<<"$out" || { echo "dig example.com: want REFUSED, got $out" >&2; exit 1; }
vnet_exec sh -c 'cp /etc/resolv.conf /tmp/resolv.conf && printf "nameserver 100.64.0.2\n%s\n" "$(cat /tmp/resolv.conf)" >/etc/resolv.conf'
got=$(vnet_exec curl -fsS "http://$WEB/")
[[ "$got" == werbot-probe ]] || { echo "curl $WEB: want werbot-probe, got $got" >&2; exit 1; }
got=$(vnet_exec sh -c "dig +short @\$(dig +short @100.64.0.2 $DNS) probe.werbot.test")
[[ "$got" == 10.0.0.1 ]] || { echo "dig through $DNS: want 10.0.0.1, got $got" >&2; exit 1; }
got=$(vnet_exec sh -c 'stat -c %U /home/werbot/.werbot/*/id_ed25519-cert.pub')
[[ "$got" == werbot ]] || { echo "certificate owner: want werbot, got $got" >&2; exit 1; }
got=$(vnet_exec find /home/werbot/.werbot -user root)
[[ -z "$got" ]] || { echo "root-owned profile files: $got" >&2; exit 1; }
# Docker's resolver again: W6b resolves server before its responder runs
vnet_exec sh -c 'cat /tmp/resolv.conf >/etc/resolv.conf'
vnet_stop /tmp/vnet.log

echo "== W6b: werbot connect --vnet as a user with CAP_NET_ADMIN (setcap)"
vnet_exec sh -c 'cp /usr/local/bin/werbot /usr/local/bin/werbot-cap && setcap cap_net_admin+ep /usr/local/bin/werbot-cap'
# setpriv execs werbot-cap in place (su would turn Ctrl+C into its own exit code)
vnet_start /tmp/vnet_cap.log 'HOME=/home/werbot setpriv --reuid=werbot --regid=werbot --init-groups werbot-cap connect --vnet'
addr=$(vnet_exec dig +short @100.64.0.2 "$WEB")
in_prefix "$addr" || { echo "setcap: dig $WEB: want an address in 100.64.0.0/10, got $addr" >&2; exit 1; }
vnet_stop /tmp/vnet_cap.log
