# werbot connect

`werbot connect` opens local ports that reach `tcp`, `udp` and other port resources through a gateway.

## Usage

```sh
werbot connect [flags] [<local_port>:]<resource>[:<target_port>] ...
werbot connect --gateway gw.example.com --login alice pg
psql -h 127.0.0.1 -p 5432
```

It opens a local port per resource on `127.0.0.1` and prints `<resource> → 127.0.0.1:<port> (tcp|udp)`.

Variables and flags `WERBOT_CONNECT_GATEWAY` (`--gateway`, required, `host[:port]`, port defaults to `2222`) and `WERBOT_CONNECT_LOGIN` (`--login`, required): see [configuration.md](configuration.md#client-werbot-connect-werbot-login).

Other flags: `-i <key>` (otherwise the ssh-agent at `SSH_AUTH_SOCK` when it answers, otherwise `~/.ssh/id_ed25519`; an encrypted key file fails with `key <path> is encrypted; load it into ssh-agent`; the agent is never forwarded) and `--known-hosts <path>` (default `~/.ssh/known_hosts`).

- Each resource gets its own SSH connection as `<login>+<resource>`. The client asks `resource@werbot.com` for the resource and opens a UDP port when it has the `datagram` kind, otherwise a TCP port for `stream` (otherwise `Resource <resource> does not support port access`).
- Target port: `:<target_port>`, or the only port of the resource. A resource without ports, with several ports or with a range needs `:<target_port>` (`resource web has several ports (8000-8002); use web:<port>`).
- Local port: `<local_port>`, or the target port, plus 10000 below 1024 (`23` becomes `10023`, `53` becomes `10053`). A busy port fails with `local port <n> is busy; use <free>:<resource>`.
- The port listens on `127.0.0.1` only, but, as with `ssh -L`, every local user of the machine can connect to it.
- Each TCP connection opens its own `direct-tcpip` channel (one gateway session, one access check) and passes half-closes on.
- A UDP port uses one `datagram@werbot.com` channel where each client address is a flow (at most 32; a new address beyond them is dropped until a flow has had no traffic for 2 minutes). The client frees a flow after 2 minutes whatever the resource's `flow_idle`: with `flow_idle` above 2 minutes and many client addresses, the gateway still holds the old flows, so its 32-flow cap can drop new flows until they age out.
- After a refused or failed channel open, datagrams are dropped for a backoff of 1 s doubling up to 30 s, and the same text is printed once.
- Gateway texts (`Access denied`, `Gateway is busy, try again later`, ...) are printed as they are. A refused resource exits with code 1; Ctrl+C closes everything with code 0.

Examples:

```sh
werbot connect pg                       # TCP: psql -h 127.0.0.1 -p 5432
werbot connect router                   # Telnet resource (port 23): telnet 127.0.0.1 10023
werbot connect dns                      # UDP (port 53): dig @127.0.0.1 -p 10053 example.com
```

## With werbot login

After [`werbot login`](cli.md) the login, the gateway, the key and `known_hosts` come from the profile, so `werbot connect pg` is enough. The gateway is the address of the resources' pool; resources in different pools need `--gateway`. The key is the profile certificate (renewed on each reconnect); `-i` and `--known-hosts` override the profile. Without a profile `werbot connect` works as described above.

## Without the client

TCP and Telnet need no client: `ssh -L <port>:<resource>:<port> <login>+<resource>@<gateway> -p 2222`, then connect to `127.0.0.1:<port>` (for Telnet, `telnet 127.0.0.1 2323` after `-L 2323:<resource>:23`). The resource must be a `tcp` or `telnet` type, or an `ssh` resource with `port_forwarding`. UDP works only through `werbot connect`.

## Host keys

The gateway host key is checked like OpenSSH `StrictHostKeyChecking=accept-new`: an unknown key is appended to `known_hosts` (created `0700`/`0600`); a different key fails with `host key mismatch for <host:port>`. All instances of a pool share one host key.

## Reconnects

A `keepalive@openssh.com` goes every 30 s; three unanswered in a row count as a drop. After a drop the client dials again with a backoff of 1 s doubling up to 30 s while the local ports stay open, and prints each new dial error (`<resource>: reconnect failed: <error>; retrying`), so the balancer can route it to another instance.

- Running TCP connections end; a new one waits up to 10 s for the connection and is closed otherwise.
- UDP opens a new channel (a new session with an empty flow table) on the next datagram.
- A changed host key or a refused key ends the client with an error.

## Windows

Releases ship `werbot` for Windows as a `.zip` archive. On Windows the client commands (`login`, `ls`, `ssh`, `ssh-config`, `connect`, `status`, `logout`) are supported; the server, gateway and web roles are not.
