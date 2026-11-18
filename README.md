# Werbot

Werbot is an open-source access gateway that authorizes every command from people and AI agents to servers and databases, records everything, and never hands out a single password.

## Status

pre-alpha (v2 rewrite)

## Requirements

- Go 1.27
- Bun 1.3
- Docker (local Postgres 18 and the compose stack)

## Quick start

```sh
make dev-db                      # Postgres 18 on localhost:27432 (postgres/password)
(cd web && bun install)          # web/node_modules is required by `make proto`
make proto                       # buf lint + generate (Go and TypeScript)
make build                       # builds the UI and bin/werbot
export WERBOT_SECRET_KEY="$(openssl rand -base64 32)"  # generate once and keep: it decrypts resource credentials and queued email jobs
WERBOT_ENV=development \
WERBOT_DATABASE_URL='postgres://postgres:password@localhost:27432/postgres?sslmode=disable' \
WERBOT_PUBLIC_URL=http://localhost:8080 ./bin/werbot server
```

`WERBOT_ENV=development` is for local plain-http runs: the session cookie is `werbot_session` without `Secure`, and without SMTP the log shows the full email text with reset and invite links. Production mode uses `__Host-werbot_session` with `Secure`, which browsers drop over plain http on any host but `localhost` (Safari drops it on `localhost` too).

Commands: `./bin/werbot <server|gateway|web|connect|migrate|version>`.

- `server` runs migrations (schema and River job queue) and creates the internal gateway CA and the ed25519 key that signs session recording manifests on first start, under an advisory lock. Then it serves the API (and the UI when embedded), the gateway tunnel on `WERBOT_GATEWAY_ADDR` and runs the River job client.
- `server --with-gateway` also runs a gateway instance in the same process. Each replica joins the system pool `local` of the first organization (by creation time) with a single-use system join token; before `Setup` it checks for an organization every 5s. It reaches its own server over loopback (`http://127.0.0.1:<WERBOT_HTTP_ADDR port>`, tunnel `https://127.0.0.1:<WERBOT_GATEWAY_ADDR port>`), so both addresses must accept connections on 127.0.0.1: leave the host empty (`:8080`) or use `0.0.0.0` or `127.0.0.1`, not a single non-loopback IP. It keeps its identity in `WERBOT_DATA_DIR/gateway` (or `WERBOT_GATEWAY_DATA_DIR`) and reads the other `WERBOT_GATEWAY_*` variables below; it refuses loopback, link-local (including cloud metadata), multicast and unspecified targets and the resolved addresses of the database host unless `WERBOT_LOCAL_POOL_ALLOW_LOOPBACK=true` (see below), so a same-host database needs that opt-in, while the host's LAN addresses stay reachable (this applies to `telnet`, `tcp` and `udp` sessions and to health probes; `ssh` resources keep their own dial, so a loopback `ssh` resource connects but its health shows `unhealthy`); its metrics go to the server's `/metrics`. Database and enrollment errors before the instance is enrolled are logged and retried (the wait doubles up to 5 minutes, a new single-use token each time); a busy SSH port or a failure after enrollment stops the server. On shutdown the gateway drains its sessions before the server closes the tunnel, so allow `WERBOT_GATEWAY_DRAIN_TIMEOUT` plus about 40s for the stop (compose: `stop_grace_period: 70s`).
- Upgrade the server and every standalone gateway to the same release together: the server requires the session kind (`terminal`, `exec`, `stream`, `datagram`) that older gateways do not send, so their new sessions fail until they are upgraded.
- `gateway` runs a standalone gateway instance (`./bin/werbot gateway --help` lists the flags: `--server`, `--join-token`, `--data-dir`, `--ssh-addr`, `--proxy-protocol`, `--name`, `--idle-timeout`, `--http-addr`, `--drain-timeout`, `--max-sessions`, `--spool-max`, `--max-duration`, and the flag-only `--insecure-enroll`).
- Web UI `/gateways` (owners and admins): create pools, see instances (online within 45s of the last heartbeat) and manage join tokens. "Create join token" shows the `WERBOT_GATEWAY_JOIN_TOKEN=<token> werbot gateway --server <origin>` command once (the env form keeps the token out of `ps` output and shell history); the token is not shown again. The resource form shows a pool select when more than one pool exists.
- Web UI `/sessions` (every user; members see only their own): session list filtered by status and period (owners and admins also by user and resource) and, per session, its details, the recording integrity badge (`Recording verified` when `X-Werbot-Recording-Verified` is `true`, a failure banner otherwise, a notice while the recording is unfinished) and an asciinema player of `/api/recordings/{session_id}.cast`. The list shows bytes up and down; the details add kind, target and, for datagram sessions, datagram counts. The resource pages offer `telnet`, `tcp` and `udp` forms (port ranges like `22, 8000-8100`), idle timeout and max duration in minutes, a health dot, connect hints per session kind, and ports on bindings of `tcp` and `udp` resources.
- Web UI `/audit` (owners and admins): the audit log filtered by action, actor and period; the filter lives in the query string (`/audit?action=session&actor=<user id>&from=YYYY-MM-DD&to=YYYY-MM-DD`, `to` inclusive). A click on an event opens its fields and `data`.
- `connect` opens local ports that reach `tcp`, `udp` and other port resources through a gateway (see `werbot connect` below).
- `migrate` runs migrations (schema and River job queue) and exits.
- `web` serves the UI and proxies the API to a separate `server`.
- `version` prints the build version.

The API covers identity (auth, profile, SSH keys, members, teams) and, for owners and admins:

- `ResourceService`: resources of registered types (`ssh`, `telnet` (default port 23, login optional, `password` credential), `tcp` (host plus 1–16 non-overlapping port ranges), `udp` (like `tcp` plus `flow_idle`, default `2m`, allowed `1s`–`1h`); `tcp` and `udp` take no credentials) with labels, optional per-resource `idle_timeout` (`1m`–`24h`) and `max_duration` (`1m`–`168h`; unset means the gateway default), a `health` state (`healthy` when an online gateway instance of the pool probed the resource successfully within 3 minutes, `unhealthy` when all such fresh probes failed (with the latest error), otherwise `unknown`), write-only credentials (sealed with `WERBOT_SECRET_KEY`) and pinned SSH host keys.
- Gateway health checks: each pool resource with an endpoint (types `ssh`, `tcp`, ...; not `udp`, not with `NoHealth` or an empty host) is probed by 2 gateway instances chosen by rendezvous hashing over the online instances of the pool. A probe is a TCP connect (timeout 5s, at most 8 at a time) to the host and the first port of each range, first pass at start, then every 60s ±20%. Each pass sends every result. A probe uses the same dialer as sessions, so with local targets denied a loopback target reports `unhealthy` with the refusal. Metric: `werbot_gateway_health_checks_total{result="ok"|"fail"}`.
- `AccessService`: bindings that grant a user or team one resource or every resource matching a label selector. A binding may list up to 16 target ports or ranges (`ports`, 1–65535, no overlap): stream and datagram sessions (TCP, UDP, `ssh -L`) then reach only those ports, while terminal and exec sessions are not restricted. A binding without ports allows every port of the resource.
- `PolicyService`: per-resource network, country and schedule (UTC) rules, and `CheckAccess` to test a decision (optional `port`; 0 tests a terminal or exec session). Decisions use Cedar and fail closed.
- `SessionService`: session list (filters by user, resource, period, status; paging; each session carries its `kind` (`terminal`, `exec`, `stream`, `datagram`), `target` and byte and datagram counters), session card and `VerifyRecording` (hash chain and signature check). Members see only their own sessions.
- `GET /api/recordings/{session_id}.cast` (session cookie): the session recording as one asciicast v2 file, for anyone who may see the session. The header `X-Werbot-Recording-Verified` is `true` only when the served bytes passed the hash chain and signature check; an active, unfinished or tampered recording is served as far as it goes with `false`. The response is an attachment (`<session_id>.cast`, `nosniff`); a chunk that inflates past 1 MiB (a gateway writes at most 64 KiB) ends the stream there. A session without a recording, or one the caller may not see, is `404`; an unreachable recording store is `503`. A replica runs at most 4 replays at once; the fifth gets `503` `replay busy, try again` with `Retry-After: 5`. The server checks the whole recording before it sends the headers, so a very large recording replayed through `werbot web` can hit its 30 s upstream header timeout (`503`).
- `AuditService`: read-only event journal (`ListEvents`, `GetEvent`) for owners and admins; other roles get `permission_denied`. `ListEvents` filters by `action` (a domain such as `session` matches `session.*`, a full action matches itself), actor, target and period `[from, to)`, newest first, `page_size` 50 by default and 200 at most (a larger value is clamped). Events without an organization are never shown. `session.start` events carry the session's `kind` and `target` in `data`.
- `GatewayService`: gateway pools, their instances and join tokens. A gateway joins with `GatewayEnrollService/Enroll` (the join token is its only credential) and then talks to the server only over the mTLS tunnel (`https://<WERBOT_PUBLIC_URL host>:<WERBOT_GATEWAY_ADDR port>`). The server never connects to a gateway.

Maintenance jobs run on the River `maintenance` queue: `session.end_lost` (every 30s) closes sessions of gateways that stopped reporting, `gateway.gc` (hourly) removes instances whose certificate expired over 24h ago and join tokens expired over 7 days ago.

Background jobs on the River `feeds` queue (one worker) need outbound HTTPS from the server:

- `blocklist.refresh` (on start, then daily) downloads the FireHOL level 1 list from `raw.githubusercontent.com`; listed IPs are always denied.
- `geoip.refresh` (on start, then daily at 04:00 UTC) downloads the monthly DB-IP Country Lite database from `download.db-ip.com`. The database is stored in Postgres (no files on disk), and every server loads it on start. Without it, country rules deny. IP geolocation by [DB-IP](https://db-ip.com).

A failed download keeps the previous data.

## Configuration

`werbot server` and `werbot migrate`:

| Variable | Default | Description |
| --- | --- | --- |
| `WERBOT_DATABASE_URL` | required | Postgres URL. Pool size: `pool_max_conns`; query limit: `statement_timeout` (default `30s`); `jit` (default `off`), all as URL parameters. |
| `WERBOT_ENV` | `production` | `production` or `development`. |
| `WERBOT_HTTP_ADDR` | `:8080` | API (and embedded UI) listen address. |
| `WERBOT_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `WERBOT_LOG_FORMAT` | `json` | `json` or `text`. |
| `WERBOT_METRICS_ADDR` | empty (disabled) | Listen address for `/metrics`. |
| `WERBOT_WEB_EMBEDDED` | `true` | Serve the UI from the server. |
| `WERBOT_PUBLIC_URL` | required | Public `http(s)://` address of the server, for example `https://werbot.example.com`. |
| `WERBOT_WEB_URL` | `WERBOT_PUBLIC_URL` | Public address of the UI; base for links in emails. |
| `WERBOT_TRUSTED_PROXIES` | empty | Comma-separated CIDRs whose `X-Forwarded-For` is trusted. |
| `WERBOT_SMTP_HOST` | empty | SMTP server. Empty: emails go to the log only (the full text, with links, only when `WERBOT_ENV=development`). |
| `WERBOT_SMTP_PORT` | `587` | SMTP port, 1-65535. |
| `WERBOT_SMTP_USERNAME`, `WERBOT_SMTP_PASSWORD` | empty | SMTP credentials. |
| `WERBOT_SMTP_FROM` | empty | Sender address; required when `WERBOT_SMTP_HOST` is set. |
| `WERBOT_SECRET_KEY` | required | Base64 of 32 random bytes (`openssl rand -base64 32`); encrypts stored secrets. |
| `WERBOT_SECRET_KEY_PREVIOUS` | empty | Comma-separated base64 keys kept for decrypting data sealed before a key rotation. |
| `WERBOT_DATA_DIR` | `./data` | Local state directory: the embedded gateway, and session recordings in `recordings/` while `WERBOT_RECORDINGS_S3_BUCKET` is empty. |
| `WERBOT_RECORDINGS_S3_BUCKET` | empty | S3 bucket for session recordings. Empty: recordings go to the file system under `WERBOT_DATA_DIR/recordings`; use that only with one replica or a ReadWriteMany volume. |
| `WERBOT_RECORDINGS_S3_ENDPOINT` | empty | S3-compatible endpoint URL. Empty: standard AWS. |
| `WERBOT_RECORDINGS_S3_REGION` | `us-east-1` when the bucket is set | S3 region. |
| `WERBOT_RECORDINGS_S3_ACCESS_KEY`, `WERBOT_RECORDINGS_S3_SECRET_KEY` | empty | S3 credentials; set both or neither. |
| `WERBOT_GATEWAY_ADDR` | `:8443` | mTLS listener of the gateway tunnel (TLS 1.3, HTTP/2). Its port is the port in the tunnel URL given to gateways. |
| `WERBOT_LOCAL_POOL_ALLOW_LOOPBACK` | `false` | The embedded gateway (`--with-gateway`, pool `local`) runs inside the server process, so by default it refuses target addresses on the server's own host: loopback (`127.0.0.0/8`, `::1`), link-local including cloud metadata (`169.254.0.0/16`, `fe80::/10`), multicast (including interface-local `ff01::/16`), unspecified (`0.0.0.0`, `::`), and every address the host of `WERBOT_DATABASE_URL` resolves to. The check runs on the resolved address when a session opens (the user gets `Access denied`, audited as `target_not_allowed`) and again on every connect. It covers `telnet`, `tcp` and `udp` sessions and health probes, not `ssh` resources: they keep their own dial, so a loopback `ssh` resource connects while its health shows `unhealthy` with the refusal. Other addresses of the host and its LAN stay reachable: restrict them with resource endpoints and access bindings. `true` turns the check off. Standalone gateways are not affected. |

`werbot web` (no database access):

| Variable | Default | Description |
| --- | --- | --- |
| `WERBOT_WEB_API_URL` | required | `http(s)://` URL of the API server. |
| `WERBOT_WEB_ADDR` | `:8090` | Listen address. |
| `WERBOT_TRUSTED_PROXIES` | empty | Comma-separated CIDRs whose `X-Forwarded-For` is trusted. |
| `WERBOT_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `WERBOT_LOG_FORMAT` | `json` | `json` or `text`. |
| `WERBOT_METRICS_ADDR` | empty (disabled) | Listen address for `/metrics`. |

`werbot gateway` (an instance of a gateway pool; each variable also has a flag, for example `--ssh-addr`):

| Variable | Default | Description |
| --- | --- | --- |
| `WERBOT_GATEWAY_SERVER_URL` | empty | `https://` URL of the server API; needed for enrollment only (a start with a valid saved identity does not check it). `http://` is accepted only for a loopback host (`127.0.0.0/8`, `::1`, `localhost`) or with the flag `--insecure-enroll`: enrollment sends the join token and receives the pool's private SSH host key, so use it only on a trusted network. |
| `WERBOT_GATEWAY_JOIN_TOKEN` | empty | Join token of the pool; needed for enrollment: the first start or a start with a stale identity (see below). A valid saved identity wins over it. |
| `WERBOT_GATEWAY_DATA_DIR` | `./gateway-data` | Instance identity: `gateway.json`, `gateway.key`, `gateway.crt`, `ca.crt`, `ssh_host_ed25519_key`, and the recording spool `spool/`. Keep it on a persistent volume (a PVC in Kubernetes): undelivered recordings and session ends wait there across restarts. |
| `WERBOT_GATEWAY_SSH_ADDR` | `:2222` | SSH listen address. |
| `WERBOT_GATEWAY_PROXY_PROTOCOL` | `false` | Require a PROXY protocol header on every SSH connection; expose the port to the load balancer only. |
| `WERBOT_GATEWAY_NAME` | OS hostname | Instance hostname shown in the pool. |
| `WERBOT_GATEWAY_IDLE_TIMEOUT` | `5m` | Ends a session without traffic in either direction (`idle_timeout`) unless its resource sets `idle_timeout`, and closes an SSH connection that runs no session for this long (keepalives do not count; typing in the resource menu does). A running session keeps its connection open; `0` disables both. |
| `WERBOT_GATEWAY_HTTP_ADDR` | empty (disabled) | Listen address for `/readyz` and `/metrics`. |
| `WERBOT_GATEWAY_DRAIN_TIMEOUT` | `30s` | On SIGTERM, how long running sessions may continue before they end with `gateway_shutdown`. |
| `WERBOT_GATEWAY_SPOOL_MAX` | `1GiB` | Size cap of the recording spool: bytes or a `KiB`, `MiB`, `GiB` suffix; must be above 0. Flag `--spool-max`. While the unsent spool is at the cap, new sessions of recorded kinds get `Gateway is busy, try again later`; unrecorded ones (`tcp`, `udp`, `ssh -L`) still open and running sessions continue. The `WERBOT_GATEWAY_DATA_DIR` volume must hold the cap plus headroom. While the spool cannot write (disk full or not writable), new recorded sessions get the same answer and the same metric, and each running session ends with `recording_unavailable` at its next chunk flush (within about 10 s); a command that ends sooner can stay unrecorded. |
| `WERBOT_GATEWAY_MAX_SESSIONS` | `10000` | Concurrent session limit, 1-10000; above it users get `Gateway is busy, try again later`. |
| `WERBOT_GATEWAY_MAX_DURATION` | `12h` | Ends a session after this long (`max_duration`) unless its resource sets `max_duration`; `0` means no limit, a negative value fails the start. Flag `--max-duration`. |

`werbot connect` (the client; each variable also has a flag):

| Variable | Default | Description |
| --- | --- | --- |
| `WERBOT_CONNECT_GATEWAY` | required | Gateway SSH address `host[:port]`; the port defaults to `2222`. Flag `--gateway`. |
| `WERBOT_CONNECT_LOGIN` | required | Werbot login. Flag `--login`. |

The gateway listens only on the SSH port and, when set, `WERBOT_GATEWAY_HTTP_ADDR`; it connects out to the server tunnel. Users log in with their SSH key: `ssh <login>+<resource>@<gateway> -p 2222` runs a shell or a command on the resource, `ssh -L <port>:<host>:<port> <login>+<resource>@<gateway> -p 2222` forwards a port (only when the SSH resource sets `port_forwarding`, default `false`; a host equal to the resource name, case-insensitive, as `werbot connect` sends it, is `localhost` on the SSH target, so `<resource>:<port>` reaches that port of the target itself), and `ssh <login>@<gateway> -p 2222` (interactive terminal only) shows a menu of the resources the user may open. Two SSH extensions serve port access for `werbot connect` on the same port and only for a `<login>+<resource>` login: the channel `datagram@werbot.com` opens one datagram session (open data as for `direct-tcpip`; each datagram is one frame `flow uint32 | len uint16 | payload`, big-endian, up to 65535 bytes, and one channel carries many client flows), and the global request `resource@werbot.com` answers the resource type, session kinds and ports as JSON (`{"type":"tcp","kinds":["stream"],"ports":[{"port":5432,"end_port":0}]}`) after the same access check with port 0. A refused, unknown or menu-login request gets `false` without payload, so it reveals nothing about resources the user cannot open; a refused one is audited as `session.deny`. A gateway that is not ready answers `false` with the text `Gateway is not ready, try again later`. Without the server or a policy snapshot the gateway refuses new sessions (`Gateway is not ready, try again later`) and closes that SSH connection after 30 s; running sessions continue. A connection must authenticate within 30 s, and the control-plane lookup of each offered key counts toward it, so a slow control plane may cut a client that offers many keys. `/readyz` returns 200 only while the tunnel is up with a snapshot (a planned reconnect for `rebalance` or a server `shutdown` keeps it for up to 5s), a session slot is free and no drain runs. On SIGTERM `/readyz` returns 503 at once and the SSH port stops accepting connections.

A `telnet` resource opens as a terminal (`ssh -t <login>+<resource>@<gateway> -p 2222`; TERM and window changes go to the target as NAWS; commands are not supported) or as a stream for a local telnet client (`ssh -L 2323:<host>:23 <login>+<resource>@<gateway> -p 2222`, then `telnet 127.0.0.1 2323`): the gateway is the telnet server to that client, waits up to 1 s for its TERM and window, and only then dials the target. When the resource has a `login` and a password credential, the gateway logs in at the target's `login:`/`password:` prompts within 10 s and shows the output after the password prompt with the password removed; the login lines are not recorded. A refused or timed-out login ends the session with `Login to <resource> failed`. Without a login or a password the user answers the target's prompts. The gateway → target leg is plain text, as telnet is: run the gateway next to the equipment, in an isolated segment.

A `tcp` resource opens as a stream (`ssh -L 5432:<host>:5432 <login>+<resource>@<gateway> -p 2222`): the gateway dials the resource `host` at the requested port (it must be inside the resource port ranges) and relays the bytes both ways. A close of one side is passed to the other as a half-close, so a request/response protocol sees the end of its input. The session ends when both sides have closed. Nothing is recorded, only bytes are counted. A failed dial ends the session; the error does not name the target.

Sessions are recorded for the kinds their resource type records: `ssh` terminal and exec sessions, `telnet` terminal and stream sessions. Streams of `ssh` (`ssh -L`), `tcp` and `udp` are not recorded; they only count bytes and datagrams. Recordings are asciicast v2 (gzip chunks of up to 64 KiB raw, flushed at least every 10 s). Chunks and session ends go to the spool in `WERBOT_GATEWAY_DATA_DIR/spool` first and are uploaded through the tunnel in order: the session end first, then the chunks, then the closing manifest. While the server or its recording storage is unreachable, running sessions continue and the spool keeps their data; a session whose unsent data passes 64 MiB ends with the reason `recording_unavailable`. On shutdown the gateway waits up to 10 s for the spool to deliver the session ends; the rest is sent after the next start. Data the server refuses moves to `spool/rejected/`, which no longer counts toward `WERBOT_GATEWAY_SPOOL_MAX`, and a running session whose recording the server refuses ends with `recording_unavailable`; this happens to the old spool of an instance that enrolled again with a new identity (its certificate expired), and those recordings stay unverified. Metrics: `werbot_gateway_spool_bytes` (unsent bytes) and `werbot_gateway_sessions_rejected_total{reason="spool_full"}`.

UDP sessions use one target socket per client flow (a `datagram@werbot.com` flow), dialed on its first datagram. Limits: 32 flows per session (the 33rd flow's datagrams are dropped, the session continues), 8192 bytes per datagram (a larger one, either way, is dropped); a UDP session with 32 flows holds about 512 KiB of memory (an 8 KiB buffer and a reader goroutine per flow). A flow with no traffic for the resource's `flow_idle` (default `2m`) is closed and reopens on its next datagram; an ICMP port-unreachable from the target is ignored. Drops are counted in `werbot_gateway_datagrams_dropped_total{reason="flows"|"size"}`.

Every channel is checked when it opens. A stream or datagram target must be a `host:port` with a port above 0; an `ssh` resource without `port_forwarding` refuses every target; unless the resource allows any target (`ssh` with `port_forwarding`), the host must be the resource's configured host or its name and the port one of its ports, otherwise the user gets `Access denied`. Each session counts bytes and datagrams in both directions (`up` is client to target); every fourth heartbeat (60 s) reports the counters that changed, and the session end carries the final ones (metrics `werbot_gateway_session_bytes_total{direction}`, `werbot_gateway_datagrams_total{direction}`). Every 5 s the gateway ends sessions without traffic for their idle timeout (`idle_timeout`) or older than their maximum duration (`max_duration`); a terminal first prints `Session closed: idle timeout` or `Session closed: max duration reached`. On each new policy snapshot and every 60 s it re-evaluates every running session against the current policy (bindings, resources and their targets, network, country and schedule rules, the IP blocklist, organization and team membership) and ends the refused ones with `access_revoked`; a re-evaluation also applies changed resource timeouts. Refused connections (`denied`, `target_not_allowed`, `unsupported_type`, `unsupported_kind`, `busy`, `spool_full`, and `auth_failed` once per SSH connection that offered only refused keys) go to the server with the next heartbeat and become `session.deny` audit events; `not_ready` is not reported. At most 100 go per heartbeat (15 s), the rest only count in `werbot_gateway_denials_dropped_total`, so a brute force adds at most 100 audit rows per instance every 15 s. Denials are kept in memory and lost if the instance is killed.

`werbot connect [flags] [<local_port>:]<resource>[:<target_port>] ...` opens a local port per resource on `127.0.0.1` and prints `<resource> → 127.0.0.1:<port> (tcp|udp)`, for example `werbot connect --gateway gw.example.com --login alice pg` and then `psql -h 127.0.0.1 -p 5432`. Flags: `--gateway`, `--login`, `-i <key>` (otherwise the ssh-agent at `SSH_AUTH_SOCK` when it answers, otherwise `~/.ssh/id_ed25519`; an encrypted key file fails with `key <path> is encrypted; load it into ssh-agent`; the agent is never forwarded) and `--known-hosts <path>` (default `~/.ssh/known_hosts`). Each resource gets its own SSH connection as `<login>+<resource>`; the client asks `resource@werbot.com` for the resource and opens a UDP port when it has the `datagram` kind, otherwise a TCP port for `stream` (otherwise `Resource <resource> does not support port access`). The target port is `:<target_port>` or the resource's only port; a resource without ports, with several ports or with a range needs `:<target_port>` (`resource web has several ports (8000-8002); use web:<port>`). The local port is `<local_port>` or the target port, plus 10000 below 1024 (`23` → `10023`, `53` → `10053`); a busy one fails with `local port <n> is busy; use <free>:<resource>`. The port listens on `127.0.0.1` only, but, as with `ssh -L`, every local user of the machine can connect to it. Each TCP connection opens its own `direct-tcpip` channel (one gateway session, one access check) and passes half-closes on; a UDP port uses one `datagram@werbot.com` channel where each client address is a flow (at most 32; a new address beyond them is dropped until a flow has had no traffic for 2 minutes). The client frees a flow after 2 minutes whatever the resource's `flow_idle`: with `flow_idle` above 2 minutes and many client addresses, the gateway still holds the old flows, so its 32-flow cap can drop new flows until they age out. After a refused or failed channel open, datagrams are dropped for a backoff of 1 s doubling up to 30 s, and the same text is printed once. The gateway host key is checked like OpenSSH `StrictHostKeyChecking=accept-new`: an unknown key is appended to known_hosts (created `0700`/`0600`), a different key fails with `host key mismatch for <host:port>`. A `keepalive@openssh.com` goes every 30 s; three unanswered in a row count as a drop. After a drop the client dials again with a backoff of 1 s doubling up to 30 s while the local ports stay open, and prints each new dial error (`<resource>: reconnect failed: <error>; retrying`): running TCP connections end, a new one waits up to 10 s for the connection and is closed otherwise, and UDP opens a new channel (a new session with an empty flow table) on the next datagram; a changed host key or a refused key ends the client with an error. Gateway texts (`Access denied`, `Gateway is busy, try again later`, ...) are printed as they are; a refused resource exits with code 1, Ctrl+C closes everything with code 0.

The instance renews its 24h certificate after 16h; until the renewed certificate is first used, the previous one stays valid, so a lost renewal response does not lock the instance out. A saved identity whose certificate has expired or no longer loads (the instance was offline for more than 24h, or crashed during a renewal) counts as not enrolled, and the next enrollment overwrites it. An instance whose certificate expires while running stops (`gateway certificate expired`). The embedded gateway enrolls again by itself. A standalone gateway offline for more than 24h needs a new join token in `WERBOT_GATEWAY_JOIN_TOKEN`; a spent token fails at start with `invalid join token`.

## Deployment modes

- All-in-one (default): `werbot server` serves the API and the UI on one address.
- Several server replicas: put an L4 (TCP passthrough) load balancer in front of the tunnel port (`8443` by default) with the same external port; mTLS ends on the replica, so a TLS-terminating proxy breaks the tunnel. The API port works behind any HTTP load balancer.
- Separate UI: run `werbot server` with `WERBOT_WEB_EMBEDDED=false` and `werbot web` with `WERBOT_WEB_API_URL` pointing to the server. Set `WERBOT_WEB_URL` on the server to the UI address (links in emails) and `WERBOT_TRUSTED_PROXIES` to the `werbot web` address (client IPs in sessions and audit).

Docker:

```sh
export WERBOT_SECRET_KEY="$(openssl rand -base64 32)"  # required; keep the same key between runs
docker compose -f deploy/docker-compose.yaml up --build
```

The stack runs `werbot server --with-gateway` and listens on http://localhost:8080 (`/healthz`, `/readyz`), SSH on port `2222` (pool `local`, after `Setup`) and the gateway tunnel on port `8443` (for external gateways). State lives in the `werbot-home` volume (`WERBOT_DATA_DIR=/home/nonroot/data`).

## Development

```sh
make test   # needs `make dev-db`
make lint
make e2e    # Playwright e2e on DB `werbot_e2e`, no internet feeds (the server runs with `HTTPS_PROXY` pointing at a closed port; needs `make dev-db`, Go, ssh and ssh-keygen; first run: `cd web && bunx playwright install chromium`)
make e2e-network  # Docker Compose e2e: SSH, telnet, TCP and UDP through a gateway pool to targets on an internal-only network
make e2e-cluster  # Go scenarios (test/cluster, tag `cluster`: topology, cross-replica login, binding propagation, bootstrap, GeoIP reload, Postgres restart, shared pool host key, replica kill, lost instance, gateway max sessions, servers down fail-closed, S3 outage, join token revocation and exhaustion, separate web UI behind `werbot web` replicas, `werbot connect` TCP and UDP through the pool with an instance kill) on a stand of 2 server, 2 web and 2 gateway replicas behind nginx
make load         # manual load tests L1–L4 (test/load, tag `load`) on the same stand; Linux only
(cd web && bun run test:unit)  # Vitest unit tests for the web UI
make sqlc   # regenerates internal/<domain>/dbq from migrations/ and internal/<domain>/queries.sql
```

`make e2e` starts `werbot server --with-gateway` (gateway SSH on `127.0.0.1:2222`, a fresh `WERBOT_DATA_DIR` per run) and the SSH target `go run ./internal/e2e/sshtarget` on `127.0.0.1:2022`; both ports must be free.

`make e2e-network` needs docker, curl, jq, ssh, ssh-keygen, Go, nc and dig. It builds the `werbot-e2e:local` image and the TCP/UDP echo target (`internal/e2e/echotarget`, into the gitignored `deploy/e2e/.bin`), publishes ports `8080`, `8081` and `2222` on localhost, and removes the `werbot-e2e` project (containers, networks, volumes) on exit. The targets sit on the internal-only network that only the gateway pool joins: an OpenSSH server, a busybox `telnetd` with a real `/bin/login` (`router`), the echo target on TCP and UDP port 7 (`echo`) and CoreDNS answering `probe.werbot.test` (`dns`). The scenario checks SSH through the pool and a gateway restart; a telnet session through the SSH bridge (the router password never reaches the user, the session is `terminal` and its recording verifies); `werbot connect` with `echo`, `router` and `dns` on `127.0.0.1:10007`, `10023` (telnet stream with autologin) and `10053` (UDP, two flows); a forward to a port outside the resource refused and audited as `session.deny` with `target_not_allowed`; and the health of the TCP and telnet resources turning `healthy` within 180s while the UDP one stays `unknown`. It also uses local ports `10007`, `10023` and `10053`.

`make e2e-cluster` needs docker, ssh and Go. It builds the `werbot:cluster` image and runs `go test -tags cluster ./test/cluster/...` against the compose project `werbot-cluster` (override with `CLUSTER_PROJECT`): Postgres, SeaweedFS (S3 recordings), `server1`/`server2`, `web1`/`web2`, nginx and the gateway pool `gw1`/`gw2`, with the targets on an internal-only network. Every published port is an ephemeral port on `127.0.0.1`; the networks use the fixed subnets `10.231.0.0/24` and `10.231.1.0/24`, so run one stand per host at a time. The stand is removed at the end; on failure its logs go to `test/cluster/compose.log` (gitignored, test-only credentials). It also builds `werbot` for the host into the gitignored `test/cluster/.bin`: the `werbot connect` scenario carries TCP to the echo target and DNS to CoreDNS on free local ports, checks the `stream` and `datagram` sessions and the health probes of both instances, then kills the instance of the DNS session and expects DNS to answer again within 30 s. The lost-instance scenario waits up to 3 minutes for `gateway_lost`, after both server replicas have run for 2 minutes; the S3 outage scenario stops SeaweedFS for 1 minute. `go test ./...` without the tag skips `test/`. CI runs it as the `e2e-cluster` job on every push and pull request to `v2` (50-minute limit; a newer push cancels the run it supersedes; on failure `compose.log` is uploaded as the `e2e-cluster-logs` artifact).

Stand description and the latest results: [`test/load/README.md`](test/load/README.md). `make load` runs on Linux only (RSS and listening ports come from `/proc`; elsewhere it stops with `load tests need Linux (/proc)` before building anything) and needs docker, Go and `ulimit -n 65536`. Each test in `LOAD_TESTS` (default `L1 L2 L3 L4`) runs on a fresh `werbot-cluster` stand, removed after it. L1: 5 000 SSH sessions through the pool, 1 KB/s each for 10 minutes, beside one `werbot connect` with 500 TCP connections at 1 KB/s (`-load.tcp`) and 100 UDP bindings on local ports `20000`+ at one datagram a second (`-load.udp`, one flow each); no errors (an echo missing for 10 s counts), connect p99 ≤ 1 s, spools empty and every session ended with its recording finished within 5 minutes, every recording verified. The mix ramps first and reaches steady state (every connection echoed once, bounded by mix size/`-load.rate` + 30 s; a timeout is a FAIL row) before the measured SSH ramp starts; the `mix ramp` row is a reference. The harness dials nginx on its bridge IP (`10.231.0.10`), not the docker-proxy published port. L2: 200 simulated gateway instances (tunnel stream only) enrolled by one join token in a pool of 10 000 resources, each bound to the user; a new binding reaches every instance within 30 s, p99 ≤ 5 s. L3: L1 with `server1` killed halfway; no session breaks. L4: 2 500 idle sessions per gateway for 5 minutes, each holding a gateway session slot; the gateways listen only on `2222` and `9090`. Every 5 s (L1 also through the drain) the test samples RSS against the budgets of 200 MB + 256 kB per active session (gateway; for every session kind, though a UDP session with 32 flows takes about 512 KiB) and 300 MB + 64 kB per tunnel stream (server), and Postgres connections per server replica (≤ `pool_max_conns` + 2). Each criterion prints a `RESULT | <load> | <metric> | <criterion> | <result> | PASS/FAIL` line. The SSH target and the TCP/UDP echo targets run in the test process and listen on the Docker bridge address (`host.docker.internal` of the gateways), so a host firewall must let the Docker networks reach it. A smaller run: `make load LOAD_ARGS='-load.sessions=100 -load.duration=1m -load.idle=1m -load.rate=50 -load.tcp=50 -load.udp=10 -load.instances=20 -load.resources=500'`. `go test -tags load -short ./test/load/` runs only the helper tests, on any OS.

Modules built on `sdk` can call `Host.Subscribe(channel, fn)` in `Init` to receive Postgres NOTIFY on the server's shared LISTEN connection (the payload is unauthenticated: validate it).

Go tools (buf, protoc-gen-go, protoc-gen-connect-go, sqlc) live in `tools/go.mod`: `go tool -modfile=tools/go.mod buf ...`. The first sqlc build compiles `pg_query_go` with cgo (needs a C compiler). Generated `dbq` code is committed and never edited by hand; CI fails when `go tool -modfile=tools/go.mod sqlc diff` is not empty.

## License

The core is licensed under AGPL-3.0-only (see `LICENSE`). `sdk/`, `proto/` and `web/` are licensed under Apache-2.0.
