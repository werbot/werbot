# AI agents

Werbot lets an AI agent (Claude Code, Codex, Cursor or any MCP client) run commands on your resources without its own SSH key or password. The agent acts for a person, its **sponsor**, and can never do more than the sponsor may. On top of that it gets a read-only baseline, limits per run and a kill switch. This guide covers the model, `werbot mcp`, `werbot agent run`, the profile modes, policies for agents, limits, locks and the run timeline.

## Model

| Term | Meaning |
| --- | --- |
| Agent | A named identity of one sponsor (`claude`, `codex`), unique per sponsor. Kind `interactive`: it acts while a person works with it. Status `active` or `frozen`. |
| Sponsor | The member the agent acts for. An agent reaches only resources its sponsor reaches, and every decision is the sponsor's decision **and** the agent's decision. |
| Run | One session of an agent, from the first tool call to the end of the client: `running`, `paused` or `ended`. It has its own token (1 hour), its own SSH key in memory and its own limits. |
| Tool call | One MCP call of the agent (`ssh_exec`, …), recorded with its arguments masked by key. |

A run starts when the CLI exchanges the sponsor's token for a run token (RFC 8693 token exchange, scope `agent:<name>`). The exchange creates the agent on first use. A run ends when the client exits (`client_exit`), when a person ends it (`ended_by_user`) or when it expires (`expired`, checked every minute).

The run token calls only what an agent needs: a user certificate, the accessible resources, access requests of its run, its tool calls and its own run. Everything else, and every route outside the API (recordings, OAuth), is refused with `not allowed for agent tokens` or `403`. An agent never approves, unlocks, changes limits or resumes a run.

Owners and admins see every agent and run on **AI agents**; a member sees their own.

## `werbot mcp`

`werbot mcp` is an MCP server on stdio. Log in once with [`werbot login`](cli.md#log-in), then add it to your MCP client:

```json
{
  "mcpServers": {
    "werbot": { "command": "werbot", "args": ["mcp", "--agent", "claude"] }
  }
}
```

- Claude Code: `claude mcp add werbot -- werbot mcp --agent claude`.
- Codex (`~/.codex/config.toml`): `[mcp_servers.werbot]` with `command = "werbot"` and `args = ["mcp", "--agent", "codex"]`.
- Cursor (`.cursor/mcp.json` or `~/.cursor/mcp.json`): the JSON above.

`--agent` names the agent; without it the agent is named after the MCP client (`claude-code`: lower case, other characters as `-`; `unknown` when the client sends no name). `--reason` is shown to approvers. stdout carries only JSON-RPC; logs go to stderr. The run starts on the first tool call and ends when the client closes stdin. A failed token exchange is a tool error, not an exit.

### Tools

| Tool | Input | Output |
| --- | --- | --- |
| `list_resources` | none | Resources with `name`, `type`, `kinds`, `labels`, `allowed_classes`, `approval_classes`, `requestable`. A resource the sponsor cannot reach is never listed. |
| `ssh_exec` | `resource`, `command`, `timeout` (seconds, 1 to 600, default 60), `approval_id` (for the transcript only) | `status` (`ok`, `denied`, `error`, `pending_approval`), `exit_code`, `stdout`, `stderr`, `truncated`, `approval_id`, `approval_url`, `message` |
| `request_access` | `resource`, `duration` (Go duration, empty: the organization default), `reason` | `request_id`, `status`, `url` |
| `run_status` | none | Status, actions used and left, expiry and the approvals the run waits for |

`ssh_exec` runs one command without a terminal through the resource's gateway, with the run certificate and the gateway host key from `list_resources` (no trust on first use). A relative path starts at the login's home directory. Output is stdout and stderr together, at most 64 KiB; the rest is dropped and `truncated` is set. A command that waits for an approval longer than 50 seconds returns `pending_approval` with the link; call `ssh_exec` again with the same resource and command once it is approved. A refusal of the gateway is `denied` with its message (`Command denied: …`, `Access denied`); `Agent limit exceeded` and `Agent run is not active` are `error`. A command the gateway ends (a lock, a revoke, the idle timeout or the max duration) has no exit code and is `error` with `the session ended without an exit status`.

## `werbot agent run`

`werbot agent run -- <cmd> [args]` runs any program as an agent run, for tools that call `ssh` themselves:

```sh
werbot agent run --agent ops --reason "nightly check" -- ./check.sh
```

- The run directory `<WERBOT_HOME>/runs/<run id>` (`0700`, files `0600`) holds the run key and certificate, `known_hosts`, an `ssh_config` with one `Host <resource>` block per ssh resource and `bin/ssh`, a wrapper that runs the system `ssh` with that config. The directory is removed when the program exits, also after a failed start.
- The program runs with `bin` first on `PATH`, an empty `SSH_AUTH_SOCK` (your SSH agent stays hidden) and without `WERBOT_HOME` and `WERBOT_SERVER`. `ssh <resource> <command>` inside it is the agent. The `ssh_config` offers only the run key (`IdentitiesOnly yes`, `IdentityAgent none`), never the keys of an SSH agent.
- On Windows the wrapper is `bin\ssh.cmd`. The program itself is found in `bin` first, and so is `ssh` for a child that looks programs up through `PATHEXT` (`cmd`, PowerShell, Go). A child that starts `ssh` without that lookup (for example Python `subprocess` or Node `child_process` without a shell) runs the system `ssh.exe` with your own configuration instead. The Windows OpenSSH agent listens on a fixed named pipe, so an empty `SSH_AUTH_SOCK` does not hide it from such a child. Use an [agent profile](#isolated-an-agent-profile) under another Windows user to keep your keys out of the agent's reach.
- The certificate is renewed while the program runs. The exit code of the program is the exit code of `werbot`; Ctrl-C reaches the program; a program that has not left 2 seconds later is killed.
- `--resource` limits the setup to the named resources. A `tcp` or `udp` resource named there gets a local port as in [`werbot connect`](connect.md). The baseline gives agents no ports: allow them with a custom Cedar policy, for example

```cedar
permit (principal is Werbot::Agent, action == Werbot::Action::"tcp.connect", resource == Werbot::Resource::"<resource id>")
when { principal.sponsor == Werbot::User::"<sponsor user id>" };
```

The banner on stderr prints the run id, the wrapper and the MCP config for the same agent.

## Profile modes

The CLI that runs the agent needs a credential. There are two modes.

### Shared: the sponsor's profile

`werbot mcp` and `werbot agent run` use your own profile in `~/.werbot` and print one warning on stderr:

```text
warning: the agent runs as your OS user and can read your werbot profile in /home/alice/.werbot; isolate it with "werbot agents profile create"
```

This is a documented limit. The agent process runs as your OS user, so it can read your refresh token and key and act as you, past its baseline. A different `HOME` or `WERBOT_HOME` under the same OS user does not change that. Use the shared mode only with an agent you would trust with your own login.

### Isolated: an agent profile

An agent profile can only start runs of one agent. Create it for another OS user or a container:

```sh
werbot agents profile create claude --dir /srv/agent-claude/.werbot --label build-box
sudo chown -R agent /srv/agent-claude/.werbot
sudo -u agent env WERBOT_HOME=/srv/agent-claude/.werbot werbot mcp
```

In an MCP config set `"env": {"WERBOT_HOME": "/srv/agent-claude/.werbot"}` next to `command` and `args`. The directory must be missing or empty; it gets the server, the agent and a refresh token of the profile, never your tokens, key or certificate.

- The profile's access token is good for one thing: the token exchange for a run of its agent. Every API call with it is `Unauthenticated`, an exchange for another agent `invalid_scope`.
- Lifetime: `--ttl` from `1h` to `90d`, at most the organization's `cli_session_max` (default and maximum). An idle profile expires after `cli_session_ttl`.
- At most 5 live profiles per agent.
- A profile is revoked by `werbot agents profile revoke <id>`, on **AI agents**, **Profiles**, by freezing or deleting the agent, when the sponsor leaves the organization and when the sponsor's password changes. A revoked profile starts no run, and the live run tokens and certificates of its runs stop at the next request or connection.
- `werbot status` and `werbot logout` work with an agent profile; other commands refuse it.

## What an agent may do

### Baseline

Every agent gets built-in policies (`agent:baseline:<name>` in the decision reasons). They come on top of the sponsor's bindings, grants, rules and locks:

| Baseline | Effect |
| --- | --- |
| `connect` | An agent opens exec sessions only (`<type>.connect` with kind `exec`). |
| `read` | A command whose classes are exactly `read` runs. |
| `dangerous` | `destructive`, `secrets` and `egress` are denied, even under an `allow` rule. |
| `terminal` | No shell, no PTY and no command console. |
| `sftp` | No file transfer. |
| `frozen` | A frozen agent is denied everything. |
| `blocked_runs` | A paused or locked run is denied everything. |

So by default an agent runs `read` commands (`ls`, `cat`, `grep`, `uname`, …) where its sponsor may, and nothing else. `read` still reads any file the login can read; limit it with a folder rule (below). Ports (`tcp`, `udp`) need a custom `permit`.

### Command rules for agents

[Command rules](policies.md#command-rules) take agents as subjects: `all_agents`, or users and teams, which mean the agents those users (or team members) sponsor. `all_users` covers people only.

- `allow` lets an agent run a command whose classes are all among the rule's classes (`write`, `opaque`, `privilege`; list `read` too for commands that also read, such as `cat a > b`). It never lifts the `dangerous` baseline.
- `approval` asks a person before an agent runs such a command; the same "all among the rule's classes" applies.
- `deny` refuses them, as for people.

A command of an agent runs only when the sponsor's pass and the agent's pass both allow it. An approval is asked only when every refusing pass asks for one.

### Folder rules for agents

[Folder rules](policies.md#folders) take the same subjects. An exec of an agent starts in `~`, so write home folders with `~`. A tight setup lets an agent read only its project and logs:

```text
name: agents-read-scope   subjects: all_agents   access: read   effect: only   paths: ~/app/**, /var/log/**
```

An `approval` folder rule for an agent lifts the path refusal once approved; it does not widen the classes.

### Command approvals

When a command of an agent needs an approval, the approvers of its sponsor decide ([policies.md](policies.md#command-approvals)). The sponsor may approve their own agent's command unless a matching approval policy sets `four_eyes`. An approval belongs to the run: the same command on the same resource in the same run reuses it while it is pending and for 10 minutes after it was approved.

### Access requests

`request_access` needs an approval policy with `all_agents` among its requesters. The grant goes to the sponsor, so it opens the resource to the sponsor's own sessions too ([jit.md](jit.md#access-requests)).

## Limits

Each agent has limits; a run takes them when it starts, so a change applies to new runs only.

| Limit | Default | Range |
| --- | --- | --- |
| `max_actions` | 100 | 1 to 10000 sessions per run |
| `max_duration` | 1h | 1m to 1h |
| `max_concurrent` | 4 | 1 to 32 open sessions per run |

An action is a session the gateway opens. Past a limit the gateway refuses the command with `Agent limit exceeded` before it connects to the resource; that refusal is not a denial. A run also records at most 1000 tool calls, each with arguments up to 16 KiB.

- A run past its `max_duration` refuses new actions at once, but its open sessions end when the expiry job ends the run, which runs every minute: up to about one minute later.
- If the server fails to sign the run token after it started the run, the client gets no token and the run stays `running` without one until it expires.
- The gateway decides an agent missing from its policy snapshot as active for 30 seconds after the agent's SSH login. A session of a deleted agent opened in that window ends at the next re-evaluation, up to about 90 seconds after the login.

### Pause after denials

Five policy denials in a row pause the run; an allowed action resets the count. A paused run refuses every tool call, `run_status` included, with `agent run is not active`. A person resumes it on the run page (**Resume**, `ResumeRun`). Right after a resume the gateway may refuse the run for about a second more, until its next policy snapshot. Refusals by the run's own state (a paused or ended run, a frozen, locked or deleted agent) are audited as denials but never count toward the pause.

A denial of a `secrets` or `egress` command emails `agent_alert` to the sponsor and to every owner and admin.

## Kill switch

Owners and admins stop agents with a [lock](jit.md#locks):

| Target | Stops |
| --- | --- |
| `agent_run` | One run. |
| `agent` | Every run of one agent. |
| `agents_of_user` | Every agent of one sponsor. |
| `all_agents` | Every agent of the organization. |

A lock ends the open sessions of the matched runs within about 2 seconds (`access_revoked`; a command cut short this way has no exit status), and refuses their next tool call (`agent is locked`); a locked agent gets no new run. Only a lock is the owner and admin kill switch: the sponsor cannot remove it.

**Freeze** on **AI agents** (`UpdateAgent` with status `frozen`) stops one agent in the same way and also revokes its profiles, but the sponsor can unfreeze their own agent. **Delete agent** removes the agent with its runs and tool calls and ends its open sessions; the sessions and approvals stay in the history.

In the shared profile mode without `--agent`, the agent name is the name the MCP client reports. A client that reports another name runs as another agent, which a lock on the first agent does not stop. To stop every agent of a sponsor, lock `agents_of_user` (or `all_agents`).

## Run timeline

The run page shows what the run did, newest first:

- **Tool calls**, reported by the client (`werbot mcp`): the tool, its arguments masked by key, the traceparent and the status.
- **Sessions**, **approvals** and **denials**, recorded by the gateway and the server: the command, its classes, the exit code and the recording.

A session of an agent stores its tool call, its exit code and `output_sha256`, the SHA-256 of the output as recorded. The output of agents is masked by default (`mask_output = agents`, [policies.md](policies.md#output-masking)); the hash is of the recorded, unmasked output and matches the replay.

## Custom Cedar for agents

Agents are `Werbot::Agent` entities with `sponsor` (a `Werbot::User`) and `status`. The context adds `kind` (`exec`, `terminal`, `stream`, `datagram`) and `run_id` (empty for people). A `permit` written as `permit (principal, …)` matches agents too; write `principal is Werbot::User` to limit it to people ([policies.md](policies.md#custom-cedar)). A custom policy cannot lift a baseline `forbid`.

## Honest limits

- The shared profile mode does not isolate the agent from its sponsor ([Profile modes](#profile-modes)).
- A run token sent to `/oauth/revoke` ends the CLI session it was exchanged from: in the shared mode that is the sponsor's login. An agent can only cut access this way, never gain it.
- The tool call record comes from the client; the sessions, approvals and denials come from the gateway.
- Phase 2 serves MCP over stdio only, a run lasts at most 1 hour, and there is no `db_query` or `propose_plan` tool.
