# Werbot

Werbot is an open-source access gateway that authorizes every command from people and AI agents to servers and databases, records everything, and never hands out a single password.

## Status

Pre-release (v0.x): breaking changes are possible, see [docs/upgrading.md](docs/upgrading.md).

## Quick start (5 minutes)

You need Docker with Compose v2. Run in an empty directory:

```sh
export WERBOT_VERSION=v0.1.1
mkdir werbot && cd werbot
curl -fsSLO https://raw.githubusercontent.com/werbot/werbot/$WERBOT_VERSION/deploy/docker-compose.yaml
curl -fsSL https://raw.githubusercontent.com/werbot/werbot/$WERBOT_VERSION/deploy/.env.example -o .env
sed -i.bak \
  -e "s|^WERBOT_VERSION=.*|WERBOT_VERSION=$WERBOT_VERSION|" \
  -e "s|^WERBOT_SECRET_KEY=.*|WERBOT_SECRET_KEY=$(openssl rand -base64 32)|" \
  -e "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$(openssl rand -hex 24)|" \
  .env && rm .env.bak
docker compose up -d --wait
```

On a remote server the login over plain `http://<ip>:8080` does not work (the session cookie is `Secure`): use an SSH tunnel or a TLS proxy, see [docs/install.md](docs/install.md#remote-access).

Open http://localhost:8080 and create the first account. What runs, ports and volumes: [docs/install.md](docs/install.md#docker-compose).

## Client quick start

Set the SSH address of the pool (Gateways page, pool details; `localhost:2222` for the quick start above), add a resource and bind it to yourself. Then run `werbot login --server http://localhost:8080` on your computer and continue with [docs/cli.md](docs/cli.md).

AI agents (Claude Code, Codex, Cursor) connect through `werbot mcp`: they run commands as a run of your account, under a read-only baseline, with no SSH key or password of their own ([docs/agents.md](docs/agents.md)).

## Other ways to install

- Binary and systemd: [docs/install.md](docs/install.md#binary).
- Gateway on another host: [docs/gateway.md](docs/gateway.md).
- Web UI on its own host and domain: [docs/web.md](docs/web.md).
- Several server replicas and gateway pools: [docs/scaling.md](docs/scaling.md#cluster-example).

## Verify downloads

Releases and images are signed with cosign (keyless) and ship SBOMs. Commands: [docs/security.md](docs/security.md#verifying-releases).

## Documentation

- [install.md](docs/install.md): Docker Compose, remote access, binary.
- [gateway.md](docs/gateway.md): pools, join tokens, instances, SSH address, firewall, connections.
- [web.md](docs/web.md): the web UI on its own host (`werbot web`).
- [configuration.md](docs/configuration.md): every environment variable.
- [upgrading.md](docs/upgrading.md): pre-1.0 policy, backup, upgrade order.
- [security.md](docs/security.md): reporting, release verification, threat model, join tokens, CLI sessions and user certificates.
- [scaling.md](docs/scaling.md): replicas, load balancing, PostgreSQL, Kubernetes, metrics, cluster example.
- [failure-modes.md](docs/failure-modes.md): what happens when a part fails.
- [connectors.md](docs/connectors.md): adding a target type.
- [cli.md](docs/cli.md): `werbot login`, profiles, certificates, `ssh`, `ssh-config`, `proxy`, access requests.
- [jit.md](docs/jit.md): just-in-time access: requests, approval policies, break-glass, grants, locks, lock mode.
- [policies.md](docs/policies.md): command policies: risk classes, class overrides, command rules, folder rules, the command console, custom Cedar, command approvals, output masking.
- [ssh-hardening.md](docs/ssh-hardening.md): host permissions under the Werbot rules: accounts per access level, ACLs, SFTP-only accounts, narrow `sudo`.
- [connect.md](docs/connect.md): the `werbot connect` client, also over WebSocket.
- [vnet.md](docs/vnet.md): `werbot connect --vnet`: an address and a DNS name per resource, TCP and UDP, on Linux and macOS.
- [web-terminal.md](docs/web-terminal.md): the web terminal, the live view of a running session, Terminate, close codes and limits.
- [sso.md](docs/sso.md): single sign-on with OpenID Connect: IdP setup, claims mapping, provisioning and linking, the hourly check, IdP outages.
- [mfa.md](docs/mfa.md): TOTP, passkeys and recovery codes, `require_mfa`, MFA rules for SSH and the web terminal, fresh MFA for admin actions, rate limits.
- [agents.md](docs/agents.md): AI agents: `werbot mcp`, `werbot agent run`, sponsors, runs, the baseline, limits, the kill switch, agent profiles.
- [migrate-from-boundary.md](docs/migrate-from-boundary.md): how HashiCorp Boundary objects map to Werbot, what you move by hand.

The same pages are on the documentation site: https://docs.werbot.com. A demo stack is in [`deploy/demo/`](deploy/demo/). The MCP Registry entry is [`server.json`](server.json).

Release notes (the changelog): [GitHub Releases](https://github.com/werbot/werbot/releases).

## Security

Report vulnerabilities privately: [docs/security.md](docs/security.md#reporting-a-vulnerability).

## Development

Requirements:

- Go 1.27
- Bun 1.3
- Docker (local Postgres 18 and the compose stack)

### Quick start from source

```sh
make dev-db                      # Postgres 18 on localhost:27432 (postgres/password), Mailpit SMTP :27025, UI http://localhost:27825
(cd web && bun install)          # web/node_modules is required by `make proto`
make proto                       # buf lint + generate (Go and TypeScript)
make build                       # builds the UI and bin/werbot
export WERBOT_SECRET_KEY="$(openssl rand -base64 32)"  # generate once and keep: it decrypts resource credentials and queued email jobs
WERBOT_ENV=development \
WERBOT_DATABASE_URL='postgres://postgres:password@localhost:27432/postgres?sslmode=disable' \
WERBOT_SMTP_HOST=localhost WERBOT_SMTP_PORT=27025 WERBOT_SMTP_FROM=werbot@localhost \
WERBOT_PUBLIC_URL=http://localhost:8080 ./bin/werbot server
```

`WERBOT_ENV=development` is for local plain-http runs: the session cookie is `werbot_session` without `Secure`, STARTTLS is used only when the SMTP server offers it (Mailpit does not), and without SMTP the log shows the full email text with reset and invite links. Production mode uses `__Host-werbot_session` with `Secure`, which browsers drop over plain http on any host but `localhost` (Safari drops it on `localhost` too).

Commands: `./bin/werbot <server|gateway|web|login|logout|status|ls|ssh|ssh-config|proxy|connect|request|requests|approve|deny|migrate|version|healthcheck>`.

- `server` runs migrations (schema and River job queue) and creates the internal gateway CA and the ed25519 key that signs session recording manifests on first start, under an advisory lock. Then it serves the API (and the UI when embedded), the gateway tunnel on `WERBOT_GATEWAY_ADDR` and runs the River job client.
- `server --with-gateway` also runs a gateway instance in the same process. Each replica joins the system pool `local` of the first organization (by creation time) with a system join token (max uses 1); before `Setup` it checks for an organization every 5s. It reaches its own server over loopback (`http://127.0.0.1:<WERBOT_HTTP_ADDR port>`, tunnel `https://127.0.0.1:<WERBOT_GATEWAY_ADDR port>`), so both addresses must accept connections on 127.0.0.1: leave the host empty (`:8080`) or use `0.0.0.0` or `127.0.0.1`, not a single non-loopback IP. It keeps its identity in `WERBOT_DATA_DIR/gateway` (or `WERBOT_GATEWAY_DATA_DIR`) and reads the other `WERBOT_GATEWAY_*` variables ([docs/configuration.md](docs/configuration.md#gateway-werbot-gateway)); it refuses loopback, link-local (including cloud metadata), multicast and unspecified targets and the resolved addresses of the database host unless `WERBOT_LOCAL_POOL_ALLOW_LOOPBACK=true` ([docs/configuration.md](docs/configuration.md#embedded-gateway)), so a same-host database needs that opt-in, while the host's LAN addresses stay reachable (this applies to `telnet`, `tcp` and `udp` sessions and to health probes; `ssh` resources keep their own dial, so a loopback `ssh` resource connects but its health shows `unhealthy`); its metrics go to the server's `/metrics`. Database and enrollment errors before the instance is enrolled are logged and retried (the wait doubles up to 5 minutes, a new system token each time); a busy SSH port or a failure after enrollment stops the server. On shutdown the gateway drains its sessions before the server closes the tunnel, so allow `WERBOT_GATEWAY_DRAIN_TIMEOUT` plus about 40s for the stop (compose: `stop_grace_period: 70s`).
- Upgrade the server and every standalone gateway to the same release together: the server requires the session kind (`terminal`, `exec`, `stream`, `datagram`) that older gateways do not send, so their new sessions fail until they are upgraded.
- `gateway` runs a standalone gateway instance (`./bin/werbot gateway --help` lists the flags: `--server`, `--join-token`, `--data-dir`, `--ssh-addr`, `--proxy-protocol`, `--name`, `--idle-timeout`, `--http-addr`, `--drain-timeout`, `--max-sessions`, `--spool-max`, `--max-duration`, and the flag-only `--insecure-enroll`).
- Web UI `/gateways` (owners and admins): create pools, see instances (online within 45s of the last heartbeat) and manage join tokens. "Create join token" shows the `WERBOT_GATEWAY_JOIN_TOKEN=<token> werbot gateway --server <origin>` command once (the env form keeps the token out of `ps` output and shell history); the token is not shown again. The resource form shows a pool select when more than one pool exists.
- Web UI `/sessions` (every user; members see only their own): session list filtered by status and period (owners and admins also by user and resource) and, per session, its details, the recording integrity badge (`Recording verified` when `X-Werbot-Recording-Verified` is `true`, a failure banner otherwise, a notice while the recording is unfinished) and an asciinema player of `/api/recordings/{session_id}.cast`. The list shows the command a session ran with its classes and bytes up and down; the details add kind, target and, for datagram sessions, datagram counts. The resource pages offer `telnet`, `tcp` and `udp` forms (port ranges like `22, 8000-8100`), idle timeout and max duration in minutes, a health dot, connect hints per session kind, and ports on bindings of `tcp` and `udp` resources.
- Web UI `/requests` (every user): new request (resource, duration, reason, break-glass), tabs My requests, To approve (with the pending count) and All (owners and admins), approve and deny with a comment, grants with revoke and break-glass review. `/policies/approvals` and `/locks` (owners and admins) manage approval policies and locks.
- Web UI `/audit` (owners and admins): the audit log filtered by action, actor and period; the filter lives in the query string (`/audit?action=session&actor=<user id>&from=YYYY-MM-DD&to=YYYY-MM-DD`, `to` inclusive). A click on an event opens its fields and `data`.
- `connect` opens local ports that reach `tcp`, `udp` and other port resources through a gateway ([docs/connect.md](docs/connect.md)); `connect --vnet` gives each resource an address and a name `<resource>.<org>.werbot` instead (Linux and macOS, root or `CAP_NET_ADMIN`; [docs/vnet.md](docs/vnet.md)).
- `request`, `requests`, `approve`, `deny` ask for just-in-time access to a resource, list requests and decide them ([docs/cli.md](docs/cli.md#access-requests)); `ls` marks resources you can only request with `ACCESS` `request`.
- `login`, `logout`, `status`, `ls`, `ssh`, `ssh-config` sign the CLI in with a browser or device flow, keep a profile and a short-lived SSH certificate in `WERBOT_HOME`, and open sessions with the system `ssh` ([docs/cli.md](docs/cli.md)); `connect` uses the profile when there is one. `proxy` connects stdin and stdout to a gateway (over SSH or its WebSocket URL) for an OpenSSH `ProxyCommand`.
- `mcp`, `agent run`, `agents` run an AI agent as a run of your account (MCP on stdio, or a program with `ssh` set up) and manage your agents and agent profiles ([docs/agents.md](docs/agents.md)).
- `migrate` runs migrations (schema and River job queue) and exits.
- `web` serves the UI and proxies the API to a separate `server`.
- `version` prints the build version.
- `healthcheck` probes `GET http://<WERBOT_HTTP_ADDR>/healthz` of a local server for 3s (empty or unspecified host becomes `127.0.0.1`) and exits 0 on `200`, 1 otherwise; it reads no other config, so it works without `WERBOT_DATABASE_URL`.

The API covers identity (auth, profile, SSH keys, members, teams) and, for owners and admins:

- `ResourceService`: resources of registered types (`ssh`, `telnet` (default port 23, login optional, `password` credential), `tcp` (host plus 1–16 non-overlapping port ranges), `udp` (like `tcp` plus `flow_idle`, default `2m`, allowed `1s`–`1h`); `tcp` and `udp` take no credentials) with labels, optional per-resource `idle_timeout` (`1m`–`24h`) and `max_duration` (`1m`–`168h`; unset means the gateway default), a `health` state (`healthy` when an online gateway instance of the pool probed the resource successfully within 3 minutes, `unhealthy` when all such fresh probes failed (with the latest error), otherwise `unknown`), write-only credentials (sealed with `WERBOT_SECRET_KEY`) and pinned SSH host keys.
- Gateway health checks: each pool resource with an endpoint (types `ssh`, `tcp`, ...; not `udp`, not with `NoHealth` or an empty host) is probed by 2 gateway instances chosen by rendezvous hashing over the online instances of the pool. A probe is a TCP connect (timeout 5s, at most 8 at a time) to the host and the first port of each range, first pass at start, then every 60s ±20%. Each pass sends every result. A probe uses the same dialer as sessions, so with local targets denied a loopback target reports `unhealthy` with the refusal. Metric: `werbot_gateway_health_checks_total{result="ok"|"fail"}`.
- `AccessService`: bindings that grant a user or team one resource or every resource matching a label selector. A binding may list up to 16 target ports or ranges (`ports`, 1–65535, no overlap): stream and datagram sessions (TCP, UDP, `ssh -L`) then reach only those ports, while terminal and exec sessions are not restricted. A binding without ports allows every port of the resource.
- `ApprovalPolicyService` (owners and admins): approval policies that say who may request access to which resources, who approves and how (`manual` or `auto`, `max_duration`, `require_reason`, `four_eyes`, `break_glass`) ([docs/jit.md](docs/jit.md#approval-policies)).
- `AccessRequestService` (every member): access requests, approvals, grants and break-glass reviews. Nobody approves their own request, and a pending request that times out is `expired`, never approved ([docs/jit.md](docs/jit.md#access-requests)).
- `LockService` (owners and admins): locks on a user, team, resource, grant, CLI session, agent, agent run, the agents of a user or all agents; a lock beats every binding and grant and ends matching sessions within 2 s ([docs/jit.md](docs/jit.md#locks)).
- Organization settings (owners and admins, web UI `/settings`): CLI session lifetimes, grant and request lengths, `lock_mode` and `mask_output` (`agents` by default, `all` or `none`: whose exec output the gateway masks) ([docs/jit.md](docs/jit.md#organization-settings)).
- `PolicyService`: per-resource network, country and schedule (UTC) rules, and `CheckAccess` to test a decision (optional `port`; 0 tests a terminal or exec session). Decisions use Cedar and fail closed.
- `PolicyService` command policies (owners and admins, web UI `/policies/commands` and `/policies/custom`): the gateway sorts each exec command into risk classes, and class overrides, command rules (`deny` or `approval`) and custom Cedar policies decide it; a command under `approval` waits until an approver decides on `/requests`. Details: [docs/policies.md](docs/policies.md).
- `PolicyService` folder and terminal rules (owners and admins, web UI `/policies/paths` and `/policies/terminal`): folder rules limit the paths a command may touch, and terminal rules open interactive shells as a command console that checks every line, or refuse them ([docs/policies.md](docs/policies.md#folders)).
- `AgentService`, `AgentProfileService` (web UI `/agents`): agents with their limits and status, runs with their timeline (tool calls, sessions, approvals, denials), resume and end, and agent profiles for another OS user or a container ([docs/agents.md](docs/agents.md)).
- `SessionService`: session list (filters by user, resource, period, status; paging; each session carries its `kind` (`terminal`, `exec`, `stream`, `datagram`), `target`, the `command` it ran (if any) with its classes, and byte and datagram counters), session card and `VerifyRecording` (hash chain and signature check). Members see only their own sessions. `TerminateSession` ends a running session within 2 seconds (owners and admins any session, a member their own; the user sees `Session closed: terminated`, the reason stays in the audit log).
- `IdentityProviderService` (owners and admins, web UI `/settings/sso`): OpenID Connect single sign-on with claims mapping to teams and roles, verified-email linking and an hourly IdP check that ends sessions of users the IdP refused ([docs/sso.md](docs/sso.md)).
- `MFAService`, `MFAChallengeService` (web UI `/profile/mfa`, `/policies/mfa`): TOTP, passkeys and recovery codes; `require_mfa` off, admins or all; MFA rules that ask a second factor on SSH and in the web terminal; fresh MFA for admin actions; rate limits on login, password reset, MFA and the token endpoint ([docs/mfa.md](docs/mfa.md)).
- Web terminal and live view (web UI buttons **Open terminal**, **Watch**, **Terminate**): WebSockets `GET /api/terminal?resource=<name>&cols=&rows=` and `GET /api/sessions/{id}/live`, only with a session (cookie or a person's CLI token) and the `Origin` of `WERBOT_WEB_URL`, never for agent tokens. The terminal goes through the gateway like SSH (same access rules, recording and audit, `front: web`); the live view is read-only and audited as `session.observe`. Server replicas bridge streams over the mTLS listener `WERBOT_INTERNAL_ADDR` (default `:8444`). Details, close codes and limits: [docs/web-terminal.md](docs/web-terminal.md).
- `GET /api/recordings/{session_id}.cast` (session cookie): the session recording as one asciicast v2 file, for anyone who may see the session. The header `X-Werbot-Recording-Verified` is `true` only when the served bytes passed the hash chain and signature check; an active, unfinished or tampered recording is served as far as it goes with `false`. The response is an attachment (`<session_id>.cast`, `nosniff`); a chunk that inflates past 1 MiB (a gateway writes at most 64 KiB) ends the stream there. A session without a recording, or one the caller may not see, is `404`; an unreachable recording store is `503`. A replica runs at most 4 replays at once; the fifth gets `503` `replay busy, try again` with `Retry-After: 5`. The server checks the whole recording before it sends the headers, so a very large recording replayed through `werbot web` can hit its 30 s upstream header timeout (`503`).
- `AuditService`: read-only event journal (`ListEvents`, `GetEvent`) for owners and admins; other roles get `permission_denied`. `ListEvents` filters by `action` (a domain such as `session` matches `session.*`, a full action matches itself), actor, target and period `[from, to)`, newest first, `page_size` 50 by default and 200 at most (a larger value is clamped). Events without an organization are never shown. `session.start` events carry the session's `kind` and `target` in `data`.
- `GatewayService`: gateway pools, their instances and join tokens. A gateway joins with `GatewayEnrollService/Enroll` (the join token is its only credential) and then talks to the server only over the mTLS tunnel (`https://<WERBOT_PUBLIC_URL host>:<WERBOT_GATEWAY_ADDR port>`). The server never connects to a gateway.

Maintenance jobs run on the River `maintenance` queue: `session.end_lost` (every 30s) closes sessions of gateways that stopped reporting, `gateway.gc` (hourly) removes instances whose certificate expired over 24h ago and join tokens expired over 7 days ago, `approval.expire` (every minute) marks pending requests past their deadline `expired`, `agent_run.expire` (every minute) ends agent runs past their expiry.

Background jobs on the River `feeds` queue (one worker) need outbound HTTPS from the server:

- `blocklist.refresh` (on start, then daily) downloads the FireHOL level 1 list from `raw.githubusercontent.com`; listed IPs are always denied.
- `geoip.refresh` (on start, then daily at 04:00 UTC) downloads the monthly country database from `WERBOT_GEOIP_URL`, by default DB-IP Country Lite from `download.db-ip.com`; MaxMind GeoLite2 and IPinfo Lite also work ([configuration.md](docs/configuration.md#geoip-database)). The database is stored in Postgres (no files on disk), and every server loads it on start. Without it, country rules deny. IP geolocation by [DB-IP](https://db-ip.com) (default database).

A failed download keeps the previous data.

### Docker image from source

```sh
docker build -f deploy/Dockerfile -t ghcr.io/werbot/werbot:dev .
cd deploy && cp .env.example .env
```

Fill `.env` as in [docs/install.md](docs/install.md#docker-compose) with `WERBOT_VERSION=dev`, then run `docker compose up -d --wait`.

### Tests and tools

```sh
make test   # needs `make dev-db`
make lint
make release-check     # goreleaser config check
make release-snapshot  # local snapshot build into dist/: archives, checksums and Docker images (needs docker and syft)
make e2e    # Playwright e2e on DB `werbot_e2e`, no internet feeds (the server runs with `HTTPS_PROXY` pointing at a closed port; needs `make dev-db`, Go, ssh and ssh-keygen; first run: `cd web && bunx playwright install chromium`)
make e2e-network  # Docker Compose e2e: SSH, telnet, TCP and UDP through a gateway pool to targets on an internal-only network, then the werbot CLI
make e2e-cluster  # Go scenarios (test/cluster, tag `cluster`: topology, cross-replica login, binding propagation, bootstrap, GeoIP reload, Postgres restart, shared pool host key, replica kill, lost instance, gateway max sessions, servers down fail-closed, S3 outage, join token revocation and exhaustion, separate web UI behind `werbot web` replicas, `werbot connect` TCP and UDP through the pool with an instance kill; phase 2: web terminal, live view and terminate across replicas, a killed bridge replica, relay unavailable when the internal listener is down, command approval on the other replica, grant expiry while a replica is killed, the agent action limit across replicas, `werbot login --device` through a separate web replica) on a stand of 2 server, 2 web and 2 gateway replicas behind nginx
make load         # manual load tests L1–L5 (test/load, tag `load`) on the same stand; Linux only
make redteam      # red-team scenarios: Go (test/redteam, tag `redteam`) on the cluster stand, then the browser SSO scenario (web/redteam) with Keycloak; needs `make dev-db` and free ports 18080 and 18180; uses the `10.231.x` subnets of `make e2e-cluster`, so run one stand per host
make fuzz         # Go fuzz targets of the command classifier, the datagram frame and the stream frame
make site         # builds the documentation site into `site/dist` from `docs/`
(cd web && bun run test:unit)  # Vitest unit tests for the web UI
make sqlc   # regenerates internal/<domain>/dbq from migrations/ and internal/<domain>/queries.sql
```

`make e2e` starts `werbot server --with-gateway` (gateway SSH on `127.0.0.1:2222`, a fresh `WERBOT_DATA_DIR` per run) and the SSH target `go run ./internal/e2e/sshtarget` on `127.0.0.1:2022`; both ports must be free.

`make e2e-network` needs docker, curl, jq, ssh, ssh-keygen, Go, nc and dig. It builds the `werbot-e2e:local` image and the TCP/UDP echo target (`internal/e2e/echotarget`, into the gitignored `deploy/e2e/.bin`), publishes ports `8080`, `8081` and `2222` on localhost, and removes the `werbot-e2e` project (containers, networks, volumes) on exit. The targets sit on the internal-only network that only the gateway pool joins: an OpenSSH server, a busybox `telnetd` with a real `/bin/login` (`router`), the echo target on TCP and UDP port 7 (`echo`) and CoreDNS answering `probe.werbot.test` (`dns`). The scenario checks SSH through the pool and a gateway restart; a telnet session through the SSH bridge (the router password never reaches the user, the session is `terminal` and its recording verifies); `werbot connect` with `echo`, `router` and `dns` on `127.0.0.1:10007`, `10023` (telnet stream with autologin) and `10053` (UDP, two flows); a forward to a port outside the resource refused and audited as `session.deny` with `target_not_allowed`; and the health of the TCP and telnet resources turning `healthy` within 180s while the UDP one stays `unknown`. Then `deploy/e2e/cli.sh` sets the pool's SSH address, runs `werbot login --device` (the code approved over the API), `werbot ls`, `werbot ssh` and OpenSSH through `werbot ssh-config`, revokes the CLI session and expects the next `werbot ssh` and OpenSSH connection to fail and the audit log to hold `cli.login` (`flow` `device`) and `cli_session.revoke`. It also uses local ports `10007`, `10023` and `10053`.

`make e2e-cluster` needs docker, ssh and Go. It builds the `werbot:cluster` image and runs `go test -tags cluster ./test/cluster/...` against the compose project `werbot-cluster` (override with `CLUSTER_PROJECT`): Postgres, SeaweedFS (S3 recordings), `server1`/`server2`, `web1`/`web2`, nginx and the gateway pool `gw1`/`gw2`, with the targets on an internal-only network. Every published port is an ephemeral port on `127.0.0.1`; the networks use the fixed subnets `10.231.0.0/24` and `10.231.1.0/24`, so run one stand per host at a time. The stand is removed at the end; on failure its logs go to `test/cluster/compose.log` (gitignored, test-only credentials). It also builds `werbot` for the host into the gitignored `test/cluster/.bin`: the `werbot connect` scenario carries TCP to the echo target and DNS to CoreDNS on free local ports, checks the `stream` and `datagram` sessions and the health probes of both instances, then kills the instance of the DNS session and expects DNS to answer again within 30 s. The lost-instance scenario waits up to 3 minutes for `gateway_lost`, after both server replicas have run for 2 minutes; the S3 outage scenario stops SeaweedFS for 1 minute. `go test ./...` without the tag skips `test/`. CI runs it as the `e2e-cluster` job on every push and pull request to `main` (60-minute limit; a newer push cancels the run it supersedes; on failure `compose.log` is uploaded as the `e2e-cluster-logs` artifact).

Stand description and the latest results: [`test/load/README.md`](test/load/README.md). `make load` runs on Linux only (RSS and listening ports come from `/proc`; elsewhere it stops with `load tests need Linux (/proc)` before building anything) and needs docker, Go and `ulimit -n 65536`. Each test in `LOAD_TESTS` (default `L1 L2 L3 L4 L5`) runs on a fresh `werbot-cluster` stand, removed after it; the stand logs of each failed test go to `test/cluster/compose.log`, under a `== <test>` line. L1: 5 000 SSH sessions through the pool, 1 KB/s each for 10 minutes, beside one `werbot connect` with 500 TCP connections at 1 KB/s (`-load.tcp`) and 100 UDP bindings on local ports `20000`+ at one datagram a second (`-load.udp`, one flow each); no errors (an echo missing for 10 s counts), connect p99 ≤ 1 s, spools empty and every session ended with its recording finished within 5 minutes, every recording verified. The mix ramps first and reaches steady state (every connection echoed once, bounded by mix size/`-load.rate` + 30 s; a timeout is a FAIL row) before the measured SSH ramp starts; the `mix ramp` row is a reference. The harness dials nginx on its bridge IP (`10.231.0.10`), not the docker-proxy published port. L2: 200 simulated gateway instances (tunnel stream only) enrolled by one join token in a pool of 10 000 resources, each bound to the user; a new binding reaches every instance within 30 s, p99 ≤ 5 s. L3: L1 with `server1` killed halfway; no session breaks. L4: 2 500 idle sessions per gateway for 5 minutes, each holding a gateway session slot; the gateways listen only on `2222` and `9090`. L5: agent `ssh_exec` calls through `werbot mcp` runs at 1 000 calls a minute, then web terminals relayed across the server replicas (criteria in [`test/load/README.md`](test/load/README.md#l5-agents-and-relays)). Every 5 s (L1 also through the drain) the test samples RSS against the budgets of 200 MB + 256 kB per active session (gateway; for every session kind, though a UDP session with 32 flows takes about 512 KiB) and 300 MB + 64 kB per tunnel or relay stream (server), and Postgres connections per server replica (≤ `pool_max_conns` + 2). Each criterion prints a `RESULT | <load> | <metric> | <criterion> | <result> | PASS/FAIL` line. The SSH target and the TCP/UDP echo targets run in the test process and listen on the Docker bridge address (`host.docker.internal` of the gateways), so a host firewall must let the Docker networks reach it. A smaller run: `make load LOAD_ARGS='-load.sessions=100 -load.duration=1m -load.idle=1m -load.rate=50 -load.tcp=50 -load.udp=10 -load.instances=20 -load.resources=500'`. `go test -tags load -short ./test/load/` runs only the helper tests, on any OS; CI runs it with `-race` in the `go` job.

Modules built on `sdk` can call `Host.Subscribe(channel, fn)` in `Init` to receive Postgres NOTIFY on the server's shared LISTEN connection (the payload is unauthenticated: validate it).

Go tools (buf, protoc-gen-go, protoc-gen-connect-go, sqlc) live in `tools/go.mod`: `go tool -modfile=tools/go.mod buf ...`. The first sqlc build compiles `pg_query_go` with cgo (needs a C compiler). Generated `dbq` code is committed and never edited by hand; CI fails when `go tool -modfile=tools/go.mod sqlc diff` is not empty.

## License

The core is licensed under AGPL-3.0-only (see `LICENSE`). `sdk/`, `proto/` and `web/` are licensed under Apache-2.0.
