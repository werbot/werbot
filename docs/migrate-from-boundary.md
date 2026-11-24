# Migrate from HashiCorp Boundary

This page maps the objects of Boundary Community Edition to Werbot. Werbot has no import tool. You create the objects in Werbot by hand or with the API.

## Mapping

| Boundary | Werbot | Notes |
| --- | --- | --- |
| Org and project scope | Organization | Werbot has one level: the organization. Use labels instead of project scopes. |
| Host catalog and host set | Resource and labels | One host is one resource. A label such as `env=prod` replaces a host set. |
| Target | Resource plus binding | The resource holds the type, address and port. A binding grants a user or a team the resource, or every resource with some labels. |
| Target port | Resource ports | `tcp` and `udp` resources take 1 to 16 non-overlapping port ranges. A binding can narrow them. |
| Role and grant | Binding and approval policy | A binding is standing access. An approval policy lets people ask for time-limited access ([jit.md](jit.md)). |
| User, group and managed group | User, team, claims mapping | An IdP group maps to a team or a role with [claims mapping](sso.md#claims-mapping). |
| Auth method (password, OIDC) | Password, OIDC, MFA | See [sso.md](sso.md) and [mfa.md](mfa.md). |
| Credential store and credential | Resource credential | Werbot encrypts the credential with `WERBOT_SECRET_KEY` and never shows it in the UI or the API. If a target prints it, output masking hides it from agents by default ([policies.md](policies.md#output-masking)). |
| Worker | Gateway | A gateway joins a pool with a join token and holds one outbound mTLS connection ([gateway.md](gateway.md)). |
| `boundary authenticate` | `werbot login` | [cli.md](cli.md#log-in). |
| `boundary connect` | `werbot connect` | `werbot connect --vnet` gives each resource a name ([vnet.md](vnet.md)). |
| `boundary connect ssh` | `werbot ssh` | OpenSSH through `werbot ssh-config` also works ([cli.md](cli.md#commands)). |

## What you move by hand

1. Create the organization and invite the users, or set up [SSO](sso.md#set-up-an-idp).
2. Install a gateway in each network that holds targets ([gateway.md](gateway.md)).
3. Create one resource per host, with the credential and labels.
4. Create teams. Create bindings from the Boundary roles and grants.
5. Create approval policies for the access that people should request.
6. Change your scripts and runbooks: `werbot login`, then `werbot ssh` or `werbot connect`.

Session history of Boundary stays in Boundary. Keep Boundary read-only for as long as your audit rules need.

## What Werbot has and Boundary Community Edition does not

- Session recording with a verified hash chain and a signed manifest ([security.md](security.md)).
- [Just-in-time access](jit.md): requests, approvals, break-glass and locks.
- [Command policies](policies.md): risk classes, command rules, folder rules and output masking.
- [AI agents](agents.md): a sponsor, a baseline, limits per run and a kill switch.
- A [web terminal](web-terminal.md) with a live view and Terminate.

## What Boundary has and Werbot does not

- Several nested scopes (global, org, project). Werbot has one organization level.
