# Command policies

Werbot can decide each command a user runs through a gateway, not only whether the user may open the resource. The gateway sorts the command into risk classes, checks the command rules and custom Cedar policies of the organization, and then runs the command, refuses it or asks an approver first. This guide covers the classes, class overrides, command rules, custom Cedar, command approvals, output masking and the test box.

> **Command text is stored.** Every session started with a command keeps it in the session list (`session.command`, up to 64 KiB), and a refused command is kept in its `session.deny` audit event (the first 1024 bytes). A command waiting for approval is stored with the request and shown to the approvers. Owners and admins see all of these. A command line can contain a password or a token: pass secrets through files or environment variables, never as command arguments.

## What is checked

A binding (or a grant) lets a user open a resource; on an `ssh` resource it allows both the action `ssh.connect` and the action `ssh.exec`. The command check runs on top of it, for every session that carries a command:

- `ssh user+resource@gateway <command>` (an exec, with or without `-t`) is checked before it starts.
- An interactive shell (`ssh user+resource@gateway` without a command) is checked only as a connection. Commands typed into the shell are not checked one by one. Use a rule on the connection (or no binding) where every command matters.
- The decision runs on the gateway, from the policy snapshot it already holds; the server is asked only for approvals.

The check fails closed: a command that cannot be parsed, an empty one or one longer than 64 KiB is `opaque`, and an error while a policy is evaluated denies.

## Classes

The gateway parses the command as a bash command line (pipes, lists, `$(…)`, redirections) and gives it every class of every part. A command always has at least one class.

| Class | Built-in examples |
| --- | --- |
| `read` | `ls`, `cat`, `head`, `tail`, `less`, `grep`, `wc`, `df`, `du`, `ps`, `top`, `uptime`, `hostname`, `whoami`, `date`, `echo`, `sort`, `awk`, `jq`, `diff`, `journalctl`; `sed` without `-i`; `find` without `-delete` or `-exec`; `tar t…`; `crontab -l`; `systemctl status`, `kubectl get`, `docker ps`, `git status` and other read-only subcommands |
| `write` | `cp`, `mv`, `mkdir`, `touch`, `ln`, `tee`, `rm` without `-r` or `-f`, `chmod`, `kill`, `vi`, `vim`, `nano`, `tar c…`/`x…`, `unzip`, `sed -i`, `sort -o`, `uniq` with an output file, `date -s`, `hostname <name>`, `dmesg -c`/`-C`, `journalctl --vacuum-…`/`--rotate`, `git … --output`, redirections to a file (`>`, `>>`); `systemctl restart`, `kubectl apply`, `docker pull`, `git pull`, `apt install` and other changing subcommands |
| `destructive` | `rm -r`, `rm -f` (any combination), `dd`, `mkfs`, `truncate`, `shred`, `wipefs`, `fdisk`, `parted`, `reboot`, `shutdown`, `crontab -r`, `find -delete`; `kubectl delete`, `docker rm`, `docker system prune`, `apt remove` |
| `privilege` | `sudo`, `doas`, `su`, `pkexec` (plus the classes of the command they run), `chown`, `useradd`, `usermod`, `passwd`, `visudo`, `setcap`, `chmod 777`, `chmod +s`, writes to `/etc/sudoers` |
| `secrets` | `printenv`, `env` and `set` without arguments, `history`; any argument or redirection that names `~/.ssh`, `.env`, `/proc/*/environ`, `/etc/shadow`, `.aws/credentials`, `.kube/config`, `.git-credentials`, `.netrc`, `.pgpass` |
| `egress` | `curl`, `wget`, `scp`, `rsync`, `nc`, `socat`, `ssh`, `sftp`, `ftp`, `telnet` to any host other than `localhost`, `127.0.0.1`, `::1` and the resource's own host (those are `read`); a proxy option or a host the classifier cannot find is `egress` too |
| `opaque` | Scripts and unknown programs or paths (`./deploy.sh`, `/opt/app/bin/x`), `python`, `perl`, `node` and other interpreters, `bash` without `-c` or with a non-literal `-c`, `eval`, `source`, `base64 -d`, `kubectl exec`, `docker run`, an unknown subcommand of a listed tool, shell functions, and anything that does not parse |

Examples: `tail -n 50 /var/log/syslog | grep error` is `read`; `sudo systemctl restart nginx` is `privilege` and `write`; `cat ~/.ssh/id_ed25519` is `read` and `secrets`.

### Normalization

Before the table is consulted the classifier:

- removes quotes and escapes (`r''m` and `"rm"` are `rm`);
- strips wrappers and classifies the command they run: `env`, `command`, `busybox`, `exec`, `nohup`, `nice`, `time`, `timeout`, `stdbuf`, `xargs`, `find -exec`, and `bash -c '<literal>'` (and `sh`, `zsh`, `dash`, `ksh`), up to 8 levels deep;
- reduces a binary in `/bin`, `/sbin`, `/usr/bin`, `/usr/sbin`, `/usr/local/bin` or `/usr/local/sbin` to its name (`/usr/bin/rm` is `rm`); any other path stays as typed and is `opaque` unless an override names it;
- marks as `opaque` a command whose name is not literal (`$CMD`, `$(…)`) and an assignment to `PATH`, `LD_PRELOAD`, `LD_LIBRARY_PATH`, `BASH_ENV`, `ENV` or `IFS`.

### Limits of the classifier

The classifier is a heuristic. It reads the command line, not what the programs do, and it is a guard on top of the session recording, not a sandbox.

- A script or an unknown program is `opaque`: the classifier cannot see inside it. Treat `opaque` as "may do anything".
- Some `read` programs can still write files or run other programs: `awk`, `sed` without `-i` and `less`, and `curl -o` or `wget` to the local or resource host (they write the download to a file). `vi` and `vim` are `write`, yet they can run other programs too. The table knows the common writing options (`sort -o`, `date -s`, …), not every one. Rule out such programs with an override or a custom policy where it matters.
- The interactive shell is not checked per command (see [What is checked](#what-is-checked)).

## Class overrides

Owners and admins add overrides on **Policies**, **Command rules** (`PolicyService` `CreateClassRule`, `DeleteClassRule`). An override has a pattern and one or more classes. It **replaces** the built-in classes of a matching command; the classes of its arguments (a `secrets` path, for example) still apply.

- The pattern is a glob per word, matched against the normalized command name and its leading arguments: `./deploy.sh`, `kubectl rollout *`, `backup-*`. Every word must match a literal argument.
- When several overrides match, the one with more words wins, then the first by pattern.
- Typical use: give your own scripts a class (`./deploy.sh` → `write`) so they stop being `opaque`.

Overrides reach the gateways with the next policy snapshot. Audit: `class_rule.create`, `class_rule.delete`.

## Command rules

Owners and admins manage command rules on **Policies**, **Command rules** (`CreateCommandRule`, `UpdateCommandRule`, `DeleteCommandRule`). A rule has:

| Field | Meaning |
| --- | --- |
| `name` | 1 to 63 characters. |
| `subjects` | Users, teams, or all users. |
| `resource_selector` | Resource ids, or labels (`key=value`; a resource must carry every pair). A label selector also covers resources added later. |
| `classes` | One or more classes. The rule applies when the command has **any** of them. |
| `effect` | `deny`, `approval` or `allow`. |
| `message` | Optional, up to 256 printable characters, shown to the user on a refusal. |

- `deny` refuses the command: `Command denied: <message>`, or `Access denied` without a message.
- `approval` refuses the command unless an approver allows it first ([Command approvals](#command-approvals)).
- `allow` has no effect on people: a binding already allows every command. It is reserved for agents.
- A `deny` beats an `approval`: a command matched by both is refused without asking anyone.
- When several rules or policies refuse a command, the user sees the message of the first one by policy id.
- There is no built-in rule. A new organization runs every command its bindings allow; the empty rule list offers a preset `opaque` → `approval`.

Rules reach the gateways with the next policy snapshot (within about 2 seconds). A running command is checked again on every new snapshot: a new `deny` rule ends it with `access_revoked`, a new `approval` rule does not. Audit: `command_rule.create`, `command_rule.update`, `command_rule.delete` with the name, effect and classes.

## Custom Cedar

Owners and admins can write their own [Cedar](https://www.cedarpolicy.com) policies on **Policies**, **Custom Cedar** (`CreateCustomPolicy`, `UpdateCustomPolicy`, `DeleteCustomPolicy`). A policy has a name, the Cedar text (up to 64 KiB) and an `enabled` switch; only enabled policies reach the gateways.

What a policy can use:

- `principal`: `Werbot::User`, a member of `Werbot::Team` entities.
- `action`: `Werbot::Action::"<type>.connect"` and `Werbot::Action::"<type>.exec"` (for `ssh`: `ssh.connect`, `ssh.exec`).
- `resource`: `Werbot::Resource` with `name`, `type` and its labels as tags (`resource.hasTag("env") && resource.getTag("env") == "prod"`).
- `context`: `ip`, `country`, `weekday`, `hour` (UTC), `now`, `ip_blocked`, `port`, `cli_session_id`, and for commands `command` (the command text), `classes` (a set of strings) and `approval_granted`.

Annotations:

- `@message("…")` is the text the user sees when this `forbid` refuses the command.
- `@obligation("approval")` on a `forbid` turns the refusal into an approval request; add `unless { context.approval_granted }` so an approved command passes.
- `@id` is reserved: Werbot sets it.

```cedar
@message("Production changes need an approval")
@obligation("approval")
forbid (principal, action == Werbot::Action::"ssh.exec", resource)
when { resource.hasTag("env") && resource.getTag("env") == "prod" && context.classes.containsAny(["write", "destructive"]) }
unless { context.approval_granted };

@message("No package changes at night")
forbid (principal, action == Werbot::Action::"ssh.exec", resource)
when { context.command like "apt*" && (context.hour < 8 || context.hour >= 20) };
```

The server checks a policy when it is saved: syntax, the schema (an unknown attribute, or a tag read without `hasTag`, is an error), the annotations, and a trial evaluation of each policy. The first error comes back with its line and column, and nothing is stored. An error at run time still denies.

> **A custom `permit` widens access.** A `permit` policy allows what it matches even without a binding or a grant, on every resource it covers. Only owners and admins can save one and every change is audited, but review a `permit` as carefully as a new binding. Prefer `forbid` policies, which can only take access away.

Audit: `custom_policy.create`, `custom_policy.update`, `custom_policy.delete` with the name and `enabled`; the Cedar text is not in the audit log.

## Command approvals

When only an approval can lift the refusal, the gateway asks the server for one and the command waits. The user sees, on stderr of an exec or in the terminal:

```text
Waiting for approval 0199f0c2-…, timeout 5m
https://werbot.example.com/requests?id=0199f0c2-…
```

- The approvers get the `access_request` email and see the request, with the command and its classes, on **Requests**. A command approval is never approved automatically and nobody approves their own command. The approvers are those of the approval policies that match the user and the resource; without a matching policy, the owners and admins ([jit.md](jit.md#approval-policies)).
- The wait lasts `command_approval_ttl` (default `5m`, `30s` to `1h`, **Settings**). The gateway learns the decision from the tunnel at once and also reads it every 15 seconds, so a lost notification delays the command by at most 15 seconds.
- **Approved**: the gateway decides the command again on its current policy (a binding, lock or rule may have changed meanwhile) and runs it on the resource as it is now. If the resource's connection settings (for example its host or port) changed during the wait, the command is refused with `Access denied`. One approval covers one run of one command.
- **Denied** or canceled: `Command denied: <message>`, or `Command denied: approval denied`.
- **No decision in time**: `Approval timed out`. A timeout is always a refusal.
- The session counts against `WERBOT_GATEWAY_MAX_SESSIONS` while it waits, and the gateway sends SSH keepalives so the connection stays open.
- If the server cannot be reached when the approval is requested, the user gets `Gateway is not ready, try again later`. If the server cannot take the request (for example, nobody but the user could approve), the user gets `Command denied: approval denied`.
- A command longer than 16384 bytes cannot be sent for approval: it gets `Command denied: approval denied` and no request is created.
- If the user disconnects or closes the session channel while waiting (a multiplexed connection may stay open), the wait ends at once and the request stays `pending` until it expires. An approver may still approve it; nothing runs.

Refusals are audited as `session.deny` with the reason `denied`, `approval_denied` or `approval_timeout`; decisions as `access.request.approve` and `access.request.deny` with `kind` `command`.

## Output masking

The gateway can hide secrets in the output of exec sessions. The organization setting `mask_output` (**Settings**) chooses whose output is masked:

| Value | Masked |
| --- | --- |
| `agents` (default) | Sessions of agents only. People's output is not masked. |
| `all` | Every exec session. |
| `none` | Nothing. |

What is masked, each match replaced by `[masked]`:

- the credential the gateway used for the session (the resource password or key), wherever it appears;
- AWS access key ids, GitHub tokens and JWTs;
- private key blocks (`-----BEGIN … PRIVATE KEY-----` to `-----END … PRIVATE KEY-----`);
- values of at least 8 characters that look random, assigned to a key such as `password`, `secret`, `token` or `api_key` (`password=…`, `token: …`).

Limits:

- Only exec output (stdout and stderr, without a PTY) is masked. A terminal, including `ssh -t … <command>`, is never masked: holding back output would break interactive programs.
- A secret shorter than 4 bytes is not masked.
- The gateway checks whole lines and holds an unfinished line up to 4 KiB. In a longer line, a match split at a 4 KiB boundary is not masked.
- A `BEGIN … PRIVATE KEY` line without a matching `END` hides the rest of that command's output.
- The recording keeps the original output: masking protects the user's screen and the tools that read it, not the replay.

## Testing a command

The test box on **Policies**, **Command rules** (`PolicyService` `TestCommand`, owners and admins) takes a user, a resource and a command and shows its classes and the decision now: allowed or denied, the rules that decided, whether an approval would be asked and the message. It uses the same class overrides, rules and custom policies as the gateways and does not run anything.
