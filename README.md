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
export WERBOT_SECRET_KEY="$(openssl rand -base64 32)"  # generate once and keep: it decrypts queued email jobs
WERBOT_ENV=development \
WERBOT_DATABASE_URL='postgres://postgres:password@localhost:5433/postgres?sslmode=disable' \
WERBOT_PUBLIC_URL=http://localhost:8080 ./bin/werbot server
```

`WERBOT_ENV=development` is for local plain-http runs: the session cookie is `werbot_session` without `Secure`, and without SMTP the log shows the full email text with reset and invite links. Production mode uses `__Host-werbot_session` with `Secure`, which browsers drop over plain http on any host but `localhost` (Safari drops it on `localhost` too).

Commands: `./bin/werbot <server|web|migrate|version>`.

- `server` runs migrations (schema and River job queue) under an advisory lock, then serves the API (and the UI when embedded) and runs the River job client.
- `migrate` runs migrations (schema and River job queue) and exits.
- `web` serves the UI and proxies the API to a separate `server`.
- `version` prints the build version.

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

`werbot web` (no database access):

| Variable | Default | Description |
| --- | --- | --- |
| `WERBOT_WEB_API_URL` | required | `http(s)://` URL of the API server. |
| `WERBOT_WEB_ADDR` | `:8090` | Listen address. |
| `WERBOT_TRUSTED_PROXIES` | empty | Comma-separated CIDRs whose `X-Forwarded-For` is trusted. |
| `WERBOT_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `WERBOT_LOG_FORMAT` | `json` | `json` or `text`. |
| `WERBOT_METRICS_ADDR` | empty (disabled) | Listen address for `/metrics`. |

## Deployment modes

- All-in-one (default): `werbot server` serves the API and the UI on one address.
- Separate UI: run `werbot server` with `WERBOT_WEB_EMBEDDED=false` and `werbot web` with `WERBOT_WEB_API_URL` pointing to the server. Set `WERBOT_WEB_URL` on the server to the UI address (links in emails) and `WERBOT_TRUSTED_PROXIES` to the `werbot web` address (client IPs in sessions and audit).

Docker:

```sh
export WERBOT_SECRET_KEY="$(openssl rand -base64 32)"  # required; keep the same key between runs
docker compose -f deploy/docker-compose.yaml up --build
```

The stack listens on http://localhost:8080 (`/healthz`, `/readyz`).

## Development

```sh
make test   # needs `make dev-db`
make lint
make e2e    # Playwright e2e on DB `werbot_e2e` (needs `make dev-db`; first run: `cd web && bunx playwright install chromium`)
make sqlc   # regenerates internal/<domain>/dbq from migrations/ and internal/<domain>/queries.sql
```

Go tools (buf, protoc-gen-go, protoc-gen-connect-go, sqlc) live in `tools/go.mod`: `go tool -modfile=tools/go.mod buf ...`. The first sqlc build compiles `pg_query_go` with cgo (needs a C compiler). Generated `dbq` code is committed and never edited by hand; CI fails when `go tool -modfile=tools/go.mod sqlc diff` is not empty.

## License

The core is licensed under AGPL-3.0-only (see `LICENSE`). `sdk/`, `proto/` and `web/` are licensed under Apache-2.0.
