# Web UI

## Topologies

- **All-in-one (default).** `werbot server` serves the UI itself on `WERBOT_HTTP_ADDR`, next to the API.
- **Separate web.** `werbot web` runs on its own host and domain. It serves the UI and proxies the paths `/werbot.v1.` (the Connect API) and `/api/` (recording replay) to the server, so the browser sees one origin and no CORS is needed. The server's `Set-Cookie` passes unchanged and becomes a cookie of the web domain.

`werbot web` keeps no state, never connects to PostgreSQL and does not get `WERBOT_SECRET_KEY` or `WERBOT_DATABASE_URL`. Run as many replicas as you need behind an L7 load balancer.

## Configure web

| Variable | Default | Meaning |
| --- | --- | --- |
| `WERBOT_WEB_ADDR` | `:8090` | Listen address. |
| `WERBOT_WEB_API_URL` | required | Internal `http(s)://` address of the server or of its load balancer. |
| `WERBOT_TRUSTED_PROXIES` | empty | CIDRs of the load balancer in front of web, so the client IP comes from its `X-Forwarded-For`. |
| `WERBOT_LOG_LEVEL`, `WERBOT_LOG_FORMAT` | `info`, `json` | As on the server. |
| `WERBOT_METRICS_ADDR` | empty (disabled) | Listen address for `/metrics`. |

```sh
WERBOT_WEB_API_URL=http://werbot-server.internal:8080 werbot web
```

Terminate TLS on the load balancer or ingress in front of web. Web removes incoming `X-Forwarded-*` headers and sends the server exactly one `X-Forwarded-For` with the client IP it trusts. A proxied request waits up to 5 s for the connection and 30 s for the response headers; a very large recording replay can hit the 30 s limit (`503`).

`/healthz` and `/readyz` serve the load balancer. On SIGTERM `/readyz` answers 503 and web stops within 10 s. Kubernetes probes: [scaling.md](scaling.md#kubernetes).

## Configure server

- `WERBOT_WEB_EMBEDDED=false`: the server serves only the API, `/api/`, `/healthz` and `/readyz`; UI paths answer 404.
- `WERBOT_WEB_URL` = the public UI address, for example `https://ui.example.com`. Links in emails (invites, password resets) use it.
- `WERBOT_PUBLIC_URL` stays the server address: gateways build the tunnel address from it. They enroll at `--server` (`WERBOT_GATEWAY_SERVER_URL`), and the start command in the UI passes the web domain there ([gateway.md](gateway.md#firewall)).
- Put the subnet of the web replicas into `WERBOT_TRUSTED_PROXIES` of the server. Otherwise audit and IP rules see the address of web, and an "allowed CIDRs" rule denies everyone.
- Open the server HTTP port only to web and the gateways.

## Failures

- The server is down or times out: web answers `503` with `{"code":"unavailable","message":"api unavailable"}` (on `/api/`: `text/plain` `api unavailable`), and the UI offers a retry. Each such request adds to `werbot_web_upstream_errors_total` and logs `upstream error`; a request the browser cancels is not counted.
- `/readyz` of web does not depend on the server, so the load balancer keeps the UI in rotation while the API is down.
- A load balancer between web and the server that answers `502` or `504` looks the same to the UI: "unavailable".

## Versions

The UI compares its version with the server version and shows a banner when they differ. Upgrade web and the server together: [upgrading.md](upgrading.md#upgrade-a-cluster).

## Example

The `web` service (two replicas behind nginx) of `deploy/docker-compose.cluster.yaml`: [scaling.md](scaling.md#cluster-example).
