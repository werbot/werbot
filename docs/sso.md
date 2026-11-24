# Single sign-on (OpenID Connect)

Members sign in to the web UI with an identity provider (IdP) that speaks OpenID Connect: Keycloak, Okta, Google Workspace, Microsoft Entra ID and others. The IdP vouches for the user. Werbot keeps its own account, role, teams, sessions and audit log.

Owners and admins manage IdPs on **Settings, Single sign-on** (`/settings/sso`, `IdentityProviderService`). Every create, change and delete asks for a [fresh second factor](mfa.md#fresh-mfa-for-admin-actions).

## Set up an IdP

1. In the IdP, create a confidential OIDC client (authorization code flow with PKCE).
2. Set the redirect URI to `<WERBOT_WEB_URL>/api/oidc/callback`, for example `https://werbot.example.com/api/oidc/callback`.
3. In Werbot, add an IdP with a name (1 to 63 characters), the issuer URL, the client id and the client secret. The secret is stored encrypted and never shown again. A change of the issuer or the client id needs the client secret again; an update without it is refused.
4. Enable it. The login page then shows a button with the name of the IdP.

The issuer is the URL whose `/.well-known/openid-configuration` the IdP serves.

| IdP | Issuer | Notes |
| --- | --- | --- |
| Keycloak | `https://<host>/realms/<realm>` | Tested (Keycloak 26, e2e). Client authentication on, standard flow on. Groups come from a group membership mapper on the client with the claim name `groups` (full group path off: `ops`; on: `/ops`); Keycloak has no `groups` scope. For the [hourly check](#hourly-check-and-cli-renewal) set the scopes to `openid email profile offline_access`, add `offline_access` to the client's optional client scopes and give users the realm role `offline_access` (it is in the realm's default roles unless removed). |
| Okta | `https://<org>.okta.com` (or a custom authorization server URL) | Not tested. App type Web, authorization code, refresh token grant on. Add a `groups` claim to the ID token (filter by regex or prefix). Okta knows the scopes `groups` and `offline_access`; add both for groups and the hourly check. |
| Google Workspace | `https://accounts.google.com` | Not tested. Keep the default scopes: Google lists only `openid`, `email` and `profile` in its discovery document. Google sends no group claim: use the `email` or `hd` claim in the mapping. Google sets `email_verified`. Google issues a refresh token only for `access_type=offline`, which Werbot does not send: Google users are [never rechecked](#no-refresh-token-no-check). |
| Microsoft Entra ID | `https://login.microsoftonline.com/<tenant>/v2.0` | Not tested. Entra ID does not send `email_verified`: its users are never linked or created (see [Linking](#provisioning-and-linking)). Group claims carry object ids and are set up in the app registration, not by a scope. Add `offline_access` for a refresh token. |

### Scopes

Default scopes: `openid email profile`. A list you set replaces the default and must contain `openid`. An IdP may refuse a scope it does not know: Keycloak answers `invalid_scope` and the login fails. Add only scopes the IdP lists. Add `offline_access` when the IdP issues a long-lived refresh token for it (see [Hourly check](#hourly-check-and-cli-renewal)).

### The issuer is admin input

The Werbot server fetches the issuer URL (discovery, token and userinfo endpoints). Only owners and admins can set it. The issuer must use `https://`; `http://` is accepted only when `WERBOT_ENV=development`. The issuer URL may carry no query and no fragment.

The IdP list on the login page is public: it shows the `id` and `name` of every enabled IdP of the installation, before any sign-in.

## Claims mapping

The mapping (`claims_mapping` in the API) is a list of rules stored with the IdP. In the web UI, open it from the row menu of the IdP on **Settings, Single sign-on** (**Claim mapping**); the IdP drawer holds the connection fields only.

A mapping rule reads one claim of the ID token (the userinfo claims are merged over it) and gives a team or a role.

| Field | Meaning |
| --- | --- |
| `claim` | Claim name, for example `groups`, `email`, `hd`. |
| `regex` | Regular expression, anchored at both ends: `ops` matches only `ops`, not `devops`. A claim may be a string or a list of strings; one match is enough. A regex that does not compile matches nothing (the editor refuses it). |
| `team` (`team_id`) | The team the user belongs to when the rule matches. |
| `role` | `admin` or `member`. `owner` is never granted. |

A rule has exactly one of `team` and `role`. An IdP takes up to 50 rules.

Examples:

| Claim | Regex | Result |
| --- | --- | --- |
| `groups` | `/ops` | Member of team `ops` while the claim holds `/ops`. |
| `groups` | `/admins` | Role `admin`. |
| `groups` | `/eng/.*` | Team `engineering` for every subgroup of `/eng`. |
| `hd` | `example\.com` | Role `member`. |

Every login applies the rules:

- A team rule that matches adds the user to the team. A team rule that does not match removes the user from that team, also when someone added the user by hand. Teams without a rule are never changed.
- With role rules, the highest matching role wins (`admin` over `member`). When role rules exist and none matches, the role is `member`: a user who left the admin group loses `admin` at the next login.
- Without role rules the role stays as it is.
- An `owner` keeps the role; no rule changes it.
- A role or team change is audited with the login (`auth.login`, method `oidc`).

## Provisioning and linking

The callback links an IdP identity to a Werbot member in this order:

1. The identity (IdP and `sub`) is already linked: the linked member signs in.
2. The ID token has `email_verified: true` and the email matches a member of the organization: Werbot links them and signs the member in.
3. The IdP has `allow_signup` on (**Create a member on first sign-in**), the email is verified and no member has it: Werbot creates a member with the role `member` and a login made from the local part of the email.
4. Anything else: the login fails with `Sign-in with the identity provider failed.`

Only a verified email links or creates an account. An IdP that omits the `email_verified` claim, such as Entra ID, never links by email and never provisions; users of such an IdP need an account that is linked another way, or a different IdP setup that returns the claim. A user already linked keeps signing in.

A member created by signup has no password (see [Passwords](#no-password-for-idp-users)).

## What the IdP decides, and what Werbot still asks

Werbot asks no second factor after an IdP login. MFA at the IdP is the IdP's job.

Some Werbot actions still ask for a Werbot factor (TOTP or a passkey) even for an SSO user, so such a user enrolls one under **Profile, MFA**:

- [admin actions](mfa.md#fresh-mfa-for-admin-actions) (fresh MFA);
- [MFA rules](mfa.md#mfa-rules) of SSH connections and web terminals;
- the organization setting `require_mfa` ([enrollment](mfa.md#requiring-mfa)).

## No password for IdP users

A user created by an IdP has no password. `Login` answers as for a wrong password, a password reset request sends nothing, and `ChangePassword` fails with `FailedPrecondition`. The only way in is the IdP. So disabling the user at the IdP cannot be bypassed with a password reset by email.

## Hourly check and CLI renewal

Werbot keeps the IdP refresh token of each linked user (sealed with `WERBOT_SECRET_KEY`). The job `idp.recheck` runs every 10 minutes and takes up to 100 links with a refresh token that were not checked for 50 minutes. It marks each link as checked before it asks the IdP, so a link is asked by one caller at a time:

- The IdP accepts the token: the check passes and a new refresh token, when issued, replaces the old one.
- The IdP answers `invalid_grant` (user disabled or deleted, session ended, token revoked): Werbot ends all cookie sessions of the user, revokes the CLI sessions with the reason `idp`, drops the token and writes the audit event `idp.recheck.revoke`.
- The IdP is unreachable (network error, timeout, 5xx): nothing changes. Sessions stay, and the link is asked again 50 minutes later. An unreachable IdP does not hold back the links of other IdPs.

The same check runs when a CLI session renews its token ([cli.md](cli.md)) and refreshes only links not checked for 50 minutes. If the IdP refuses, the renewal fails and the sessions of the user are revoked. If the IdP is unreachable, the renewal goes on.

Worst case: a user disabled at the IdP keeps access for up to about 1 hour (the 50 minutes of the check interval plus the 10 minutes between job runs), or until the next CLI renewal that finds the link stale. Short-lived CLI certificates (1 hour) end the rest. To cut access at once, lock the user ([jit.md](jit.md#locks)).

### No refresh token, no check

Werbot checks only links that have a refresh token. When the IdP sends none at login (Google without `access_type=offline`, an IdP without `offline_access` in the scopes or without the refresh token grant), the user is never rechecked. Disabling such a user at the IdP blocks new SSO logins only. The running sessions stay until they expire: a cookie session 7 days after its last use (each use extends it), a CLI session after `cli_session_ttl` idle or `cli_session_max` (default 30 days). To end access at once, lock the user ([jit.md](jit.md#locks)) or remove the member.

### The IdP must keep the refresh token alive

The check needs a refresh token that still works about an hour after the login. If the IdP expires it sooner, the next check gets `invalid_grant` and Werbot revokes the sessions of every SSO user about an hour after login. To avoid this:

- Request the scope `offline_access` (add it to the scopes of the IdP in Werbot, and allow it for the client at the IdP), or
- set the IdP session idle time to at least 1 hour.

Keycloak: the default SSO Session Idle is 30 minutes, shorter than the 50-minute check interval. Without `offline_access` the refresh token expires and sessions are revoked about an hour after login. Use the scopes `openid email profile offline_access` with the client and role setup of the [table above](#set-up-an-idp) (tested), or raise **SSO Session Idle** of the realm to 1 hour or more. An offline token outlives the SSO session, but not a **Sign out** of the user by a Keycloak admin: it makes the user's earlier tokens stale, offline ones included, so the next check revokes the user's Werbot sessions.

Some IdPs issue a refresh token that can be used once. The job and a CLI renewal never ask with the same link at the same time, so they do not race for it.

## IdP unavailable

Discovery runs on every login and callback; Werbot does not cache it. When the IdP is down:

- A new SSO login fails and the browser returns to the login page with `Sign-in with the identity provider failed.`
- Running sessions are not revoked: the hourly check treats an unreachable IdP as no answer.
- Password login of members who have a password still works.

See [failure-modes.md](failure-modes.md).

## Login from the CLI

`werbot login` opens the authorization page of the web UI in the browser ([cli.md](cli.md)). Without a web session the page sends the user to the login page, where the IdP button works as usual, and returns to the authorization page afterwards. The CLI session is then checked at every renewal as described above.

## Audit

| Event | When |
| --- | --- |
| `auth.login` | Sign-in; data has `method: oidc`, `idp_id` and, when the role changed, `role`. |
| `idp.recheck.revoke` | The IdP refused a user's refresh token and the sessions were revoked. |
