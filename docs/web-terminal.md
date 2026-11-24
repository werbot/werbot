# Web terminal, live view and terminate

Three features of the web UI work on terminal sessions:

- **Web terminal**: open a shell on a resource in the browser, with no SSH client.
- **Live view**: watch a running terminal session, read-only.
- **Terminate**: end a running session at once.

All three go through the gateway, like SSH. The browser never reaches a target and never sees a credential.

## Web terminal

Press **Open terminal** on a resource (Resources page or Dashboard). The page opens an xterm terminal on `/api/terminal` and connects it to a shell on the resource.

- **Resources.** Resources with an interactive shell: SSH and telnet.
- **Access.** The gateway decides exactly as it does for an SSH connection: bindings, network, country and schedule rules, locks and [terminal rules](policies.md#command-console). A terminal rule in `console` mode opens the shell as a checked command console; `deny` refuses it. An unknown resource and a resource you may not use look the same: close code `4003`.
- **Recording.** The session is recorded and verified like an SSH terminal. The session list and the audit event `session.start` show `front: web`.
- **Client IP.** Rules and the audit log use the IP address of the browser, not the address of a proxy.
- **Window.** The page sends its size (`cols` 1 to 500, `rows` 1 to 200) when it opens and again when the window changes.
- **Who.** People only. An agent token and an agent profile never get a web terminal ([agents.md](agents.md)).
- **Browsers.** A current Chrome, Edge, Firefox or Safari. The UI sends `Content-Security-Policy: default-src 'self'` without a `connect-src` rule. Same-origin `ws:` and `wss:` connections are allowed by browsers that implement CSP level 3 (current releases of the browsers above); an older Safari may refuse them. Playwright tests cover Chromium only.
- **No clipboard mode.** The terminal has no file transfer and no special clipboard channel. Copy and paste work as in any browser page.

## Live view

Press **Watch** on a running terminal session in the Sessions list. The page opens `/api/sessions/<id>/live`.

- **Who.** Owners and admins watch any terminal session of the organization. A member watches only their own.
- **Read-only.** The observer sees the window size and the output. Everything the observer types is dropped: it never reaches the shell of the observed session.
- **No history.** The view starts empty. A full-screen program (`vim`, `top`) looks broken until it redraws. Press `Ctrl+L` in the observed terminal, or open the [recording](scaling.md#session-recordings) replay for the past.
- **Silent.** The observed user gets no notice that someone watches. Each watch is audited as `session.observe` with the id of the session and the observer.
- **Limit.** Up to 16 observers per session; the 17th gets close code `4008`. A slow observer loses output; the session itself never waits for it.
- **Not live.** A session that has ended, is not a terminal (`exec`, `stream`, `datagram`), or that you may not see gives `4004`.

## Terminate

Press **Terminate** on the live page and give a reason.

- **Who.** Owners and admins end any session of the organization. A member ends only their own. Agent tokens cannot.
- **Reason.** 1 to 256 characters. It is stored with the audit event `session.terminate`. The user of the session sees only `Session closed: terminated`; the reason is never shown to them.
- **Speed.** The session ends within 2 seconds, on any replica: the request reaches the gateway instance that serves the session through PostgreSQL `NOTIFY`.
- **How.** The gateway cuts the session like a lock or an idle timeout: an SSH client exits with status 255, the end reason is `terminated` and the recording closes.
- **Lost signal.** The gateway gets pending terminations again when its tunnel reconnects, so a lost `NOTIFY` does not leave the session running.
- **Already ended.** Terminating an ended session fails with `session already ended`.

## Close codes

The WebSocket closes with one of these codes and a short reason:

| Code | Reason | Meaning |
| --- | --- | --- |
| `1000` | `closed` | Normal end: the session ended or the browser left. |
| `4001` | `mfa_required` | An [MFA rule](mfa.md#mfa-rules) covers the resource and the browser session has no fresh enough factor, or the terminal was opened with a Bearer token. The UI asks for a step-up and opens the terminal again. |
| `4003` | `denied` | No access, or the resource is unknown. |
| `4004` | `not_found` | Live view: the session is unknown, ended, not a terminal or not visible to you. |
| `4008` | `limit` | A replica holds 1000 streams, or the session has 16 observers. |
| `1011` | `error` | The stream failed: the gateway did not open it within 10 seconds, a replica died or is starting or stopping, or the connection was lost. |
| `1011` | `Gateway is unavailable` | The resource exists but its pool has no online instance. |
| `1009` | | A browser message was larger than 64 KiB. |

Before the upgrade the server answers plain HTTP: `403` when `Origin` is not the origin of `WERBOT_WEB_URL` or the principal is an agent, `401` without a session, `400` for a window outside the bounds.

## Limits

- 1000 streams per server replica, counting browser connections and bridges to other replicas. The limit is per replica, not per user: one member can fill it.
- The gauge `werbot_relay_streams` shows the streams a replica holds now; `werbot_relay_rejected_total{reason}` counts refused ones ([scaling.md](scaling.md#metrics)).
- Replicas reach each other on `WERBOT_INTERNAL_ADDR` (mTLS, default `:8444`). Each replica publishes `WERBOT_INTERNAL_URL` for the others to dial. The network between replicas must allow it ([scaling.md](scaling.md#internal-listener)).
- A ticket is single use, valid for 30 seconds, and tied to the organization and the gateway instance.
- The browser connection is pinged every 30 seconds; the load balancer must keep idle WebSockets longer ([scaling.md](scaling.md#websocket-streams)).
- One UI origin: `Origin` must equal `WERBOT_WEB_URL` ([web.md](web.md#websocket)).
- When a server replica that carries a stream dies, the stream and its web session end. SSH sessions are not affected ([failure-modes.md](failure-modes.md)).
