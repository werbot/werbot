# werbot CLI

`werbot login` signs the CLI in to a server. Afterwards `werbot ssh`, `werbot ssh-config` and `werbot connect` use a short-lived SSH certificate instead of a key you register by hand.

## Log in

```sh
werbot login --server https://werbot.example.com
```

The CLI opens the browser (the authorization code flow with PKCE and a redirect to `127.0.0.1`). The browser shows a consent page; approve it. On success the CLI prints `Logged in to <server> as <login>; certificate valid until <time>`. If the browser cannot be opened the URL is printed.

- `--device` uses the device flow (RFC 8628) for a machine without a browser: the CLI prints a code and a URL to open on another device.
- The server comes from `--server`, else `WERBOT_SERVER`, else the server of the last login. Without any of them the command fails with `--server is required`.
- Use an `https://` server URL in production. Over `http://` the tokens travel in clear text ([security.md](security.md#cli-sessions-and-user-certificates)).

## Profiles

Profiles live in `WERBOT_HOME` (default `~/.werbot`, Windows `%USERPROFILE%\.werbot`), one directory per server:

| File | Content |
|---|---|
| `profile.json` | server, login, access and refresh token |
| `id_ed25519`, `id_ed25519-cert.pub` | the CLI key and its user certificate |
| `known_hosts` | host keys of the gateways, written by `ssh`, `ssh-config` and `connect` |
| `current` (in the home) | the server of the last login |

Files are `0600`, directories `0700` ([security.md](security.md#cli-sessions-and-user-certificates)). Set `WERBOT_SERVER` to use another logged-in server for one command.

## Commands

| Command | Does |
|---|---|
| `werbot ls` | Table `NAME TYPE KINDS GATEWAY ACCESS` of the resources you can open (`ACCESS` `yes`) or request (`request`). |
| `werbot ssh <resource> [-- command...]` | Runs the system `ssh` with the profile key, certificate and `known_hosts` as `<login>+<resource>`. The exit code of `ssh` is the exit code of `werbot`. |
| `werbot ssh-config` | Prints an OpenSSH config block `Host <resource>.<server-host>` for each resource with the `terminal`, `exec` or `stream` kind, and refreshes the certificate and `known_hosts`. |
| `werbot connect <resource> ...` | As in [connect.md](connect.md), with the login, gateway, key and `known_hosts` taken from the profile. |
| `werbot request <resource>` | Asks for access and waits for the decision ([Access requests](#access-requests)). |
| `werbot requests` | Your access requests; `--to-approve` lists the pending requests you can decide. |
| `werbot approve <id>`, `werbot deny <id>` | Decide a request; `--comment` adds a note for the requester. |
| `werbot status` | Server, user, organization, token expiry and certificate; renews a certificate close to expiry. |
| `werbot logout` | Revokes the session on the server and removes the profile. |
| `werbot mcp` | Serves MCP on stdio as an agent of your account ([AI agents](#ai-agents)). |
| `werbot agent run -- <cmd>` | Runs a program with `ssh` set up for an agent run ([AI agents](#ai-agents)). |
| `werbot agents` | Table `NAME STATUS MAX ACTIONS MAX DURATION MAX CONCURRENT ACTIVE RUNS` of your agents (owners and admins: all). |
| `werbot agents profile create`, `list`, `revoke` | Manage agent profiles ([AI agents](#ai-agents)). |

`ssh`, `ssh-config` and `connect` use only the resources you can open. `werbot ssh` to a resource you can only request fails with `resource <r> is not accessible; run werbot request <r>`.

A resource without a gateway address fails with `gateway address for <resource> is not set; ask an admin to set it on the Gateways page` ([gateway.md](gateway.md#ssh-address)).

## Access requests

Resources marked `request` in `werbot ls` need an approved request first ([jit.md](jit.md)).

```console
$ werbot request db --for 2h --reason "deploy 1.4"
Request 019a3c4e-7d1f-7a2b-9c3d-5e6f7a8b9c0d is waiting for approval: https://werbot.example.com/requests?id=019a3c4e-7d1f-7a2b-9c3d-5e6f7a8b9c0d
Access to db granted until 2026-11-20T16:30:00Z
```

- `--for` is the grant length (`30m`, `2h`); without it the organization default applies. A longer duration than the policy or the organization allows is refused.
- `--reason` is required when the policy asks for one.
- A resource without a matching approval policy fails with `resource <r> is not requestable`.
- `--break-glass` (needs `--reason`) gives emergency access at once, for at most 1 hour, with an email to every owner and admin and a review afterwards.
- The command checks the request every 5 seconds. Ctrl-C cancels the request and prints `Request <id> canceled`.
- An auto-approved or break-glass request prints only the grant line.

```console
$ werbot requests --to-approve
ID                                    STATUS   RESOURCE  REQUESTER  DURATION  EXPIRES               REASON
019a3c4e-7d1f-7a2b-9c3d-5e6f7a8b9c0d  pending  db        bob        2h0m0s    2026-11-20T15:30:00Z  deploy 1.4
$ werbot approve 019a3c4e-7d1f-7a2b-9c3d-5e6f7a8b9c0d --comment ok
Request 019a3c4e-7d1f-7a2b-9c3d-5e6f7a8b9c0d approved
```

`EXPIRES` of a pending request is its decision deadline; of an approved request, the end of the grant. Nobody can approve their own request.

Exit codes: `0` when access is granted, the request is canceled with Ctrl-C, or `approve`/`deny` succeeds; `1` with the message on any error, including `Request <id> was denied: <comment>` and `Request <id> expired`.

## Certificates

The server signs an SSH user certificate for the CLI key and the gateways accept it directly.

- A certificate is valid for 1 hour. `werbot ssh`, `ssh-config`, `connect` and `status` renew it when less than 10 minutes remain.
- When renewal fails, a certificate that is still valid is used as it is.
- If the clock of the computer differs from the server clock by more than 1 minute, renewal fails with a `clock skew` error: sync the system clock.

## OpenSSH config

```sh
werbot ssh-config >> ~/.ssh/config.d/werbot   # or Include it from ~/.ssh/config
ssh db.werbot.example.com
```

The block sets `User <login>+<resource>`, `IdentityFile`, `CertificateFile`, `UserKnownHostsFile` and `IdentitiesOnly yes`. The certificate in it is renewed by `werbot ssh`, `ssh-config`, `connect` and `status`, so run `werbot status` before a long-idle `ssh`. A resource without a gateway address gets a comment line. Resources that only have the `datagram` kind are skipped: use `werbot connect`.

## CLI sessions

Every login is a CLI session. An organization sets its lifetime in Settings (idle timeout and maximum age). A user sees the sessions in Profile, CLI sessions and revokes one there; `logout` revokes the current one, and a new `login` to the same server revokes the previous one. A revoked session stops working at the next API request and the next gateway connection: commands fail with `session expired or revoked; run werbot login`. Open connections keep running; to end them, an owner or admin locks the CLI session on the Locks page ([jit.md](jit.md#revoking-a-cli-session)).

Refresh-token rotation and reuse detection: [security.md](security.md#cli-sessions-and-user-certificates).

## AI agents

The guide is [agents.md](agents.md).

```text
werbot mcp [--agent <name>] [--reason <text>]
werbot agent run [--agent <name>] [--reason <text>] [--resource <name>]... -- <cmd> [args]
werbot agents
werbot agents profile create <agent> --dir <path> [--ttl <duration>] [--label <text>]
werbot agents profile list [<agent>]
werbot agents profile revoke <id>
```

- `werbot mcp` starts a run of the agent on the first tool call and ends it when stdin closes. `--agent` defaults to the MCP client name. It exits `0` at the end of stdin and `1` when no profile can be loaded; a refused token exchange is a tool error, not an exit.
- `werbot agent run` exits with the exit code of `<cmd>`, and `1` with the message when the run cannot start. `--agent` defaults to the command name; `--resource` (repeatable) limits the setup to those resources and opens local ports for `tcp` and `udp` ones.
- `werbot agents profile create` writes an agent profile into `--dir` (missing or empty, `0700`): another OS user or a container runs `WERBOT_HOME=<dir> werbot mcp`. It creates the agent if you have none of that name. `--ttl` is `1h` to `90d` (default: the organization's `cli_session_max`); `--label` defaults to the host name. `list` prints `ID AGENT LABEL CREATED LAST USED EXPIRES`; `revoke` takes an id from it.

Profile modes:

- **Shared**: with your own profile, `werbot mcp` and `werbot agent run` print `warning: the agent runs as your OS user and can read your werbot profile in <home>; isolate it with "werbot agents profile create"`. The agent can read your tokens and key ([agents.md](agents.md#shared-the-sponsors-profile)).
- **Isolated**: with an agent profile (`WERBOT_HOME` pointing at its directory) only `mcp`, `agent run`, `status` and `logout` work; other commands fail with `this is an agent profile: only werbot mcp, werbot agent run, werbot status and werbot logout use it`. `--agent` must be the profile's agent or empty.

## Windows

`werbot login`, `ls`, `status`, `ssh-config` and `connect` work on Windows. `werbot ssh` needs the OpenSSH client (`ssh.exe`) in `PATH`. `werbot agent run` writes the wrapper `bin\ssh.cmd`; a child that starts `ssh` without a `PATHEXT` lookup runs the system `ssh.exe` instead, and the Windows OpenSSH agent stays reachable to it ([agents.md](agents.md#werbot-agent-run)).
