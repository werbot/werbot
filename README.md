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

## Other ways to install

- Binary and systemd: [docs/install.md](docs/install.md#binary).
- Gateway on another host: [docs/gateway.md](docs/gateway.md).
- Web UI on its own host and domain: [docs/web.md](docs/web.md).
- Several server replicas and gateway pools: [docs/scaling.md](docs/scaling.md#cluster-example).

## Verify downloads

Releases and images are signed with cosign (keyless) and ship SBOMs. Commands: [docs/security.md](docs/security.md#verifying-releases).

## Documentation

- [install.md](docs/install.md): Docker Compose, remote access, binary.
- [gateway.md](docs/gateway.md): pools, join tokens, instances, firewall, connections.
- [web.md](docs/web.md): the web UI on its own host (`werbot web`).
- [configuration.md](docs/configuration.md): every environment variable.
- [upgrading.md](docs/upgrading.md): pre-1.0 policy, backup, upgrade order.
- [security.md](docs/security.md): reporting, release verification, threat model, join tokens.
- [scaling.md](docs/scaling.md): replicas, load balancing, PostgreSQL, Kubernetes, metrics, cluster example.
- [failure-modes.md](docs/failure-modes.md): what happens when a part fails.
- [connectors.md](docs/connectors.md): adding a target type.
- [connect.md](docs/connect.md): the `werbot connect` client.

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

Commands: `./bin/werbot <server|gateway|web|connect|migrate|version|healthcheck>`.

- `server` runs migrations (schema and River job queue) and creates the internal gateway CA and the ed25519 key that signs session recording manifests on first start, under an advisory lock. Then it serves the API (and the UI when embedded), the gateway tunnel on `WERBOT_GATEWAY_ADDR` and runs the River job client.
- `server --with-gateway` also runs a gateway instance in the same process. Each replica joins the system pool `local` of the first organization (by creation time) with a system join token (max uses 1); before `Setup` it checks for an organization every 5s. It reaches its own server over loopback (`http://127.0.0.1:<WERBOT_HTTP_ADDR port>`, tunnel `https://127.0.0.1:<WERBOT_GATEWAY_ADDR port>`), so both addresses must accept connections on 127.0.0.1: leave the host empty (`:8080`) or use `0.0.0.0` or `127.0.0.1`, not a single non-loopback IP. It keeps its identity in `WERBOT_DATA_DIR/gateway` (or `WERBOT_GATEWAY_DATA_DIR`) and reads the other `WERBOT_GATEWAY_*` variables ([docs/configuration.md](docs/configuration.md#gateway-werbot-gateway)); it refuses loopback, link-local (including cloud metadata), multicast and unspecified targets and the resolved addresses of the database host unless `WERBOT_LOCAL_POOL_ALLOW_LOOPBACK=true` ([docs/configuration.md](docs/configuration.md#embedded-gateway)), so a same-host database needs that opt-in, while the host's LAN addresses stay reachable (this applies to `telnet`, `tcp` and `udp` sessions and to health probes; `ssh` resources keep their own dial, so a loopback `ssh` resource connects but its health shows `unhealthy`); its metrics go to the server's `/metrics`. Database and enrollment errors before the instance is enrolled are logged and retried (the wait doubles up to 5 minutes, a new system token each time); a busy SSH port or a failure after enrollment stops the server. On shutdown the gateway drains its sessions before the server closes the tunnel, so allow `WERBOT_GATEWAY_DRAIN_TIMEOUT` plus about 40s for the stop (compose: `stop_grace_period: 70s`).
- Upgrade the server and every standalone gateway to the same release together: the server requires the session kind (`terminal`, `exec`, `stream`, `datagram`) that older gateways do not send, so their new sessions fail until they are upgraded.
- `gateway` runs a standalone gateway instance (`./bin/werbot gateway --help` lists the flags: `--server`, `--join-token`, `--data-dir`, `--ssh-addr`, `--proxy-protocol`, `--name`, `--idle-timeout`, `--http-addr`, `--drain-timeout`, `--max-sessions`, `--spool-max`, `--max-duration`, and the flag-only `--insecure-enroll`).
- Web UI `/gateways` (owners and admins): create pools, see instances (online within 45s of the last heartbeat) and manage join tokens. "Create join token" shows the `WERBOT_GATEWAY_JOIN_TOKEN=<token> werbot gateway --server <origin>` command once (the env form keeps the token out of `ps` output and shell history); the token is not shown again. The resource form shows a pool select when more than one pool exists.
- Web UI `/sessions` (every user; members see only their own): session list filtered by status and period (owners and admins also by user and resource) and, per session, its details, the recording integrity badge (`Recording verified` when `X-Werbot-Recording-Verified` is `true`, a failure banner otherwise, a notice while the recording is unfinished) and an asciinema player of `/api/recordings/{session_id}.cast`. The list shows bytes up and down; the details add kind, target and, for datagram sessions, datagram counts. The resource pages offer `telnet`, `tcp` and `udp` forms (port ranges like `22, 8000-8100`), idle timeout and max duration in minutes, a health dot, connect hints per session kind, and ports on bindings of `tcp` and `udp` resources.
- Web UI `/audit` (owners and admins): the audit log filtered by action, actor and period; the filter lives in the query string (`/audit?action=session&actor=<user id>&from=YYYY-MM-DD&to=YYYY-MM-DD`, `to` inclusive). A click on an event opens its fields and `data`.
- `connect` opens local ports that reach `tcp`, `udp` and other port resources through a gateway ([docs/connect.md](docs/connect.md)).
- `migrate` runs migrations (schema and River job queue) and exits.
- `web` serves the UI and proxies the API to a separate `server`.
- `version` prints the build version.
- `healthcheck` probes `GET http://<WERBOT_HTTP_ADDR>/healthz` of a local server for 3s (empty or unspecified host becomes `127.0.0.1`) and exits 0 on `200`, 1 otherwise; it reads no other config, so it works without `WERBOT_DATABASE_URL`.

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
make e2e-network  # Docker Compose e2e: SSH, telnet, TCP and UDP through a gateway pool to targets on an internal-only network
make e2e-cluster  # Go scenarios (test/cluster, tag `cluster`: topology, cross-replica login, binding propagation, bootstrap, GeoIP reload, Postgres restart, shared pool host key, replica kill, lost instance, gateway max sessions, servers down fail-closed, S3 outage, join token revocation and exhaustion, separate web UI behind `werbot web` replicas, `werbot connect` TCP and UDP through the pool with an instance kill) on a stand of 2 server, 2 web and 2 gateway replicas behind nginx
make load         # manual load tests L1–L4 (test/load, tag `load`) on the same stand; Linux only
(cd web && bun run test:unit)  # Vitest unit tests for the web UI
make sqlc   # regenerates internal/<domain>/dbq from migrations/ and internal/<domain>/queries.sql
```

`make e2e` starts `werbot server --with-gateway` (gateway SSH on `127.0.0.1:2222`, a fresh `WERBOT_DATA_DIR` per run) and the SSH target `go run ./internal/e2e/sshtarget` on `127.0.0.1:2022`; both ports must be free.

`make e2e-network` needs docker, curl, jq, ssh, ssh-keygen, Go, nc and dig. It builds the `werbot-e2e:local` image and the TCP/UDP echo target (`internal/e2e/echotarget`, into the gitignored `deploy/e2e/.bin`), publishes ports `8080`, `8081` and `2222` on localhost, and removes the `werbot-e2e` project (containers, networks, volumes) on exit. The targets sit on the internal-only network that only the gateway pool joins: an OpenSSH server, a busybox `telnetd` with a real `/bin/login` (`router`), the echo target on TCP and UDP port 7 (`echo`) and CoreDNS answering `probe.werbot.test` (`dns`). The scenario checks SSH through the pool and a gateway restart; a telnet session through the SSH bridge (the router password never reaches the user, the session is `terminal` and its recording verifies); `werbot connect` with `echo`, `router` and `dns` on `127.0.0.1:10007`, `10023` (telnet stream with autologin) and `10053` (UDP, two flows); a forward to a port outside the resource refused and audited as `session.deny` with `target_not_allowed`; and the health of the TCP and telnet resources turning `healthy` within 180s while the UDP one stays `unknown`. It also uses local ports `10007`, `10023` and `10053`.

`make e2e-cluster` needs docker, ssh and Go. It builds the `werbot:cluster` image and runs `go test -tags cluster ./test/cluster/...` against the compose project `werbot-cluster` (override with `CLUSTER_PROJECT`): Postgres, SeaweedFS (S3 recordings), `server1`/`server2`, `web1`/`web2`, nginx and the gateway pool `gw1`/`gw2`, with the targets on an internal-only network. Every published port is an ephemeral port on `127.0.0.1`; the networks use the fixed subnets `10.231.0.0/24` and `10.231.1.0/24`, so run one stand per host at a time. The stand is removed at the end; on failure its logs go to `test/cluster/compose.log` (gitignored, test-only credentials). It also builds `werbot` for the host into the gitignored `test/cluster/.bin`: the `werbot connect` scenario carries TCP to the echo target and DNS to CoreDNS on free local ports, checks the `stream` and `datagram` sessions and the health probes of both instances, then kills the instance of the DNS session and expects DNS to answer again within 30 s. The lost-instance scenario waits up to 3 minutes for `gateway_lost`, after both server replicas have run for 2 minutes; the S3 outage scenario stops SeaweedFS for 1 minute. `go test ./...` without the tag skips `test/`. CI runs it as the `e2e-cluster` job on every push and pull request to `v2` (50-minute limit; a newer push cancels the run it supersedes; on failure `compose.log` is uploaded as the `e2e-cluster-logs` artifact).

Stand description and the latest results: [`test/load/README.md`](test/load/README.md). `make load` runs on Linux only (RSS and listening ports come from `/proc`; elsewhere it stops with `load tests need Linux (/proc)` before building anything) and needs docker, Go and `ulimit -n 65536`. Each test in `LOAD_TESTS` (default `L1 L2 L3 L4`) runs on a fresh `werbot-cluster` stand, removed after it; the stand logs of each failed test go to `test/cluster/compose.log`, under a `== <test>` line. L1: 5 000 SSH sessions through the pool, 1 KB/s each for 10 minutes, beside one `werbot connect` with 500 TCP connections at 1 KB/s (`-load.tcp`) and 100 UDP bindings on local ports `20000`+ at one datagram a second (`-load.udp`, one flow each); no errors (an echo missing for 10 s counts), connect p99 ≤ 1 s, spools empty and every session ended with its recording finished within 5 minutes, every recording verified. The mix ramps first and reaches steady state (every connection echoed once, bounded by mix size/`-load.rate` + 30 s; a timeout is a FAIL row) before the measured SSH ramp starts; the `mix ramp` row is a reference. The harness dials nginx on its bridge IP (`10.231.0.10`), not the docker-proxy published port. L2: 200 simulated gateway instances (tunnel stream only) enrolled by one join token in a pool of 10 000 resources, each bound to the user; a new binding reaches every instance within 30 s, p99 ≤ 5 s. L3: L1 with `server1` killed halfway; no session breaks. L4: 2 500 idle sessions per gateway for 5 minutes, each holding a gateway session slot; the gateways listen only on `2222` and `9090`. Every 5 s (L1 also through the drain) the test samples RSS against the budgets of 200 MB + 256 kB per active session (gateway; for every session kind, though a UDP session with 32 flows takes about 512 KiB) and 300 MB + 64 kB per tunnel stream (server), and Postgres connections per server replica (≤ `pool_max_conns` + 2). Each criterion prints a `RESULT | <load> | <metric> | <criterion> | <result> | PASS/FAIL` line. The SSH target and the TCP/UDP echo targets run in the test process and listen on the Docker bridge address (`host.docker.internal` of the gateways), so a host firewall must let the Docker networks reach it. A smaller run: `make load LOAD_ARGS='-load.sessions=100 -load.duration=1m -load.idle=1m -load.rate=50 -load.tcp=50 -load.udp=10 -load.instances=20 -load.resources=500'`. `go test -tags load -short ./test/load/` runs only the helper tests, on any OS; CI runs it with `-race` in the `go` job.

Modules built on `sdk` can call `Host.Subscribe(channel, fn)` in `Init` to receive Postgres NOTIFY on the server's shared LISTEN connection (the payload is unauthenticated: validate it).

Go tools (buf, protoc-gen-go, protoc-gen-connect-go, sqlc) live in `tools/go.mod`: `go tool -modfile=tools/go.mod buf ...`. The first sqlc build compiles `pg_query_go` with cgo (needs a C compiler). Generated `dbq` code is committed and never edited by hand; CI fails when `go tool -modfile=tools/go.mod sqlc diff` is not empty.

## License

The core is licensed under AGPL-3.0-only (see `LICENSE`). `sdk/`, `proto/` and `web/` are licensed under Apache-2.0.
