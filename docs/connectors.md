# Connectors

For developers who add a target type. A connector serves one resource type on the gateway; the server, UI and connect hints take the type from the registry.

## Pipeline

A front accepts the user in its own protocol and produces a session of one kind. A connector serves the kinds it declares. A new front or connector costs M+N, not M×N.

- Kinds: `terminal`, `exec`, `stream`, `datagram` (`http` and `operation` come later).
- Front `ssh`: shell and exec with a PTY give `terminal`; exec without a PTY gives `exec`; `direct-tcpip` gives `stream`; the channel `datagram@werbot.com` (only `werbot connect`) gives `datagram`.
- Fronts `tunnel`, `mcp` and `web` are on the roadmap.
- The interfaces live in `internal/gateway` and move to `sdk` unchanged in wave 1.

## The Connector interface

```go
type Connector interface {
	Type() string
	Kinds() []Kind
	Serve(ctx context.Context, s *Session) error // ctx is canceled on terminate and drain
}
```

`Session` carries the fields of its kind: `Terminal`, `Exec`, `Stream` or `Datagram` (a `Datagrams` with `Recv`, `Send` and `Close`), plus `Target`, `PinKey`, `Recording`, `Resource`, `Secret` and `Dial`. `Dial` connects to the target; on the embedded gateway it refuses local addresses. A connector that reaches its target through `Dial` (`telnet`, `tcp`, `udp`) also implements `TargetDialer` (`DialsTarget()`), so the pipeline checks the host before it starts the session.

## What the pipeline does

- Enforces `WERBOT_GATEWAY_MAX_SESSIONS` and `WERBOT_GATEWAY_SPOOL_MAX`.
- Authenticates the user and checks the policy `<type>.connect`.
- Checks that the kind is in `Kinds()`.
- Fetches the credential through the server for one session.
- Records the session.
- Recovers panics: the session ends with `internal_error`, neighbours live.
- Cancels `ctx` on terminate and drain.
- Delivers `EndSession` through the spool.
- Counts metrics and logs the error from `Serve` without the secret value.

## What a connector must do

- Work only through `Session`.
- Not log: return the error.
- Start nothing before the first `Serve` (0 goroutines, 0 listeners).
- Return from `Serve` when `ctx` is canceled.
- Write output and input to `Recording.Output` and `Recording.Input`.
- Verify the target host key through `PinKey` (TOFU through the server).
- Take copy buffers from the shared `sync.Pool`.

## Adding a connector

1. Add the resource type package `internal/connector/<type>`: `connector.Register` with its `Kinds` (server, UI and the connect hint read it), `Recorded` (a subset of `Kinds`) and, for `stream` or `datagram`, `Endpoint`.
2. Add the gateway package `internal/gateway/<type>connector` with the `Connector` and `gateway.RegisterConnector(c)` (a duplicate type or empty `Kinds` panics).
   Blank-import both packages in `internal/app/connectors.go`: without the import the server does not know the type and the gateway does not serve it.
3. Add the Cedar action `<type>.connect`.
4. Add a `_test.go` with `connectortest.Run(t, c, target)`.
5. Instances announce their connectors at enrollment and in every heartbeat: a resource of the new type is served only by a pool where every online instance has the connector.

## Contract test

`connectortest.Run` checks that:

- every declared `Kind` works with the test target;
- a canceled `ctx` ends `Serve` within 1 s;
- no goroutine is left;
- the secret value is not in the error from `Serve` or in the log;
- `Recording.Output` and `Recording.Input` are used for the recorded kinds only;
- no goroutine starts between registration and the first session;
- `Kinds()` equals the type on the server;
- a `TargetDialer` dials only through `Session.Dial`.

`connectortest.Echo` is an example (`stream`, reachable with `ssh -L`).

## Built-in connectors

| Type | Level | Kinds | Recorded | Notes |
| --- | --- | --- | --- | --- |
| `ssh` | L2 | `terminal`, `exec`, `stream` | `terminal`, `exec` | `port_forwarding` is off by default; `stream` is `ssh -L`. `remote_forwarding` is off by default; with it, `ssh -R` is a `stream` (see below). |
| `telnet` | L2 | `terminal`, `stream` | `terminal`, `stream` | Auto-login and recording. The gateway to target leg is plain text: run the gateway next to the equipment. |
| `tcp` | L1 | `stream` | none | Several ports, RBAC per port; only bytes are counted. |
| `udp` | L1 | `datagram` | none | Request-response protocols only: DNS, SNMP, syslog, NTP, RADIUS. RTP and VoIP do not fit: UDP rides over TCP. `flow_idle` (default `2m`), up to 32 flows per session, 8192 bytes per datagram. |

`Endpoint` sets the host and ports a `stream` or `datagram` session may reach (and the TCP health check); `Recorded` sets the kinds whose payload is recorded as asciicast. The health check is a TCP connect from two instances of the pool; `udp` has none.

## Remote forwarding (`ssh -R`)

A resource of type `ssh` with `remote_forwarding` on accepts remote forwards on a direct login (`<login>+<resource>`). The gateway asks the target to listen on the bind address and opens a `forwarded-tcpip` channel to the client for every connection that arrives there.

- Policy: the request is decided as `ssh.connect` (with the MFA rules, so the second factor comes from the login) and then as `ssh.remote_forward`, both with the bind port as `context.port`. Bindings and JIT grants already cover `ssh.remote_forward`; narrow it with a custom Cedar `forbid` on the action. Agents never get it: their baseline permits only `exec`.
- Bind address: only `localhost`, `127.0.0.1`, `::1` or an empty host (the target's `localhost`). Any other host, `0.0.0.0` included, and port `0` are refused, as is a second forward of the same address on the connection. The client gets back the bind host it sent.
- Session: one `stream` session per forward, target `<bind_host>:<bind_port>` (an empty host is `localhost`), not recorded; only bytes are counted. Port-bound bindings limit the bind port.
- End: `cancel-tcpip-forward` or closing the connection ends the session as `closed`. When the gateway ends it (`access_revoked`, `terminated`, `idle_timeout`, `max_duration`, a target failure), it closes the whole SSH connection, its other channels included, so the client learns that the forward is gone.
- Idle timeout: a forward without traffic idles out like any `stream`. Raise the resource `idle_timeout` (up to 24 h) for long-lived forwards.
- Run `ssh -R` as a separate `ssh` with `-N -o ExitOnForwardFailure=yes`: a refused forward then fails the command, and a gateway-ended forward does not take other sessions with it.

## Heavy and third-party connectors

A heavy connector (for example `guacd` for RDP) or a third-party one implements the same `Connector` through an adapter to an external process (go-plugin or a sidecar), passes the same `connectortest.Run` and runs only on specialised pools, so other pools do not pay for it. There is no adapter in `v0.x`: it comes with the first heavy connector.
