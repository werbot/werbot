# Gateways

A gateway is the bastion users connect to. This guide covers pools, enrollment, instances, the firewall, connections and limits. Variables and flags: [configuration.md](configuration.md#gateway-werbot-gateway).

## How it works

- Users connect by SSH straight to a gateway (port `2222`). The gateway opens the session to the resource.
- Each gateway instance holds one outbound mTLS connection (the tunnel) to the server. The server never connects to a gateway and needs no route to the resource network.
- A gateway in the UI is a **pool**. A resource belongs to exactly one pool. Every `werbot gateway` process is an **instance** of a pool, and all instances of a pool share one SSH host key.
- The gateway gets credentials per session from the server and does not cache them. Without the server, new sessions are refused with `Gateway is not ready, try again later` and that SSH connection closes after 30 s; running sessions continue.
- `werbot server --with-gateway` runs an instance of the system pool `local` inside the server process ([Embedded gateway](#embedded-gateway)).

## Firewall

| From | To | Port | Purpose |
| --- | --- | --- | --- |
| Users | Gateway | TCP `2222` (`WERBOT_GATEWAY_SSH_ADDR`) | SSH sessions. |
| Gateway | Server or web | `--server` / `WERBOT_GATEWAY_SERVER_URL` (HTTPS) | Enrollment. The start command in the UI passes the UI address: the server, or the `werbot web` domain, which proxies enrollment to the server. |
| Gateway | Server | TCP `8443` (`WERBOT_GATEWAY_ADDR`) | Tunnel, mTLS. TCP passthrough only: a proxy must not terminate TLS, and the external port must equal the port of `WERBOT_GATEWAY_ADDR`. |
| Gateway | Resources | Their SSH, Telnet, TCP or UDP ports | Sessions and health checks. |
| Monitoring | Gateway | Port of `WERBOT_GATEWAY_HTTP_ADDR`, when set | `/readyz` and `/metrics`. Never open it to users. |

The gateway listens only on the SSH port and, when set, `WERBOT_GATEWAY_HTTP_ADDR`. The tunnel address is `https://<host of WERBOT_PUBLIC_URL>:<port of WERBOT_GATEWAY_ADDR>`, so `WERBOT_PUBLIC_URL` of the server must be a name or IP the gateway host reaches, not `localhost`.

The tunnel shares that host name with the UI, so the name must resolve straight to the server, not to a TLS-terminating proxy. Behind Cloudflare set its DNS record to DNS only (grey cloud) and terminate TLS on your own proxy ([install.md](install.md#remote-access)). Otherwise enrollment works, but the instance stays offline and logs `tunnel: stream failed … x509: certificate signed by unknown authority`.

## Enroll a gateway

Owners and admins manage pools on the **Gateways** page.

1. **Create the pool.** Click **Create pool** and enter a pool name. The first pool of an organization (a created one or the system pool `local`) is assigned to resources without a pool. With more than one pool, the resource form shows a pool select.
2. **Create a join token.** In the pool click **Create join token** and set **Expires in** (hours, 1 to 8760, default 24) and **Max uses** (1 to 1000, default 10). The UI shows the token `wjt_…` once, inside the start command `WERBOT_GATEWAY_JOIN_TOKEN=<token> werbot gateway --server <UI address>`. It is not shown again.
3. **Start an instance on each host.** With Docker Compose:

   ```sh
   export WERBOT_VERSION=v0.1.1   # the server's version
   mkdir werbot-gateway && cd werbot-gateway
   curl -fsSLO https://raw.githubusercontent.com/werbot/werbot/$WERBOT_VERSION/deploy/docker-compose.gateway.yaml
   cat > .env <<EOF
   WERBOT_VERSION=$WERBOT_VERSION
   WERBOT_GATEWAY_SERVER_URL=https://werbot.example.com
   WERBOT_GATEWAY_JOIN_TOKEN=wjt_...
   EOF
   chmod 600 .env
   docker compose -f docker-compose.gateway.yaml up -d
   ```

   Or with the [binary](install.md#binary): `WERBOT_GATEWAY_JOIN_TOKEN=wjt_... werbot gateway --server https://werbot.example.com`. The flag `--join-token` also works, but it shows the token in `ps` output and shell history.
4. **Check the pool.** The host appears as an instance with hostname, IP, version, enrollment time and certificate expiry. It is online while its last heartbeat (every 15 s) is less than 45 s old.

One token enrolls new instances until it expires or reaches **Max uses**. Revoke it after the rollout (**Revoke**); the risks are in [security.md](security.md#gateway-join-tokens). A wrong, expired, revoked and exhausted token give the same refusal; the gateway then fails with `invalid join token`. After enrollment the instance no longer needs the token: delete the `WERBOT_GATEWAY_JOIN_TOKEN` line from `.env`.

The token ends with the fingerprint of the server CA. The gateway checks it before it writes any file and stops on `enroll: CA fingerprint mismatch: server CA does not match the join token`.

`WERBOT_GATEWAY_SERVER_URL` must be `https://` (a loopback host may use `http://`). Enrollment sends the join token and receives the pool's private SSH host key, so never use `--insecure-enroll` across hosts or networks you do not control. The [cluster example](scaling.md#cluster-example) uses it only on the private `edge` network of one host.

## Instances and the data volume

The data directory (`WERBOT_GATEWAY_DATA_DIR`; in `docker-compose.gateway.yaml`, project `werbot-gateway`, the volume `werbot-gateway_gateway-data` at `/var/lib/werbot`) holds the instance identity (`gateway.json`, `gateway.key`, `gateway.crt`, `ca.crt`), the pool host key `ssh_host_ed25519_key` and the recording spool `spool/`. A restart with the same volume needs no token.

- **Lost volume.** Enroll again with a valid token. The host becomes a new instance with the same host key (the key belongs to the pool). Unsent recording chunks are lost, and those recordings fail `Verify`.
- **Old instance.** Delete it in the UI (**Delete instance**) or wait: instances whose certificate expired more than 24 h ago are deleted automatically.
- **Retire a host.** Stop it, then **Delete instance**: its tunnel closes. Deleting a running instance closes its tunnel too; it must enroll again.

## Certificates

An instance certificate lives 24 h and is renewed after 16 h. Until the renewed certificate is first used, the previous one stays valid, so a lost renewal reply does not lock the instance out.

A saved identity whose certificate has expired or no longer loads counts as not enrolled. That happens when the instance was offline for more than 24 h or crashed during a renewal. The next enrollment overwrites it, so a standalone gateway then needs a new join token in `WERBOT_GATEWAY_JOIN_TOKEN`; a spent token fails at start with `invalid join token`. An instance whose certificate expires while running stops with `gateway certificate expired`.

## Several instances

Put the instances of one pool behind DNS round-robin or an L4 load balancer with PROXY protocol. Users see one host key on every instance. Balancer rules: [scaling.md](scaling.md#load-balancing); Kubernetes: [scaling.md](scaling.md#kubernetes).

## SSH address

The SSH address of a pool is the `host:port` that users and the [werbot CLI](cli.md) connect to: the DNS name of the instances or of their load balancer, for example `gw.example.com:2222`. An owner or admin sets it on the Gateways page (pool details, SSH address) or with `GatewayService/UpdateGateway` (`{"id": "<pool id>", "sshAddress": "gw.example.com:2222"}`); an empty value resets it. The port must be 1 to 65535.

The pool details also show the pool host key, and the API returns it with the pool (`host_key`) and with each resource from `ListAccessibleResources`. `werbot ssh`, `werbot ssh-config` and `werbot connect` write it to the profile's `known_hosts` under the SSH address, so `ssh-keyscan` is no longer needed. Until the address is set, the CLI fails for the pool's resources with `gateway address for <resource> is not set`.

## PROXY protocol

Set `WERBOT_GATEWAY_PROXY_PROTOCOL=true` only behind an L4 balancer that sends the header: HAProxy `send-proxy-v2`, nginx `proxy_protocol on`, AWS NLB with proxy protocol v2. Then every SSH connection must start with the header, and port `2222` must be open to the balancer only. Otherwise a client can forge its IP, and IP and country rules rely on it.

## Connect

Users log in with their SSH key or with the short-lived certificate of `werbot login` ([cli.md](cli.md)):

- `ssh -p 2222 <login>+<resource>@<gateway>` runs a shell or a command on the resource.
- `ssh -p 2222 <login>@<gateway>` shows a menu of the resources the user may open (interactive terminal only).
- `ssh -p 2222 -L <port>:<host>:<port> <login>+<resource>@<gateway>` forwards a port. For an `ssh` resource this needs `port_forwarding` (default `false`); a host equal to the resource name (case-insensitive, as `werbot connect` sends it) is `localhost` on the target, so `<resource>:<port>` reaches that port of the target itself.
- UDP and port access without `-L`: [connect.md](connect.md).

A connection must authenticate within 30 s. The server lookup of each offered key counts toward it, so a slow control plane may cut a client that offers many keys.

Two SSH extensions serve `werbot connect` on the same port, only for a `<login>+<resource>` login:

- The channel `datagram@werbot.com` opens one datagram session. Its open data is as for `direct-tcpip`. Each datagram is one frame `flow uint32 | len uint16 | payload`, big-endian, up to 65535 bytes; one channel carries many client flows.
- The global request `resource@werbot.com` answers the resource type, session kinds and ports as JSON after the same access check with port 0, for example `{"type":"tcp","kinds":["stream"],"ports":[{"port":5432,"end_port":0}]}`.

A refused, unknown or menu-login request gets `false` without payload, so it reveals nothing about resources the user cannot open; a refused one is audited as `session.deny`. A gateway that is not ready answers `false` with the text `Gateway is not ready, try again later`.

## Resource types

**Telnet.** A `telnet` resource opens as a terminal (`ssh -t -p 2222 <login>+<resource>@<gateway>`; TERM and window changes go to the target as NAWS; commands are not supported) or as a stream for a local telnet client (`ssh -p 2222 -L 2323:<host>:23 <login>+<resource>@<gateway>`, then `telnet 127.0.0.1 2323`). In the stream case the gateway is the telnet server to that client: it waits up to 1 s for its TERM and window, and only then dials the target. When the resource has a `login` and a password credential, the gateway logs in at the target's `login:`/`password:` prompts within 10 s and shows the output after the password prompt with the password removed; the login lines are not recorded. A refused or timed-out login ends the session with `Login to <resource> failed`. Without a login or a password the user answers the target's prompts. The gateway-to-target leg is plain text, as telnet is: run the gateway next to the equipment, in an isolated segment.

**TCP.** A `tcp` resource opens as a stream (`ssh -p 2222 -L 5432:<host>:5432 <login>+<resource>@<gateway>`). The gateway dials the resource host at the requested port, which must be inside the resource port ranges, and relays the bytes both ways. A close of one side reaches the other as a half-close, so a request/response protocol sees the end of its input. The session ends when both sides have closed. Nothing is recorded, only bytes are counted. A failed dial ends the session; the error does not name the target.

**UDP.** A `udp` resource is reached only through `werbot connect`. The gateway opens one target socket per client flow on its first datagram. Limits: 32 flows per session (datagrams of the 33rd flow are dropped, the session continues) and 8192 bytes per datagram (a larger one, either way, is dropped). A session with 32 flows holds about 512 KiB of memory (an 8 KiB buffer and a reader goroutine per flow). A flow without traffic for the resource's `flow_idle` (default `2m`) closes and reopens on its next datagram. An ICMP port-unreachable from the target is ignored. Drops count in `werbot_gateway_datagrams_dropped_total{reason="flows"|"size"}`.

## Channel checks

Every channel is checked when it opens:

- A stream or datagram target must be `host:port` with a port above 0.
- An `ssh` resource without `port_forwarding` refuses every target.
- Unless the resource allows any target (`ssh` with `port_forwarding`), the host must be the resource's host or its name and the port one of its ports; otherwise the user gets `Access denied`.

Each session counts bytes and datagrams both ways (`up` is client to target). Every fourth heartbeat (60 s) reports the counters that changed, and the session end carries the final ones (`werbot_gateway_session_bytes_total{direction}`, `werbot_gateway_datagrams_total{direction}`).

Every 5 s the gateway ends sessions without traffic for their idle timeout (`idle_timeout`) or older than their maximum duration (`max_duration`); a terminal first prints `Session closed: idle timeout` or `Session closed: max duration reached`. On each new policy snapshot and every 60 s it re-evaluates every running session against bindings, resources and their targets, network, country and schedule rules, the IP blocklist, and organization and team membership. It ends refused sessions with `access_revoked` and applies changed resource timeouts.

Refused connections (`denied`, `target_not_allowed`, `unsupported_type`, `unsupported_kind`, `busy`, `spool_full`, and `auth_failed` once per SSH connection that offered only refused keys) go to the server with the next heartbeat and become `session.deny` audit events. `not_ready` is not reported. Per-heartbeat limits: [security.md](security.md#threat-model-summary).

## Lock mode and clock

The gateway checks every open session again when a new snapshot arrives and when a grant or lock of the snapshot expires ([jit.md](jit.md#ending-sessions)).

- **Clock.** Expiry is measured by the clock of the gateway host, not of the server. Run NTP (chrony, systemd-timesyncd) on every gateway host: a clock that runs behind keeps sessions open past their grant, one that runs ahead ends them early.
- **Without the server.** Expiry needs no server: the timer works from the last snapshot. Revokes and new locks reach the gateway only through the tunnel.
- **Lock mode** is an organization setting (Settings, `lock_mode`). With `best_effort` (default) open sessions keep running while the tunnel is down. With `strict` the gateway ends every session with `policy_stale` (log line `policy is stale, sessions terminated`) once the tunnel has been down for more than 60 seconds (checked every 5 seconds, so 60 to 65 seconds after the loss); a reconnect resets the wait. Use `strict` when a lock must win over availability.

## Recordings

Recorded: `ssh` terminal and exec sessions, `telnet` terminal and stream sessions. Streams of `ssh` (`ssh -L`), `tcp` and `udp` are not recorded; they count bytes and datagrams only.

- Recordings are asciicast v2 in gzip chunks of up to 64 KiB raw, flushed at least every 10 s.
- Chunks and session ends go to the spool (`WERBOT_GATEWAY_DATA_DIR/spool`) first and upload through the tunnel in order: the session end, then the chunks, then the closing manifest.
- While the server or its recording storage is unreachable, running sessions continue and the spool keeps their data. A session whose unsent data passes 64 MiB ends with `recording_unavailable`.
- On shutdown the gateway waits up to 10 s for the spool to deliver the session ends; the rest goes after the next start.
- Data the server refuses moves to `spool/rejected/`, which no longer counts toward `WERBOT_GATEWAY_SPOOL_MAX`. A running session whose recording the server refuses ends with `recording_unavailable`. This happens to the old spool of an instance that enrolled again with a new identity; those recordings stay unverified.
- Metrics: `werbot_gateway_spool_bytes` (unsent bytes) and `werbot_gateway_sessions_rejected_total{reason="spool_full"}`.

## Readiness, shutdown and limits

- `WERBOT_GATEWAY_HTTP_ADDR` enables `/readyz` and `/metrics`. `/readyz` answers 200 while the tunnel is up with a policy snapshot, a session slot is free and no drain runs.
- The compose files have no Docker healthcheck for gateways: the distroless image has no curl, and `werbot healthcheck` checks only a server. Balancers and Kubernetes read `/readyz`.
- On SIGTERM the gateway stops accepting connections, and running sessions continue for `WERBOT_GATEWAY_DRAIN_TIMEOUT` (default `30s`); the rest end with `gateway_shutdown`. Hence `stop_grace_period: 40s` in compose.
- Above `WERBOT_GATEWAY_MAX_SESSIONS` (default 10000) new sessions get `Gateway is busy, try again later`. The same answer comes while the unsent spool is at `WERBOT_GATEWAY_SPOOL_MAX` (default `1GiB`); that cap affects recorded sessions, and every SSH terminal or exec session is recorded. Details: [configuration.md](configuration.md#readiness-drain-and-limits).
- Set `ulimit -n` to at least 4 × `WERBOT_GATEWAY_MAX_SESSIONS`; compose sets `nofile` to 65536.

## Embedded gateway

`werbot server --with-gateway` joins the system pool `local` of the first organization (by creation time) with a system join token (max uses 1) per replica, so it needs no token from you. It waits for Setup, checking for an organization every 5 s. Before the instance is enrolled, database and enrollment errors are logged and retried: the wait doubles up to 5 minutes, with a new system token each time. A busy SSH port or a failure after enrollment stops the server. On shutdown the gateway drains its sessions before the server closes the tunnel, so allow `WERBOT_GATEWAY_DRAIN_TIMEOUT` plus about 40 s for the stop (compose: [install.md](install.md#docker-compose)). Its target restrictions: [configuration.md](configuration.md#embedded-gateway).

## Upgrading

A gateway runs the same version as the server. In `v0.x` stop the gateways before the server upgrade and start them after it: [upgrading.md](upgrading.md#upgrade-a-cluster).
