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
| `Access revoked` | Sessions without access close with `access_revoked` after the next snapshot, within 1 s (60 s for schedules); a revoked grant or a new lock within 2 s, an expired grant within 1 s of `expires_at` ([jit.md](jit.md#ending-sessions)). | Refused with `Access denied`; event `session.deny`. | No loss. |
| `Server unavailable while a request waits for approval` | Live. | `werbot request` fails at its next poll (every 5 s) with the connection error; the request stays `pending` on the server and expires after `request_pending_ttl` (a timeout is always a denial). | No loss: the request, the decision and the grant are in PostgreSQL. |
| `Server unavailable while a command waits for approval` | Live. | The command keeps waiting: the gateway reads the approval status every 15 s and logs failed reads. A decision made meanwhile runs or refuses the command at the next successful read; with no successful read by the deadline (`command_approval_ttl`) the user gets `Approval timed out`. A command that needs an approval while the server is unreachable is refused with `Gateway is not ready, try again later` (`RequestApproval` fails). | No loss: the request and the decision are in PostgreSQL. |
| `Target cannot run pwd -P when a command console starts` | Not affected. | The console session closes with `Console unavailable: cannot read working directory` ([policies.md](policies.md#command-console)). | No loss. |
| `Server unavailable while a console line waits for approval` | Live; the console stays open. | The line waits as an exec command does (see the row above): with no successful status read by the deadline it gets `Approval timed out` and the console shows the next prompt. | No loss: the request and the decision are in PostgreSQL. |
| `NOTIFY lost while a replica reconnects LISTEN` | Live. | A waiting command learns a decision made meanwhile at its next status read, within 15 s. After a tunnel reconnect the server also resends the decisions of the last 10 minutes of that instance. | No loss. |
| `Server replica dies that carries a web terminal or live view` (the replica of the browser connection or the one holding the gateway tunnel) | SSH sessions are not affected. The web session ends with `error`. A browser whose own replica survives closes with `1011` within about 10 s (a bridge ping after 5 s without frames + 5 s timeout). The gateway notices a dead tunnel replica only through its tunnel ping (15 s + 10 s), so the target session can outlive the connection by up to 25 s. | The user presses Reconnect and gets a new session through another replica. | No loss. |
| `Internal listener unreachable (WERBOT_INTERNAL_URL wrong, firewall, replica down)` | Not affected. | A web terminal or live view that needs a bridge through it closes with `1011`; one served by the replica that holds the tunnel works. | No loss. |
| `NOTIFY lost (terminate, stream open)` | Live. | A terminate is sent again when the gateway tunnel connects and after a replica reconnects `LISTEN`, so it takes effect, later than 2 s. A lost stream-open signal is not sent again: the browser gets `1011` after the 10 s wait for the gateway and the user retries. | No loss. |
| `Gateway without a tunnel, lock_mode = best_effort` | Live on the last snapshot; revokes and locks wait for the reconnect. A grant still ends on time: the gateway timer works from the last snapshot. | Refused with `Gateway is not ready, try again later`. | No loss. |
| `Gateway without a tunnel, lock_mode = strict` | End with `policy_stale` once the tunnel has been down for more than 60 s (checked every 5 s: 60 to 65 s after the loss); a reconnect before that resets the wait. Grant expiry works as in `best_effort`. | Refused with `Gateway is not ready, try again later`. | Session ends wait in the spool. |
| `Authorization server unavailable` (the CLI cannot reach the server) | Live. | `werbot login`, the commands that call the API (`ls`, `ssh`, `status`) and certificate renewal fail. OpenSSH through `werbot ssh-config` keeps connecting with the current certificate until it expires (at most 1 hour). | No loss. |
| `Identity provider unavailable` ([sso.md](sso.md#idp-unavailable)) | Live. The `idp.recheck` job treats an unreachable IdP as no answer, so sessions are not revoked. | An SSO login fails and returns to the login page with `Sign-in with the identity provider failed.`; password login still works. | No loss. |
| `MFA confirmation did not arrive` (the user did not confirm, or the server is slow) | Live. | The gateway reads the challenge status every 2 s and waits 60 s; then the user reads `Permission denied (MFA timeout)` and the connection closes ([mfa.md](mfa.md#ssh)). A read that fails is logged and the wait goes on. | No loss: the challenge expires on the server. |
| `mfa_confirmed lost` (NOTIFY or stream message missed) | Live. | The confirmation is only a hint; the 2-second poll of the status finds it, so the login opens at most 2 s later. | No loss. |
| `rate_limit with the database down` | Live. | A login or an MFA attempt that must be counted is refused (fail-closed), never allowed; `POST /oauth/token` answers 500. | No loss. |
| `Client clock skew` | Not affected. | A certificate is valid from 1 minute before the server time. A larger skew fails renewal with `clock skew <d> between this computer and <server>; sync the system clock`. | No loss. |
| `WebSocket input behind an L7 balancer` (idle timeout, dropped connection) | The gateway pings every connection every 30 s, so the idle timeout of the balancer (nginx 60 s, Cloudflare 100 s, AWS ALB 60 s) does not close it ([gateway.md](gateway.md#websocket-input)). A dropped connection ends its sessions; `werbot connect` reconnects with backoff and prints `connection to the gateway lost; reconnecting`. | Work through the balancer. | No loss. |
| `werbot connect --vnet killed` (`kill -9`, crash) | Connections through the virtual addresses break. | Need a new `werbot connect --vnet`. Linux: the interface `werbot0`, its route and DNS settings go with the process. macOS: remove the resolver file yourself, `sudo rm /etc/resolver/<org>.werbot` ([vnet.md](vnet.md#cleanup)). | No loss. |
| `Remote forward revoked` (`ssh -R`: access revoked, `remote_forwarding` turned off, idle timeout, maximum duration, Terminate) | The gateway stops listening on the target and closes the client's SSH connection, so `ssh -R` learns that the forward is gone. | A new `ssh -R` is checked against the current policy. | No loss. |

## End reasons

- `closed`: normal end.
- `error`: the target or the channel failed.
- `internal_error`: a panic in a connector or front.
- `gateway_shutdown`: the gateway drained on SIGTERM.
- `gateway_lost`: the instance was silent for more than 2 minutes (after all replicas were down together, the check waits 2 minutes after the start).
- `recording_unavailable`: the recording cannot be kept.
- `idle_timeout`: no traffic for the idle timeout.
- `max_duration`: the maximum duration passed.
- `terminated`: an owner, an admin or the session's own user ended it with Terminate ([web-terminal.md](web-terminal.md#terminate)); the user sees `Session closed: terminated`.
- `access_revoked`: the policy no longer allows the session: a binding, grant or lock changed, or a grant or lock expired.
- `policy_stale`: the organization uses `strict` lock mode and the gateway lost its tunnel to the server for more than 60 seconds (checked every 5 seconds).

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
