# Failure modes

What happens to running and new sessions and to data when a part of Werbot fails.

## Failure matrix

| Failure | Active sessions | New sessions | Data |
| --- | --- | --- | --- |
| `One server replica` | Live. | Served by the other replicas; the gateway reconnects within 5 s. | No loss. |
| `All server replicas or PostgreSQL` | Live. | Refused with `Gateway is not ready, try again later` (fail-closed: credentials are not cached on the gateway). | Recording chunks, `FinishRecording` and `EndSession` wait in the gateway spool. |
| `Gateway instance` | Break (TCP); the client reconnects through the balancer. | Served by the other instances of the pool. | The server closes the sessions with `gateway_lost` within 2.5 minutes; the recording tail uploads after a restart when the disk is persistent. |
| `S3-compatible storage` | Live. | Work. | `IngestChunk` answers `Unavailable`, chunks go to the spool and upload later. |
| `SMTP, firehol and GeoIP feeds` | Not affected. | Not affected. | River retries; the old data stays. |
| `Panic in a connector or front` | Only that session ends. | Work. | `end_reason` is `internal_error`, the stack goes to the log, `werbot_gateway_panics_total` grows. |
| `Overloaded instance` | Live. | Refused with `Gateway is busy, try again later` at `WERBOT_GATEWAY_MAX_SESSIONS`; `/readyz` answers 503. | No loss. |
| `Total spool ≥ WERBOT_GATEWAY_SPOOL_MAX` | Live; a session with more than 64 MiB unsent ends with `recording_unavailable`. | Recorded sessions (`ssh` terminal and exec, `telnet`) are refused with `Gateway is busy, try again later`; `ssh -L`, `tcp` and `udp` are not recorded and keep working. | No loss. |
| `Target unreachable` | The channel closes, `end_reason` is `error`. | Resource health becomes `unhealthy`. | No loss. |
| `Access revoked` | Sessions without access close with `access_revoked` after the next snapshot, within 1 s (60 s for schedules). | Refused with `Access denied`; event `session.deny`. | No loss. |

## End reasons

- `closed`: normal end.
- `error`: the target or the channel failed.
- `internal_error`: a panic in a connector or front.
- `gateway_shutdown`: the gateway drained on SIGTERM.
- `gateway_lost`: the instance was silent for more than 2 minutes (after all replicas were down together, the check waits 2 minutes after the start).
- `recording_unavailable`: the recording cannot be kept.
- `idle_timeout`: no traffic for the idle timeout.
- `max_duration`: the maximum duration passed.
- `access_revoked`: the policy no longer allows the session.

## How Werbot protects data

- Only idempotent operations are retried, through the spool: a chunk by `seq`, `FinishRecording`, `EndSession`.
- The session id stays in the heartbeat until `EndSession` is delivered, so the reason is not overwritten by `gateway_lost`.
- HTTP/2 ping detects a half-open tunnel; a tunnel call has a 10 s deadline; queries have `statement_timeout`.
- Refused connections are reported with the next heartbeat; they are kept in memory and lost if the instance is killed.

## Durability recommendations

- Keep the gateway spool on a persistent disk: a PVC in Kubernetes, a named volume in compose.
- Run PostgreSQL with replication and `synchronous_commit = on`.
- Enable versioning on the S3 bucket.
- WORM storage for recordings is an Enterprise feature.
- Back up as described in [upgrading.md](upgrading.md#backup) and keep a copy of `WERBOT_SECRET_KEY` apart from the database backup.
