# Multi-factor authentication

Werbot asks a second factor in four places:

- **Web login** of a user who has a factor.
- **SSH and `werbot connect`**, when an [MFA rule](#mfa-rules) covers the resource.
- **Web terminal**, under the same rules.
- **Admin actions** (fresh MFA) in the web UI and the CLI.

Agents never meet a second factor: an agent token, an agent run and an agent profile cannot enroll, confirm or step up, and an MFA rule never applies to them ([agents.md](agents.md)). A command that needs MFA is refused for an agent with the normal denial text.

## Factors

Users manage their factors on **Profile, MFA** (`/profile/mfa`).

| Factor | Use |
| --- | --- |
| TOTP | A six-digit code from an authenticator app (30-second steps, one step of clock skew allowed). A code works once: the same code is refused a second time. |
| Passkey (WebAuthn) | A passkey or a security key in the browser. Several may be enrolled. |
| Recovery codes | Ten one-time codes, shown once when generated. They work for the web login only. They do not step up admin actions and never meet an SSH rule. |

Adding, removing or regenerating factors needs a [fresh second factor](#fresh-mfa-for-admin-actions) once the user has one. The first enrollment does not.

A password login of a user with a factor answers with a `mfa_token` instead of a session. The token lives 5 minutes and finishes with a passkey, a TOTP code or a recovery code. The IdP login of [SSO](sso.md) asks no Werbot factor.

### WebAuthn needs a domain name

Passkeys need HTTPS or `localhost`, and a domain name in `WERBOT_WEB_URL`. When the host of `WERBOT_WEB_URL` is an IP address, passkeys are off: the server logs a warning, `Methods` never list `webauthn`, and passkey calls answer `passkeys need a domain name in WERBOT_WEB_URL`. TOTP works.

A user who has only passkeys, no TOTP and no recovery codes, cannot sign in on such a server: the login fails closed. Enroll TOTP or generate recovery codes first, or serve the UI on a domain name.

Browsers: current Chrome, Edge, Firefox and Safari (the UI parses WebAuthn JSON with the browser's own functions). Playwright tests cover Chromium only.

## Requiring MFA

The organization setting `require_mfa` (**Settings**, owners and admins) takes:

| Value | Who must have a factor |
| --- | --- |
| `off` | Nobody (default). |
| `admins` | Owners and admins. |
| `all` | Every member. |

A user who must have a factor and has none can reach only the sign-in, MFA, system and profile calls until they enroll one: other API calls answer `mfa enrollment required` (403), and the web UI leads to `/profile/mfa`. The first enrollment does not set a fresh MFA, so the first admin action right after it asks a step-up (a TOTP code from the same app must be a new one, which takes up to 30 seconds).

## MFA rules

An MFA rule asks a second factor before a person connects to a resource. Owners and admins manage rules on **Policies, MFA** (`/policies/mfa`, `PolicyService`).

| Field | Meaning |
| --- | --- |
| Subjects | Users, teams or everyone. Not agents. |
| Resources | A selector, as for other policies. |
| `max_age` | How young the factor must be: 30 seconds to 24 hours, whole seconds. |
| `strength` | `any` (TOTP or passkey) or `webauthn` (passkey only). |

When several rules cover a connection, all must be met.

A factor is recorded for the key or the certificate the user connected with (`key:` or `cert:` and the fingerprint). Another connection with the same key within `max_age` needs no new factor (the reuse window). A passkey confirmation counts as `webauthn`, a TOTP code as `any`. A TOTP code never meets a `webauthn` rule.

## SSH

When a rule asks for a factor, the gateway answers the key with a second step. The client sees this prompt, with `<url>` the confirmation page and `<code>` a code that the page shows too:

```
Open <url> (code <code>): 
```

For a rule of strength `any` the prompt also offers TOTP:

```
Open <url> (code <code>) or enter TOTP: 
```

The user either:

1. opens the page, checks that the code matches, confirms with a passkey, and presses Enter (an empty answer), or
2. types a six-digit TOTP code.

Another answer asks again. The wait ends after 60 seconds with `Permission denied (MFA timeout)`. After too many wrong codes the user reads `Too many MFA attempts, try again later`.

- The gateway reads the challenge status every 2 seconds, and a confirmation also wakes it at once through the server stream ([failure-modes.md](failure-modes.md)).
- The page `<WERBOT_WEB_URL>/mfa/<id>` shows the resource, the gateway, the client IP and the code; only the user of the challenge can confirm it. Another user's challenge answers `not found`.
- With a menu login (`ssh user@gateway`) and with a shell the question is asked on the terminal. An exec command without a terminal prints the notice and waits for the browser confirmation only; it cannot read a TOTP code.
- The factor counts for every channel of the same connection until it closes (a second shell, an exec or SFTP over one connection).
- A port forward (`ssh -L`) cannot ask: without a factor passed at login it is refused with `MFA required, reconnect to confirm`.
- Agents are never asked ([agents.md](agents.md)).

### OpenSSH, `werbot ssh`, `werbot connect`

- **Plain OpenSSH** (also `ssh-config`): the prompt shows in the terminal. Open the URL yourself.
- **`werbot ssh`** starts `ssh` with `SSH_ASKPASS` set to the `werbot` binary and `SSH_ASKPASS_REQUIRE=force`. The helper opens the browser, prints `Opened <url> (code <code>), waiting for confirmation`, and reads the TOTP code from the terminal when you type one. Any other question is shown and read as is; the helper never answers by itself. `SSH_ASKPASS_REQUIRE` needs OpenSSH 8.4 or later.
- **`werbot connect`** answers the same prompt itself, in-process.
- **`werbot agent run`** removes `SSH_ASKPASS` and `SSH_ASKPASS_REQUIRE` from the environment of the agent.

| Client | MFA through the prompt |
| --- | --- |
| OpenSSH 8.4 or later (Linux, macOS) | `werbot ssh` works with the browser or a typed code. |
| OpenSSH before 8.4 | Ignores `SSH_ASKPASS_REQUIRE`: the prompt appears on the terminal and the user opens the URL by hand. |
| Windows OpenSSH | Support for the helper varies. Use the prompt on the terminal, or `werbot connect`. |
| Other clients | Must support keyboard-interactive and show the prompt. |

The certificate of a CLI session lives 1 hour and renews 10 minutes before its end. A factor is recorded for the certificate (`cert:`), so the reuse window of `werbot ssh` ends with the certificate: at most about 50 minutes, whatever `max_age` says. After a renewal the next connection asks again.

## Web terminal

The web terminal asks no prompt. When an MFA rule covers the resource and the session of the browser has no factor of the rule's age and strength, the WebSocket closes with `4001` (`mfa_required`) ([web-terminal.md](web-terminal.md#close-codes)). The UI then opens a step-up dialog (a passkey first, when the user has one), and the user opens the terminal again. A TOTP step-up does not meet a rule of strength `webauthn`.

A web terminal opened with a Bearer token (not a browser cookie) has no browser session to bind a factor to: under an MFA rule it always gets `4001`.

## Fresh MFA for admin actions

Admin actions need a second factor passed within the last 5 minutes. Without it the call is refused with `mfa required` and the UI shows a step-up dialog (`StepUpDialog`) before it repeats the call. The check applies when the user has a factor, or when `require_mfa` asks one of the user's role.

The guarded actions:

- access requests: approve, deny, revoke a grant, review a break-glass access; a break-glass request itself;
- approval policies, locks, bindings: create, update, delete;
- command, class, folder, terminal, custom Cedar and MFA rules, resource rules: create, update, delete;
- gateways and join tokens: create, delete;
- members: invite, change role, remove; team members: add, remove;
- resource credentials: set, clear, clear the host key;
- organization settings;
- identity providers: create, update, delete ([sso.md](sso.md)).

Changing your own factors also needs a fresh factor once you have one.

Step-up in the web UI uses a passkey or a TOTP code. A recovery code does not step up. In the CLI, `werbot request --break-glass`, `werbot approve` and `werbot deny` step up on demand: the CLI prints the prompt

```
Open <url> (code <code>) or enter TOTP: 
```

and the user confirms with a passkey in the browser and presses Enter, or types a TOTP code. A user with only TOTP gets `Enter TOTP: `. The page must be on the host of the server of the profile, or the CLI refuses it. The step-up is recorded for the CLI session and counts for 5 minutes. An agent token cannot step up.

## Locking a passkey

A lock with the target `mfa_device` and the id of a passkey refuses that passkey for every MFA step (login, step-up and challenge confirmation) until the lock ends. Create it on **Locks** ([jit.md](jit.md#locks)); the target must be a passkey of a member of the organization.

## Rate limits

Counters live in PostgreSQL, so every replica shares them. A refused call answers `ResourceExhausted` (the UI shows `Too many requests, try again later`). When the database is unreachable, the limit check fails and the login is refused, never allowed.

| Limit | Key | Window |
| --- | --- | --- |
| Login: 100 attempts | client IP | 1 minute |
| Login: 10 attempts | client IP and email | 1 minute |
| Password reset: 3 requests | email | 1 hour |
| TOTP and recovery code: 5 attempts, **successful ones too** | user | 5 minutes |
| `POST /oauth/token`: 60 requests | client IP | 1 minute; answers 429 with `Retry-After: 60` |
| Device flow poll faster than 5 seconds | device code | the token endpoint answers `slow_down` |

Notes:

- The MFA counter counts successes. A user who is busy (many connects, each with a TOTP code) can lock their own MFA for up to 5 minutes. A passkey confirmation does not count.
- Without a trusted proxy the server cannot see client IPs: all users share one IP bucket. Set `WERBOT_TRUSTED_PROXIES` ([configuration.md](configuration.md#behind-a-reverse-proxy)).
- Counter keys contain email addresses and client IPs. A job deletes keys older than 2 hours.
- Agents use the token endpoint too (token exchange): a shared NAT can reach 60 requests per minute.

## Audit

| Event | When |
| --- | --- |
| `auth.login` | A sign-in; data has the method (`password`, `oidc`). |
| `auth.mfa.fail` | A wrong code or assertion. |
| `lock.create`, `lock.delete` | A `mfa_device` lock. |
