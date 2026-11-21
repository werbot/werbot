# Just-in-time access

Werbot can give access for a limited time instead of a permanent binding. A member asks for a resource, an approver decides, and the gateway cuts the session when the grant ends. This guide covers access requests, approval policies, break-glass, locks and the organization settings that bound them.

## Access requests

A member asks for access in the web UI (**Requests**, **New request**) or with [`werbot request`](cli.md#access-requests). A request names:

- **What**: one resource, or a label selector (`env=prod,team=db`) that covers every resource carrying those labels. A selector request needs a policy that covers every resource it matches.
- **Why**: a reason, up to 1024 characters. A policy can require it; break-glass always does.
- **How long**: a duration. Without one the organization default `grant_default_ttl` applies. A duration above the policy `max_duration` or the organization `grant_max_ttl` is refused (`duration exceeds <limit>`).

A request is possible only when an approval policy matches it. `werbot ls` and the API (`ListAccessibleResources`) mark such resources: `ACCESS` is `yes` for resources you can open now and `request` for resources you can ask for. `werbot ssh`, `ssh-config` and `connect` use only the resources you can open.

A request is `pending` until an approver decides or it expires. The requester can cancel it while it is pending. An approved request creates one grant per resource; a grant ends at its `expires_at`, on revoke or under a lock. Every step is audited: `access.request.create`, `access.request.approve`, `access.request.deny`, `access.request.cancel`, `approval.expire`, `access.grant.create` and `access.grant.revoke`.

Emails:

- `access_request` goes to every possible approver of a new request, with a link to `<WERBOT_WEB_URL>/requests?id=<id>`.
- `access_decision` goes to the requester when a request is approved or denied.
- `break_glass` goes to every owner and admin when someone uses break-glass.

## Approval policies

Owners and admins manage approval policies on **Policies**, **Approvals** (`ApprovalPolicyService`). A policy has:

| Field | Meaning |
| --- | --- |
| `name` | Unique in the organization, 1 to 63 characters. Policies are applied in name order. |
| `resource_selector` | Exactly one of: up to 100 resource ids, or 1 to 8 labels (`key=value`; a resource must carry every pair). |
| `requesters` | Who may ask: users, teams, `all_users`, `all_agents`. At least one. |
| `approvers` | Who decides: users and teams. Empty means every owner and admin. |
| `mode` | `manual` (an approver decides) or `auto` (approved at once). |
| `max_duration` | Longest grant, 1 minute to 14 days, default 8 hours. |
| `require_reason` | The request must give a reason. |
| `four_eyes` | The sponsor of an agent never approves that agent's commands. |
| `break_glass` | The policy allows break-glass requests. |

When several policies match one request:

- The request is approved automatically only when **every** matching policy is `auto`. One `manual` policy makes the request manual.
- The approvers are the union of the approvers of the manual policies; a manual policy without approvers adds the owners and admins.
- The shortest `max_duration` applies; `require_reason` and `four_eyes` apply when any matching policy sets them.

Rules that no policy changes:

- **No self-approval.** Nobody approves or denies their own access request.
- **Nobody to ask.** When no one but the requester could approve (for example, the only owner asks for access), the request is refused with `no one can approve this request` and nothing is stored.
- **Decision time.** A decision checks the policies again against the current resources. Owners and admins may deny any request.
- **Timeout is a denial.** A pending request past its deadline (`request_pending_ttl`) becomes `expired` and never turns into an approval. The `approval.expire` job (River `maintenance` queue, every minute) marks such requests; an approval that reaches the server after the deadline fails.
- **Commands.** A command approval (an `approval` command rule or custom policy, [policies.md](policies.md#command-approvals)) is never approved automatically. Without a matching policy owners and admins decide. It waits `command_approval_ttl`.

## Organization settings

Owners and admins set these on **Settings**. Durations are typed as text (`30s`, `15m`, `2h`, `14d`).

| Setting | Default | Range | Meaning |
| --- | --- | --- | --- |
| `grant_default_ttl` | `8h` | above 0, at most `grant_max_ttl` | Grant length when a request names none. |
| `grant_max_ttl` | `14d` | `1m` to `14d` | Longest grant, whatever the policy says. |
| `request_pending_ttl` | `1h` | `1m` to `24h` | How long a request waits for a decision. |
| `command_approval_ttl` | `5m` | `30s` to `1h` | How long a command approval waits for a decision. |
| `lock_mode` | `best_effort` | `best_effort`, `strict` | What gateways do while their tunnel to the server is down ([lock mode](#lock-mode)). |
| `mask_output` | `agents` | `agents`, `all`, `none` | Whose exec output the gateway masks ([output masking](policies.md#output-masking)). |

## Break-glass

Break-glass is emergency access without waiting for an approver. It needs a policy with `break_glass` that covers the resource and lists the user as a requester.

- The reason is required; the grant lasts at most 1 hour (1 hour when no duration is given).
- The request is approved at once and audited as `access.break_glass`; owners and admins get the `break_glass` email.
- Sessions under a break-glass grant are recorded as usual, and their `session.start` audit event carries `data.break_glass = true`.
- The grant is marked `needs_review`. An owner or admin reviews it on **Requests** (`ReviewBreakGlass`, audited as `access.break_glass.review`). The grant owner never reviews their own break-glass.

## Grants

`ListGrants` shows grants: a member sees only their own, owners and admins see all. The grant owner, an owner or an admin can revoke a grant (`RevokeGrant`). A revoked or expired grant opens nothing.

## Locks

A lock refuses access at once, above every binding and grant. Owners and admins manage locks on **Locks** (`LockService`). Each lock has a target, a required reason and an optional expiry; without an expiry it stays until deleted.

| Target | Effect |
| --- | --- |
| `user` | Every gateway session of the user. API calls of the user are not blocked. |
| `team` | Every gateway session of the team's members. |
| `resource` | Every session to the resource. |
| `grant` | The grant opens nothing while the lock lasts. |
| `cli_session` | Connections made with the certificates of that CLI session, and new certificates for it (`cli session is locked`). Other logins of the same user keep working. |

The targets `agent`, `agent_run`, `agents_of_user`, `all_agents` and `mfa_device` are refused with `target type <t> is not supported yet`; they come with agents and MFA.

Locks are audited as `lock.create` and `lock.delete`.

### Revoking a CLI session

Revoking a CLI session (Profile, CLI sessions, or `logout`) refuses its next API request and its next gateway connection, but open connections made with its certificate keep running. To end them too, lock the CLI session on the **Locks** page or with `LockService` (`target_type = cli_session`). The CLI session id is the target of its `cli_session.revoke` event on the **Audit** page (hover the short id to see all of it). See [security.md](security.md#cli-sessions-and-user-certificates).

## Ending sessions

The gateway checks every open session again whenever its policy changes and when a grant or lock expires:

| Event | Session ends within | `end_reason` |
| --- | --- | --- |
| A grant expires | 1 s after `expires_at` | `access_revoked` |
| A grant is revoked | 2 s | `access_revoked` |
| A lock is created | 2 s | `access_revoked` |
| `strict` lock mode, tunnel down | 60 to 65 s after the tunnel loss (checked every 5 s) | `policy_stale` |

- Expiry uses a timer on the gateway, set from the last snapshot. It fires even when the server is unreachable.
- The gateway compares whole seconds: a grant ends at the second of its `expires_at`, never later.
- Expiry is measured by the gateway clock. Keep the gateway hosts on NTP ([gateway.md](gateway.md#lock-mode-and-clock)).

## Lock mode

Revokes and locks reach a gateway through its tunnel. While the tunnel is down, a gateway cannot learn about them:

- `best_effort` (default): open sessions keep running on the last snapshot until the tunnel is back. Grant expiry still ends sessions on time.
- `strict`: when the tunnel has been down for more than 60 seconds, the gateway ends every session with `policy_stale`. The gateway checks every 5 seconds, so sessions end 60 to 65 seconds after the loss. A reconnect resets the wait.

New sessions are refused while the tunnel is down in both modes ([failure-modes.md](failure-modes.md)).

## Command line

CLI: [cli.md](cli.md#access-requests).

## Enterprise

N-of-M approvals, multi-step approvals and periodic access reviews are Enterprise features.
