# shellcheck shell=bash disable=SC2154 # gw, want and the helpers come from run.sh
# CLI e2e: werbot login (device flow), ls, ssh, ssh-config and revocation of
# the CLI session. Sourced by run.sh after its checks: it uses run.sh's
# helpers (rpc, wait_for), $API, $TMP, $gw, $want and the host build of
# werbot in $TMP/werbot.

# cli ARGS...: the host werbot client with its own profile home.
cli() { WERBOT_HOME="$TMP/home" "$TMP/werbot" "$@"; }

login_done() { ! kill -0 "$login_pid"; }

# openssh_target: OpenSSH through the config werbot ssh-config printed.
openssh_target() { ssh -F "$TMP/ssh_config" -o BatchMode=yes -o ConnectTimeout=5 target.localhost hostname </dev/null; }

echo "== werbot login --device"
rpc GatewayService/UpdateGateway "$(jq -nc --arg id "$gw" '{id: $id, sshAddress: "localhost:2222"}')" >/dev/null
cli login --device --server "$API" >"$TMP/login.log" 2>&1 &
login_pid=$!
wait_for 30 grep -q 'enter code ' "$TMP/login.log" || { cat "$TMP/login.log" >&2; exit 1; }
user_code=$(sed -n 's/.*enter code \([^ ]*\).*/\1/p' "$TMP/login.log")
rpc OAuthConsentService/ApproveDeviceCode "$(jq -nc --arg c "$user_code" '{userCode: $c}')" >/dev/null
wait_for 30 login_done
wait "$login_pid" || { echo "werbot login failed: $(cat "$TMP/login.log")" >&2; exit 1; }
login_pid=

echo "== werbot ls and werbot ssh"
cli ls | grep -qw target || { cli ls >&2; exit 1; }
got=$(cli ssh target -- hostname </dev/null)
[[ "$got" == "$want" ]] || { echo "werbot ssh: want hostname $want, got $got" >&2; exit 1; }

echo "== OpenSSH through werbot ssh-config"
cli ssh-config >"$TMP/ssh_config"
got=$(openssh_target)
[[ "$got" == "$want" ]] || { echo "ssh -F ssh_config: want hostname $want, got $got" >&2; exit 1; }

echo "== revoked CLI session"
sid=$(rpc CLISessionService/ListCLISessions '{}' | jq -er '.sessions | select(length == 1) | .[0].id')
rpc CLISessionService/RevokeCLISession "$(jq -nc --arg id "$sid" '{id: $id}')" >/dev/null
# the API refuses the revoked token before werbot ssh starts ssh
if out=$(cli ssh target -- hostname </dev/null 2>&1); then
  echo "werbot ssh after revoke succeeded: $out" >&2
  exit 1
fi
# the certificate is still inside its hour; the gateway refuses its session
rc=0
out=$(openssh_target 2>&1) || rc=$?
if ((rc != 255)) || ! grep -q 'Permission denied' <<<"$out"; then
  echo "ssh after revoke: rc $rc: $out" >&2
  exit 1
fi
rpc AuditService/ListEvents '{"action":"cli.login"}' | jq -e 'any(.events[]; .data.flow == "device")' >/dev/null
rpc AuditService/ListEvents '{"action":"cli_session.revoke"}' | jq -e --arg id "$sid" 'any(.events[]; .targetId == $id)' >/dev/null
