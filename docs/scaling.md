# Scaling and high availability

Werbot scales horizontally and runs highly available in the Community edition. This guide covers topology, load balancing, PostgreSQL, recordings, gateway pools, Kubernetes, resources, metrics and a runnable cluster example.

## Topology

- N replicas of `werbot server` behind a load balancer.
- One PostgreSQL primary.
- S3-compatible storage for session recordings.
- Gateway pools: several `werbot gateway` instances per pool.
- The UI is embedded in `werbot server` (default), or it runs on its own host and domain as replicas of `werbot web` (`WERBOT_WEB_EMBEDDED=false` on the server, `WERBOT_WEB_API_URL` on `werbot web`; see [web.md](web.md)). Set `WERBOT_WEB_URL` on the server to the UI address (links in emails) and `WERBOT_TRUSTED_PROXIES` to the `werbot web` address (client IPs in sessions and audit).

All state lives in PostgreSQL:

- Signals between replicas use `LISTEN/NOTIFY`.
- One-time start-up work runs under an advisory lock.
- The River leader schedules periodic jobs and any replica runs them.

Redis and NATS are not needed. The default all-in-one mode (`werbot server`, one address for API and UI) is the single-replica case of the same design. There is no Helm chart in `v0.x`. A one-host example is in [Cluster example](#cluster-example).

## Load balancing

| Port | Balancer | Notes |
| --- | --- | --- |
| `WERBOT_HTTP_ADDR` (UI and API) | L7, any replica | Sticky sessions are not needed: sessions live in PostgreSQL. Health check `GET /readyz`. Put the balancer addresses in `WERBOT_TRUSTED_PROXIES`. |
| `WERBOT_WEB_ADDR` (UI and API through `werbot web`, when the UI is separate) | L7, any `werbot web` replica | Terminate TLS on the balancer. Health check `GET /readyz` of web (it does not depend on the server). Put the balancer address in `WERBOT_TRUSTED_PROXIES` of web and the web subnet in `WERBOT_TRUSTED_PROXIES` of the server. |
| `WERBOT_GATEWAY_ADDR` (gateway tunnel) | L4 TCP passthrough only | No TLS termination (mTLS ends on the replica) and no PROXY protocol. The external port must equal the port of `WERBOT_GATEWAY_ADDR`, because a gateway dials `https://<host of WERBOT_PUBLIC_URL>:<port of WERBOT_GATEWAY_ADDR>`. Long streams rebalance by themselves: a replica closes a stream after 30 minutes ±20% and the gateway reconnects at once without dropping SSH sessions. |
| `WERBOT_GATEWAY_SSH_ADDR` (SSH to a pool) | L4 with PROXY protocol v2 | HAProxy `send-proxy-v2`, nginx `proxy_protocol on` or AWS NLB proxy protocol v2, plus `WERBOT_GATEWAY_PROXY_PROTOCOL=true` on the instances. Expose the port to the balancer only. Health check `GET /readyz` on `WERBOT_GATEWAY_HTTP_ADDR`. Without a balancer use DNS round-robin: all instances of a pool serve one host key. |

The L4 idle timeout of the balancer must be longer than `WERBOT_GATEWAY_IDLE_TIMEOUT` (default `5m`). The gateway ends idle sessions by itself; a shorter balancer timeout cuts them first and without the `idle_timeout` reason. It must also exceed the `idle_timeout` of a resource, which can be set up to 24 h. The example `nginx.cluster.conf` uses `proxy_timeout 1h`, which covers only resources with `idle_timeout` of 1 h or less; for longer ones raise `proxy_timeout` in the `stream` block (for example `25h`) and reload nginx.

The balancer also needs enough file descriptors for the sessions it relays (the example sets `worker_rlimit_nofile 65536` and `ulimits: { nofile: 65536 }`). Werbot v0.1.0 has no per-IP limit for unauthenticated SSH connections: limit connections per source IP on the balancer or firewall ([security.md](security.md)).

## PostgreSQL

Connect directly, or through PgBouncer in **session mode** only: transaction and statement mode break `LISTEN` and River.

- Pool size per replica: `pool_max_conns` in `WERBOT_DATABASE_URL`.
- Connection budget: `replicas × (pool_max_conns + 2) ≤ max_connections` (the 2 are the `LISTEN` connection and the start-up work). Example: 3 × (20 + 2) = 66 ≤ 100.
- `statement_timeout` defaults to `30s`; override it in the URL. An exhausted pool answers `Unavailable` instead of hanging. `jit` defaults to `off`.
- High availability is the operator's job (Patroni, CloudNativePG). A failover needs no Werbot restart.
- Read replicas are not used.

## Session recordings

The file system (`WERBOT_DATA_DIR/recordings`, used while `WERBOT_RECORDINGS_S3_BUCKET` is empty) is for one server replica or a ReadWriteMany volume. With more than one replica use S3-compatible storage (AWS S3, R2, SeaweedFS) through `WERBOT_RECORDINGS_S3_*`. Otherwise chunks of one session land on different nodes and `Verify` reports `chunk_missing`.

An unreachable S3 does not drop sessions: chunks wait in the gateway spool and upload later.

## Gateway pools

A pool is a name in the UI and the `gateway_id` of a resource; its processes are instances.

- Instances join with the reusable token of the pool (limits and handling: [security.md](security.md#gateway-join-tokens)). For autoscaling set `max_uses` to the number of instances and a short lifetime, keep the token in the orchestrator secret store and revoke it after the rollout.
- Remove a retired instance with Delete instance (its tunnel closes). Instances whose certificate expired more than 24 h ago and tokens that expired more than 7 days ago are deleted automatically.
- The pool has one SSH host key for all instances.
- Pool connectors are the intersection of the connectors of its online instances; that is how specialised pools work ([connectors.md](connectors.md)).
- The embedded gateway (`server --with-gateway`) needs no join token from you: [gateway.md](gateway.md#embedded-gateway).
- Steps in the UI: [gateway.md](gateway.md#enroll-a-gateway). Threats: [security.md](security.md#gateway-join-tokens).

## Kubernetes

- **web:** a `Deployment` without PVC and secrets. `readinessProbe` `httpGet /readyz` and `livenessProbe` `httpGet /healthz` on the port of `WERBOT_WEB_ADDR`.
- **server:** a `Deployment`. `readinessProbe` `httpGet /readyz` and `livenessProbe` `httpGet /healthz` on the port of `WERBOT_HTTP_ADDR` (liveness does not ping the database, so a PostgreSQL failover does not restart pods). Use `lifecycle.preStop.sleep.seconds: 10` (Kubernetes 1.30 or newer; the distroless image has no shell, so `exec: sleep` does not work) so the endpoint leaves the balancer first, and `terminationGracePeriodSeconds: 40` (preStop 10 s + HTTP shutdown 10 s + River 10 s + margin).
- **gateway:** a `StatefulSet` with `volumeClaimTemplates` on `WERBOT_GATEWAY_DATA_DIR` (instance identity, pool host key, recording spool). Without a PVC a restart loses unsent chunks: that recording fails `Verify` and the UI shows it. A recreated pod with the same PVC does not spend a token. Set `WERBOT_GATEWAY_HTTP_ADDR` and use `readinessProbe` `httpGet /readyz` on it, with the same `preStop`. `terminationGracePeriodSeconds` must be at least 10 + `WERBOT_GATEWAY_DRAIN_TIMEOUT` + 10 (60 with the default `30s`).
- **Services:** SSH is a `LoadBalancer` with L4 and PROXY protocol; the server tunnel is L4 without it.

## Resources

- `GOMEMLIMIT` = 90% of the container memory limit (a 1 GiB limit gives `GOMEMLIMIT=920MiB`).
- Open files: `ulimit -n` at least 4 × `WERBOT_GATEWAY_MAX_SESSIONS` (default 10000, so 40000) plus 32 × the expected UDP sessions (up to 32 sockets per UDP session). Go raises the soft limit to the hard limit, so the hard limit matters: compose `ulimits: { nofile: 65536 }`, systemd `LimitNOFILE=65536`.
- Budgets checked by the load test: gateway instance RSS ≤ 200 MB + 256 KB × active sessions; server replica RSS ≤ 300 MB + 64 KB × tunnel streams; PostgreSQL connections per replica ≤ `pool_max_conns` + 2.
- The stand, method and results are in [test/load/README.md](../test/load/README.md) (Linux only, manual).

## Metrics

Prometheus text format, no histograms. The listeners are not published outside:

- server: `WERBOT_METRICS_ADDR` serves `/metrics` (empty: disabled);
- gateway: `WERBOT_GATEWAY_HTTP_ADDR` serves `/readyz` and `/metrics`; the embedded gateway (`--with-gateway`) writes into the server registry;
- web: its own `WERBOT_METRICS_ADDR`.

| Metric | Process | Meaning |
| --- | --- | --- |
| `werbot_tunnel_streams` | server | Open gateway tunnel streams on this replica (gauge). |
| `werbot_snapshot_pushes_total` | server | Policy snapshots sent to gateways. |
| `werbot_db_pool_acquired_conns` | server | Database pool connections in use (gauge). |
| `werbot_db_pool_total_conns` | server | Database pool connections open (gauge). |
| `werbot_gateway_sessions_active` | gateway | Sessions holding a `WERBOT_GATEWAY_MAX_SESSIONS` slot (gauge). |
| `werbot_gateway_sessions_rejected_total{reason}` | gateway | Refused new sessions; `reason` is `busy`, `spool_full` or `not_ready`. |
| `werbot_gateway_panics_total` | gateway | Connector panics recovered by the pipeline. |
| `werbot_gateway_spool_bytes` | gateway | Unsent recording bytes in the spool (gauge). |
| `werbot_gateway_tunnel_connected` | gateway | 1 while the tunnel stream is open and its latest snapshot compiled, else 0. |
| `werbot_gateway_session_bytes_total{direction}` | gateway | Session payload bytes; `direction` is `up` (client to target) or `down`. |
| `werbot_gateway_datagrams_total{direction}` | gateway | Session datagrams; `direction` is `up` or `down`. |
| `werbot_gateway_datagrams_dropped_total{reason}` | gateway | Datagrams dropped by the `udp` connector; `reason` is `flows` (more than 32 flows) or `size` (over 8192 bytes). |
| `werbot_gateway_health_checks_total{result}` | gateway | Resource health checks; `result` is `ok` or `fail`. |
| `werbot_gateway_denials_dropped_total` | gateway | Refused connections not reported: more than 100 per heartbeat. |
| `werbot_web_upstream_errors_total` | web | Proxied requests that failed to reach the API server (answered `503`). |

## Upgrades

In `v0.x` upgrade only with all replicas and instances stopped. Order: [upgrading.md](upgrading.md#upgrade-a-cluster).

## Cluster example

`deploy/docker-compose.cluster.yaml` with `deploy/nginx.cluster.conf` (project `werbot-cluster`) starts:

- 2 `server` replicas without a published HTTP port;
- 2 `web` replicas;
- `lb` (nginx) with ports 8080 (to `web`), 8443 (tunnel) and 2222 (SSH) following [Load balancing](#load-balancing);
- `gateway-1` and `gateway-2` in the profile `gateways`;
- PostgreSQL and SeaweedFS with the bucket `werbot-recordings`.

It needs Docker Compose 2.23.1 or newer. The example and the `test/cluster` test stand share the project name and host ports: run one at a time.

1. Download `docker-compose.cluster.yaml`, `nginx.cluster.conf` and `.env.example` of your tag from `https://raw.githubusercontent.com/werbot/werbot/<tag>/deploy/` into one directory; copy `.env.example` to `.env`.
2. Fill `WERBOT_VERSION` (your tag), `WERBOT_SECRET_KEY` and `POSTGRES_PASSWORD` in `.env` as in [install.md](install.md#docker-compose).
3. Set `WERBOT_PUBLIC_URL` in `.env` to a DNS name or IP of this host that containers can reach, for example `http://<host-ip>:8080`. `localhost` does not work: gateways dial the tunnel at that host on port 8443. Find the IP with `hostname -I` (Linux) or `ipconfig getifaddr en0 || ipconfig getifaddr en1` (macOS).
4. Append the S3 keys: `echo "WERBOT_RECORDINGS_S3_ACCESS_KEY=$(openssl rand -hex 12)" >> .env` and `echo "WERBOT_RECORDINGS_S3_SECRET_KEY=$(openssl rand -hex 24)" >> .env`.
5. Start: `docker compose -f docker-compose.cluster.yaml up -d --wait`.
6. Open the UI (on this host `http://localhost:8080`; from another machine see [install.md](install.md#remote-access)) and complete Setup.
7. Gateways, Create pool, Create join token with `max uses` of 2 or more. Append `WERBOT_GATEWAY_JOIN_TOKEN=<token>` to `.env`.
8. Start the gateways: `docker compose -f docker-compose.cluster.yaml --profile gateways up -d --wait`. Both instances show online.
9. Connect: `ssh -p 2222 <login>@<host>`.

Limits of the example:

- One host shows the topology but gives no high availability.
- SeaweedFS on one disk is not for production.
- The default network uses `172.28.0.0/24`; change it in the file if it is taken. Dynamic container addresses come from `172.28.0.128/25`. The server trusts `X-Forwarded-For` only from that range (the `web` replicas); `web` trusts only `lb` (`172.28.0.10`). Gateways sit in the separate `edge` network and reach only `lb`, so they cannot spoof client IPs to the server and never see `WERBOT_SECRET_KEY`, the database password or the S3 keys. In production keep only `lb` and the `web` replicas in the server network.
- Enrollment uses `--insecure-enroll` on the private `edge` network of one host; across hosts use an `https://` server URL.
- Scale with `docker compose -f docker-compose.cluster.yaml up -d --scale server=3` after checking the formula in [PostgreSQL](#postgresql), or `--scale web=3`.
