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
WERBOT_DATABASE_URL='postgres://postgres:password@localhost:5433/postgres?sslmode=disable' ./bin/werbot server
```

Commands: `./bin/werbot <server|web|migrate|version>`.

- `server` runs migrations under an advisory lock, then serves the API (and the UI when embedded).
- `migrate` runs migrations and exits.
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
- Separate UI: run `werbot server` with `WERBOT_WEB_EMBEDDED=false` and `werbot web` with `WERBOT_WEB_API_URL` pointing to the server.

Docker:

```sh
docker compose -f deploy/docker-compose.yaml up --build
```

The stack listens on http://localhost:8080 (`/healthz`, `/readyz`).

## Development

```sh
make test   # needs `make dev-db`
make lint
```

Go tools (buf, protoc-gen-go, protoc-gen-connect-go) live in `tools/go.mod`: `go tool -modfile=tools/go.mod buf ...`.

## License

The core is licensed under AGPL-3.0-only (see `LICENSE`). `sdk/`, `proto/` and `web/` are licensed under Apache-2.0.
