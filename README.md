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
make dev-db                      # Postgres 18 on localhost:5433 (postgres/password)
(cd web && bun install)          # web/node_modules is required by `make proto`
make proto                       # buf lint + generate (Go and TypeScript)
make build                       # builds the UI and bin/werbot
export WERBOT_SECRET_KEY="$(openssl rand -base64 32)"  # generate once and keep: it decrypts resource credentials and queued email jobs
WERBOT_ENV=development \
WERBOT_DATABASE_URL='postgres://postgres:password@localhost:5433/postgres?sslmode=disable' \
WERBOT_PUBLIC_URL=http://localhost:8080 ./bin/werbot server
```

`WERBOT_ENV=development` is for local plain-http runs: the session cookie is `werbot_session` without `Secure`, and without SMTP the log shows the full email text with reset and invite links. Production mode uses `__Host-werbot_session` with `Secure`, which browsers drop over plain http on any host but `localhost` (Safari drops it on `localhost` too).

Commands: `./bin/werbot <server|gateway|web|migrate|version>`.

- `server` runs migrations (schema and River job queue) and creates the internal gateway CA on first start, under an advisory lock. Then it serves the API (and the UI when embedded), the gateway tunnel on `WERBOT_GATEWAY_ADDR` and runs the River job client.
- `server --with-gateway` also runs a gateway instance in the same process. Each replica joins the system pool `local` of the first organization (by creation time) with a single-use system join token; before `Setup` it checks for an organization every 5s. It reaches its own server over loopback (`http://127.0.0.1:<WERBOT_HTTP_ADDR port>`, tunnel `https://127.0.0.1:<WERBOT_GATEWAY_ADDR port>`), keeps its identity in `WERBOT_DATA_DIR/gateway` (or `WERBOT_GATEWAY_DATA_DIR`) and reads the other `WERBOT_GATEWAY_*` variables below; its metrics go to the server's `/metrics`. Database and enrollment errors before the instance is enrolled are logged and retried (the wait doubles up to 5 minutes, a new single-use token each time); a busy SSH port or a failure after enrollment stops the server. On shutdown the gateway drains its sessions before the server closes the tunnel, so allow `WERBOT_GATEWAY_DRAIN_TIMEOUT` plus about 40s for the stop (compose: `stop_grace_period: 70s`).
- `gateway` runs a standalone gateway instance (`./bin/werbot gateway --help` lists the flags: `--server`, `--join-token`, `--data-dir`, `--ssh-addr`, `--proxy-protocol`, `--name`, `--idle-timeout`, `--http-addr`, `--drain-timeout`, `--max-sessions`).
- Web UI `/gateways` (owners and admins): create pools, see instances (online within 45s of the last heartbeat) and manage join tokens. "Create join token" shows the `werbot gateway --server <origin> --join-token <token>` command once; the token is not shown again. The resource form shows a pool select when more than one pool exists.
- `migrate` runs migrations (schema and River job queue) and exits.
- `web` serves the UI and proxies the API to a separate `server`.
- `version` prints the build version.

The API covers identity (auth, profile, SSH keys, members, teams) and, for owners and admins:

- `ResourceService`: resources of registered types (`ssh` today) with labels, write-only credentials (sealed with `WERBOT_SECRET_KEY`) and pinned SSH host keys.
- `AccessService`: bindings that grant a user or team one resource or every resource matching a label selector.
- `PolicyService`: per-resource network, country and schedule (UTC) rules, and `CheckAccess` to test a decision. Decisions use Cedar and fail closed.
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
| `WERBOT_DATABASE_URL` | required | Postgres URL. Pool size: `pool_max_conns`; query limit: `statement_timeout` (default `30s`), both as URL parameters. |
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
| `WERBOT_DATA_DIR` | `./data` | Local state directory (embedded gateway). |
| `WERBOT_GATEWAY_ADDR` | `:8443` | mTLS listener of the gateway tunnel (TLS 1.3, HTTP/2). Its port is the port in the tunnel URL given to gateways. |

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
| `WERBOT_GATEWAY_SERVER_URL` | empty | `http(s)://` URL of the server API; needed for the first enrollment only. |
| `WERBOT_GATEWAY_JOIN_TOKEN` | empty | Join token of the pool; needed for the first enrollment only. A saved identity wins over it. |
| `WERBOT_GATEWAY_DATA_DIR` | `./gateway-data` | Instance identity: `gateway.json`, `gateway.key`, `gateway.crt`, `ca.crt`, `ssh_host_ed25519_key`. |
| `WERBOT_GATEWAY_SSH_ADDR` | `:2222` | SSH listen address. |
| `WERBOT_GATEWAY_PROXY_PROTOCOL` | `false` | Require a PROXY protocol header on every SSH connection; expose the port to the load balancer only. |
| `WERBOT_GATEWAY_NAME` | OS hostname | Instance hostname shown in the pool. |
| `WERBOT_GATEWAY_IDLE_TIMEOUT` | `5m` | Closes an SSH connection without traffic; `0` disables. |
| `WERBOT_GATEWAY_HTTP_ADDR` | empty (disabled) | Listen address for `/readyz` and `/metrics`. |
| `WERBOT_GATEWAY_DRAIN_TIMEOUT` | `30s` | On SIGTERM, how long running sessions may continue before they end with `gateway_shutdown`. |
| `WERBOT_GATEWAY_MAX_SESSIONS` | `10000` | Concurrent session limit, 1-10000; above it users get `Gateway is busy, try again later`. |

The gateway listens only on the SSH port and, when set, `WERBOT_GATEWAY_HTTP_ADDR`; it connects out to the server tunnel. Users log in with their SSH key: `ssh <login>+<resource>@<gateway> -p 2222` runs a shell or a command on the resource, `ssh -L <port>:<host>:<port> <login>+<resource>@<gateway> -p 2222` forwards a port, and `ssh <login>@<gateway> -p 2222` (interactive terminal only) shows a menu of the resources the user may open. Without the server or a policy snapshot the gateway refuses new sessions (`Gateway is not ready, try again later`); running sessions continue. `/readyz` returns 200 only while the tunnel is up with a snapshot, a session slot is free and no drain runs. On SIGTERM `/readyz` returns 503 at once and the SSH port stops accepting connections.

The instance renews its certificate before it expires. If the process crashes during a renewal, `gateway.key` and `gateway.crt` may no longer match and the instance fails to start: clear `WERBOT_GATEWAY_DATA_DIR` and enroll again with a new join token.

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
make e2e    # Playwright e2e on DB `werbot_e2e` (needs `make dev-db`; first run: `cd web && bunx playwright install chromium`)
make e2e-network  # Docker Compose e2e: SSH through a gateway pool to a target on an internal-only network
(cd web && bun run test:unit)  # Vitest unit tests for the web UI
make sqlc   # regenerates internal/<domain>/dbq from migrations/ and internal/<domain>/queries.sql
```

`make e2e-network` needs docker, curl, jq, ssh and ssh-keygen. It builds the `werbot-e2e:local` image, publishes ports `8080`, `8081` and `2222` on localhost, and removes the `werbot-e2e` project (containers, networks, volumes) on exit.

Go tools (buf, protoc-gen-go, protoc-gen-connect-go, sqlc) live in `tools/go.mod`: `go tool -modfile=tools/go.mod buf ...`. The first sqlc build compiles `pg_query_go` with cgo (needs a C compiler). Generated `dbq` code is committed and never edited by hand; CI fails when `go tool -modfile=tools/go.mod sqlc diff` is not empty.

## License

The core is licensed under AGPL-3.0-only (see `LICENSE`). `sdk/`, `proto/` and `web/` are licensed under Apache-2.0.
