# Configuration

Werbot is configured only through environment variables. Every `werbot gateway` variable also has a flag; flags override the environment, and `werbot gateway --help` lists them. `werbot connect` flags are described in [connect.md](connect.md).

Variables that are `required` make the process refuse to start when they are empty. The tables below list every variable the code reads.

## Server (`werbot server`, `werbot migrate`)

| Variable | Default | Description |
| --- | --- | --- |
| `WERBOT_DATABASE_URL` | required | Postgres URL. URL parameters: `pool_max_conns` (pool size), `statement_timeout` (query limit, default `30s`) and `jit` (default `off`); see [Database](#database). |
| `WERBOT_ENV` | `production` | `production` or `development`. |
| `WERBOT_HTTP_ADDR` | `:8080` | API (and embedded UI) listen address. |
| `WERBOT_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `WERBOT_LOG_FORMAT` | `json` | `json` or `text`. |
| `WERBOT_METRICS_ADDR` | empty (disabled) | Listen address for `/metrics`; see [Metrics](#metrics). |
| `WERBOT_WEB_EMBEDDED` | `true` | Serve the UI from the server. |
| `WERBOT_PUBLIC_URL` | required | Public `http(s)://` address of the server, for example `https://werbot.example.com`. |
| `WERBOT_WEB_URL` | `WERBOT_PUBLIC_URL` | Public address of the UI; base for links in emails. |
| `WERBOT_TRUSTED_PROXIES` | empty | Comma-separated CIDRs whose `X-Forwarded-For` is trusted. |
| `WERBOT_SMTP_HOST` | empty | SMTP server. Empty: emails go to the log only (the full text, with links, only when `WERBOT_ENV=development`). |
| `WERBOT_SMTP_PORT` | `587` | SMTP port, 1-65535. |
| `WERBOT_SMTP_USERNAME`, `WERBOT_SMTP_PASSWORD` | empty | SMTP credentials. |
| `WERBOT_SMTP_FROM` | empty | Sender address; required when `WERBOT_SMTP_HOST` is set. |
| `WERBOT_SECRET_KEY` | required | Base64 of 32 random bytes (`openssl rand -base64 32`); encrypts stored secrets. See [Secret key](#secret-key). |
| `WERBOT_SECRET_KEY_PREVIOUS` | empty | Comma-separated base64 keys kept for decrypting data sealed before a key rotation. |
| `WERBOT_DATA_DIR` | `./data` | Local state directory: the embedded gateway, and session recordings in `recordings/` while `WERBOT_RECORDINGS_S3_BUCKET` is empty. |
| `WERBOT_RECORDINGS_S3_BUCKET` | empty | S3 bucket for session recordings. Empty: recordings go to the file system under `WERBOT_DATA_DIR/recordings`; use that only with one replica or a ReadWriteMany volume. |
| `WERBOT_RECORDINGS_S3_ENDPOINT` | empty | S3-compatible endpoint URL. Empty: standard AWS. |
| `WERBOT_RECORDINGS_S3_REGION` | `us-east-1` when the bucket is set | S3 region. |
| `WERBOT_RECORDINGS_S3_ACCESS_KEY`, `WERBOT_RECORDINGS_S3_SECRET_KEY` | empty | S3 credentials; set both or neither. |
| `WERBOT_GATEWAY_ADDR` | `:8443` | mTLS listener of the gateway tunnel (TLS 1.3, HTTP/2). Its port is the port in the tunnel URL given to gateways. |
| `WERBOT_INTERNAL_ADDR` | `:8444` | mTLS listener between server replicas (TLS 1.3, HTTP/2 only), port 1-65535. It accepts only client certificates of the internal CA with the common name of a registered replica. Keep it on the private network. |
| `WERBOT_INTERNAL_URL` | `https://<hostname>:<WERBOT_INTERNAL_ADDR port>` | `https://` address other replicas use to reach this one; its host is in the listener certificate. Set it when the OS hostname does not resolve from the other replicas. |
| `WERBOT_LOCAL_POOL_ALLOW_LOOPBACK` | `false` | See [Embedded gateway](#embedded-gateway). |
| `WERBOT_GEOIP_URL` | `https://download.db-ip.com/free/dbip-country-lite-%s.mmdb.gz` | Country database for country rules; see [GeoIP database](#geoip-database). |

### Embedded gateway

`werbot server --with-gateway` runs a gateway instance in the server process (pool, enrollment and shutdown: [gateway.md](gateway.md#embedded-gateway)). It reads the `WERBOT_GATEWAY_*` variables of the [gateway table](#gateway-werbot-gateway) below and keeps its identity in `WERBOT_DATA_DIR/gateway` (or `WERBOT_GATEWAY_DATA_DIR`). It reaches its own server over loopback (`http://127.0.0.1:<WERBOT_HTTP_ADDR port>`, tunnel `https://127.0.0.1:<WERBOT_GATEWAY_ADDR port>`), so both addresses must accept connections on 127.0.0.1: leave the host empty (`:8080`) or use `0.0.0.0` or `127.0.0.1`, not a single non-loopback IP.

Because it runs inside the server process, it refuses by default target addresses on the server's own host: loopback (`127.0.0.0/8`, `::1`), link-local including cloud metadata (`169.254.0.0/16`, `fe80::/10`), multicast (including interface-local `ff01::/16`), unspecified (`0.0.0.0`, `::`), and every address the host of `WERBOT_DATABASE_URL` resolves to. The check runs on the resolved address when a session opens (the user gets `Access denied`, audited as `target_not_allowed`) and again on every connect. It covers `telnet`, `tcp` and `udp` sessions and health probes, not `ssh` resources: they keep their own dial, so a loopback `ssh` resource connects while its health shows `unhealthy` with the refusal. Other addresses of the host and its LAN stay reachable: restrict them with resource endpoints and access bindings.

`WERBOT_LOCAL_POOL_ALLOW_LOOPBACK=true` turns the check off, which a same-host database or target needs. Standalone gateways are not affected. The embedded gateway enrolls again by itself when its certificate expires.

## Gateway (`werbot gateway`)

An instance of a gateway pool. `WERBOT_LOG_LEVEL` and `WERBOT_LOG_FORMAT` apply to the gateway too.

| Variable | Flag | Default | Description |
| --- | --- | --- | --- |
| `WERBOT_GATEWAY_SERVER_URL` | `--server` | empty | `https://` URL of the server API; needed for enrollment only (a start with a valid saved identity does not check it). `http://` is accepted only for a loopback host (`127.0.0.0/8`, `::1`, `localhost`) or with the flag `--insecure-enroll`: enrollment sends the join token and receives the pool's private SSH host key, so use it only on a trusted network. |
| `WERBOT_GATEWAY_JOIN_TOKEN` | `--join-token` | empty | Join token of the pool; needed for enrollment: the first start or a start with a stale identity. A valid saved identity wins over it. Prefer the variable: it keeps the token out of `ps` output. |
| `WERBOT_GATEWAY_DATA_DIR` | `--data-dir` | `./gateway-data` | Instance identity: `gateway.json`, `gateway.key`, `gateway.crt`, `ca.crt`, `ssh_host_ed25519_key`, and the recording spool `spool/`. Keep it on a persistent volume (a PVC in Kubernetes): undelivered recordings and session ends wait there across restarts. |
| `WERBOT_GATEWAY_SSH_ADDR` | `--ssh-addr` | `:2222` | SSH listen address. |
| `WERBOT_GATEWAY_PROXY_PROTOCOL` | `--proxy-protocol` | `false` | Require a PROXY protocol header on every SSH connection; expose the port to the load balancer only. |
| `WERBOT_GATEWAY_NAME` | `--name` | OS hostname | Instance hostname shown in the pool. |
| `WERBOT_GATEWAY_IDLE_TIMEOUT` | `--idle-timeout` | `5m` | Ends a session without traffic in either direction (`idle_timeout`) unless its resource sets `idle_timeout`, and closes an SSH connection that runs no session for this long (keepalives do not count; typing in the resource menu does). A running session keeps its connection open; `0` disables both. |
| `WERBOT_GATEWAY_HTTP_ADDR` | `--http-addr` | empty (disabled) | Listen address for `/readyz` and `/metrics`. |
| `WERBOT_GATEWAY_DRAIN_TIMEOUT` | `--drain-timeout` | `30s` | On SIGTERM, how long running sessions may continue before they end with `gateway_shutdown`; `0` ends them at once. |
| `WERBOT_GATEWAY_SPOOL_MAX` | `--spool-max` | `1GiB` | Size cap of the recording spool: bytes or a `KiB`, `MiB`, `GiB` suffix (powers of 1024); must be above 0. |
| `WERBOT_GATEWAY_MAX_SESSIONS` | `--max-sessions` | `10000` | Concurrent session limit, 1-10000; above it users get `Gateway is busy, try again later`. |
| `WERBOT_GATEWAY_MAX_DURATION` | `--max-duration` | `12h` | Ends a session after this long (`max_duration`) unless its resource sets `max_duration`; `0` means no limit, a negative value fails the start. |

The gateway listens only on the SSH port and, when set, `WERBOT_GATEWAY_HTTP_ADDR`; it connects out to the server tunnel. The flag `--insecure-enroll` has no variable.

### Readiness, drain and limits

- `/readyz` (on `WERBOT_GATEWAY_HTTP_ADDR`) returns 200 only while the tunnel is up with a policy snapshot (a planned reconnect for `rebalance` or a server `shutdown` keeps it for up to 5s), a session slot is free and no drain runs.
- On SIGTERM `/readyz` returns 503 at once and the SSH port stops accepting connections. Sessions continue for `WERBOT_GATEWAY_DRAIN_TIMEOUT`; the rest end with `gateway_shutdown`.
- At `WERBOT_GATEWAY_MAX_SESSIONS` new sessions are refused and `/readyz` returns 503.
- Without the server or a policy snapshot the gateway refuses new sessions (`Gateway is not ready, try again later`) and closes that SSH connection after 30 s; running sessions continue. A connection must authenticate within 30 s.
- `WERBOT_GATEWAY_SPOOL_MAX` caps the recording spool. While the unsent spool is at the cap, new sessions of recorded kinds get `Gateway is busy, try again later` (every SSH terminal or exec session is recorded, so in practice all new SSH sessions); unrecorded ones (`tcp`, `udp`, `ssh -L`) still open and running sessions continue. The `WERBOT_GATEWAY_DATA_DIR` volume must hold the cap plus headroom. While the spool cannot write (disk full or not writable), new recorded sessions get the same answer and the same metric, and each running session ends with `recording_unavailable` at its next chunk flush (within about 10 s); a command that ends sooner can stay unrecorded.
- The instance renews its 24h certificate after 16h; until the renewed certificate is first used, the previous one stays valid. A saved identity whose certificate has expired or no longer loads counts as not enrolled, and the next enrollment overwrites it. An instance whose certificate expires while running stops (`gateway certificate expired`). A standalone gateway offline for more than 24h needs a new join token in `WERBOT_GATEWAY_JOIN_TOKEN`; a spent token fails at start with `invalid join token`.

## Web (`werbot web`)

No database access. Its four shared variables follow the same rules as on the server, with their own values for this process.

| Variable | Default | Description |
| --- | --- | --- |
| `WERBOT_WEB_API_URL` | required | `http(s)://` URL of the API server. |
| `WERBOT_WEB_ADDR` | `:8090` | Listen address. |
| `WERBOT_TRUSTED_PROXIES` | empty | Comma-separated CIDRs whose `X-Forwarded-For` is trusted. |
| `WERBOT_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `WERBOT_LOG_FORMAT` | `json` | `json` or `text`. |
| `WERBOT_METRICS_ADDR` | empty (disabled) | Listen address for `/metrics`. |

`werbot web` does not read `WERBOT_DATABASE_URL` or `WERBOT_SECRET_KEY`. Details: [web.md](web.md).

## Client (`werbot connect`, `werbot login`)

| Variable | Flag | Default | Description |
| --- | --- | --- | --- |
| `WERBOT_CONNECT_GATEWAY` | `--gateway` | required | Gateway SSH address `host[:port]`; the port defaults to `2222`. |
| `WERBOT_CONNECT_LOGIN` | `--login` | required | Werbot login. |
| `WERBOT_SERVER` | — | server of the last `werbot login` (`<home>/current`) | Server whose profile the client commands use, `http(s)://host[:port]`. |
| `WERBOT_HOME` | — | `~/.werbot` (Windows: `%USERPROFILE%\.werbot`) | Client profiles: one `0700` directory per server with `profile.json`, the key and the certificate (`0600`). |

Other flags: `-i <key>` and `--known-hosts <path>`. Details: [connect.md](connect.md) and [cli.md](cli.md).

## Docker Compose

| Variable | Description |
| --- | --- |
| `WERBOT_VERSION` | Image tag, for example `v0.1.1`. Required. |
| `POSTGRES_PASSWORD` | Postgres password, URL-safe (`openssl rand -hex 24`); compose builds `WERBOT_DATABASE_URL` from it. Required. |

The compose files pass the whole `.env` to the server (`env_file`), so any variable from the tables above that you add to `.env` reaches `werbot server`. `docker-compose.gateway.yaml` passes the whole `.env` to the gateway the same way. Gateway and web services of the cluster example get no `.env`: they never see `WERBOT_SECRET_KEY`, `POSTGRES_PASSWORD` or the S3 keys.

## Secret key

Generate once and keep a copy: it decrypts stored resource credentials and queued email jobs. It also encrypts the gateway tunnel CA and the pool SSH host keys, so losing it breaks gateways too.

```sh
openssl rand -base64 32
```

To rotate: move the current value to `WERBOT_SECRET_KEY_PREVIOUS` (comma-separated when there are several), set a new `WERBOT_SECRET_KEY` and restart. Only the server needs the key; never copy it to gateway hosts.

## Database

`WERBOT_DATABASE_URL` accepts these URL parameters:

- `pool_max_conns`: pool size of one replica.
- `statement_timeout`: query limit, default `30s`.
- `jit`: default `off`.

The connection formula for several replicas and PgBouncer: [scaling.md](scaling.md#postgresql).

## Behind a reverse proxy

Set `WERBOT_TRUSTED_PROXIES` to the CIDRs of the proxies, so the client IP in sessions and audit comes from `X-Forwarded-For`. Set `WERBOT_PUBLIC_URL` with `https://`. The host of `WERBOT_PUBLIC_URL` and the port of `WERBOT_GATEWAY_ADDR` form the tunnel address `https://<host>:<port>` that gateways receive at enrollment. The tunnel uses mTLS that ends on the replica: put an L4 (TCP passthrough) load balancer in front of it with the same external port; a TLS-terminating proxy breaks the tunnel. The API port works behind any HTTP load balancer.

## Separate web UI

Run `werbot server` with `WERBOT_WEB_EMBEDDED=false`: it serves only the API, and `werbot web` serves the UI and proxies the API (`WERBOT_WEB_API_URL`). Set `WERBOT_WEB_URL` on the server to the UI address (links in emails; default `WERBOT_PUBLIC_URL`) and `WERBOT_TRUSTED_PROXIES` of the server to the subnet of `werbot web` (client IPs in sessions and audit). See [web.md](web.md).

## Email

An empty `WERBOT_SMTP_HOST` writes emails to the log. With `WERBOT_ENV=development` the log has the full text with links; in production it does not.

On any port but 465 the server requires STARTTLS. With `WERBOT_ENV=development` it uses STARTTLS only when the SMTP server offers it, so a local server such as Mailpit (`make dev-db`, SMTP `localhost:27025`, inbox http://localhost:27825) works without TLS.

`WERBOT_ENV=development` is for local plain-http runs: the session cookie is `werbot_session` without `Secure`. The production cookie and remote access over plain http: [install.md](install.md#remote-access).

## Session recordings

By default recordings go to the file system under `WERBOT_DATA_DIR/recordings`. That works only with one server replica or a shared ReadWriteMany volume. With several replicas use an S3-compatible store (AWS S3, Cloudflare R2, SeaweedFS) through `WERBOT_RECORDINGS_S3_BUCKET` and the related variables. See [scaling.md](scaling.md#session-recordings).

## GeoIP database

`geoip.refresh` downloads `WERBOT_GEOIP_URL` on start and then once a month (a daily check at 04:00 UTC). The URL must be `https://`; `http://` is accepted only for a loopback host. It may contain one `%s`, replaced with the month as `2006-01`: when this month's file returns 404, last month's is used. A URL without `%s` is fetched once a month with no fallback. The body may be a raw `.mmdb`, gzip, or a tar archive (optionally gzipped) whose first `.mmdb` file is used, as MaxMind permalinks serve. The country comes from `country.iso_code` (DB-IP, MaxMind GeoLite2) or a top-level `country_code` (IPinfo Lite).

The URL may carry a token in the query or as `user:password@`; Werbot never logs it or puts it in errors.

The UI shows "IP Geolocation by DB-IP" only while the URL is DB-IP Lite (a `/free/` path on `db-ip.com`), as its license requires. The credit appears only where the database is used: the country rules and the access check of a resource. A mirror of DB-IP Lite on another host still requires that attribution: add it yourself.

## Metrics

An empty `WERBOT_METRICS_ADDR` (and `WERBOT_GATEWAY_HTTP_ADDR` for a gateway) leaves metrics off. Otherwise a separate listener serves `/metrics`; never expose it on a public port. The metric list is in [scaling.md](scaling.md#metrics).

## Deployment modes

- All-in-one (default): `werbot server` serves the API and the UI on one address.
- Several server replicas: an L4 load balancer in front of the tunnel port; see [Behind a reverse proxy](#behind-a-reverse-proxy) and [scaling.md](scaling.md).
- Separate UI: see [Separate web UI](#separate-web-ui).

Compose setup, files and the cluster example: [install.md](install.md) and [scaling.md](scaling.md#cluster-example). `docker compose config` refuses an empty `WERBOT_VERSION`, `WERBOT_SECRET_KEY` or `POSTGRES_PASSWORD` and names the generate command. Keep `.env`: its `WERBOT_SECRET_KEY` decrypts stored credentials.
