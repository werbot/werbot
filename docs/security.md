# Security

## Reporting a vulnerability

Report privately through GitHub private vulnerability reporting (https://github.com/werbot/werbot/security/advisories/new) or by email to `security@werbot.com`. Do not open public issues for vulnerabilities. Fixes ship for the latest `v0.x` release.

## Verifying releases

Every release is signed keyless with cosign through GitHub OIDC. The release workflow builds the web UI in a separate job without `id-token`, so no bun or npm code runs in the job that signs.

Download `checksums.txt`, `checksums.txt.sigstore.json` and the archive for your platform from the release page, then:

```sh
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/werbot/werbot/\.github/workflows/release\.yml@refs/tags/v0\.' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt   # macOS: shasum -a 256 --check --ignore-missing checksums.txt
cosign verify \
  --certificate-identity-regexp '^https://github\.com/werbot/werbot/\.github/workflows/release\.yml@refs/tags/v0\.' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/werbot/werbot:v0.1.1
```

Expected output: `Verified OK`, `<archive>: OK` and `The cosign claims were validated`. To pin one release exactly, replace the regexp by `--certificate-identity https://github.com/werbot/werbot/.github/workflows/release.yml@refs/tags/v0.1.1`.

## SBOM

Each archive has an SPDX SBOM in the release assets: `*.tar.gz.sbom.json` and `*.zip.sbom.json`. The image SBOM:

```sh
docker buildx imagetools inspect ghcr.io/werbot/werbot:v0.1.1 --format '{{ json .SBOM }}'
```

## Threat model (summary)

- User traffic goes straight to a gateway. Its SSH port is the only thing open on the resource network.
- The server never connects to a gateway. Each gateway instance holds one outbound mTLS connection to the server.
- Credentials are issued per session and only for resources of that instance's pool. A gateway does not cache them: without the server, new sessions are refused.
- Recordings are protected by a hash chain and a manifest signed by the server.
- Secrets are encrypted with `WERBOT_SECRET_KEY` (AES-256-GCM). Only the server holds the key.
- Do not publish `/metrics` (`WERBOT_METRICS_ADDR`, `WERBOT_GATEWAY_HTTP_ADDR`) to the outside.
- A compromised server is outside the v0 threat model.
- A compromised gateway can flood the audit log with denials. Each instance reports at most 100 denials per 15s heartbeat, so it adds at most 100 `session.deny` rows per 15s; the rest only count in `werbot_gateway_denials_dropped_total`.
- v0.1.x has no per-IP limit on pre-auth SSH connections. Limit connections per source IP on the load balancer or firewall in front of the gateway SSH port.

## Gateway join tokens

A pool join token is reusable: `expires_in` from 1h to 8760h (default 24h) and `max_uses` from 1 to 1000 (default 10). A leaked token, before it is revoked, compromises one instance: a foreign instance receives the pool's host key and the resource credentials of the pool during user connections.

Mitigations:

- Keep `max_uses` and `expires_in` as small as the rollout allows.
- Check the instance list of the pool in the UI (hostname, IP, enrollment time).
- Revoke the token right after the rollout (Revoke).
- Delete an unknown instance (Delete instance): this closes its tunnel.
- Audit event `gateway.instance.enroll` carries the token id.

A wrong, expired, revoked and exhausted token give the same refusal. The system join token of `--with-gateway` has max uses 1.

## CLI sessions and user certificates

`werbot login` ([cli.md](cli.md)) talks to the OAuth authorization server built into the server (`/oauth/`, `/.well-known/`). Every login is a CLI session (`cli_session`), and the CLI reaches gateways with short-lived SSH user certificates.

- **Use https.** Tokens are issued with `iss = WERBOT_PUBLIC_URL`. The server accepts an `http://` `WERBOT_PUBLIC_URL` (the quick start uses one), but then access and refresh tokens, authorization codes and device codes travel in clear text between the CLI and the server. Production needs an `https://` `WERBOT_PUBLIC_URL`, and the CLI must log in with `--server https://…`.
- **Login flows.** The browser flow uses PKCE (`S256`) and redirects only to `http://127.0.0.1:<port>/callback` or `http://[::1]:<port>/callback`; any other `redirect_uri` gets `400` without a redirect. Consent and device approval need a browser session (cookie): a CLI token cannot approve another login.
- **Device code phishing.** The consent and device pages show the client, the hostname the CLI reported and the client IP. Approve only a login you just started; the hostname is not verified.
- **Tokens.** An access token is an ES256 JWT for 10 minutes; the signing key is sealed with `WERBOT_SECRET_KEY` and `/oauth/keys` publishes only the public key. A CLI token acts with the member's full role on the API, like the web session. Every API request checks that its CLI session is live, so a revoked session stops at the next request.
- **Refresh rotation.** Each refresh returns a new refresh token. Presenting a used one again revokes the whole CLI session (`cli_session.revoke`, reason `refresh_reuse`). A refresh whose response was lost and is retried with the old token ends the session the same way: run `werbot login` again.
- **Stored secrets.** Refresh tokens, authorization codes and device codes are stored only as SHA-256 hashes; they never appear in logs, audit data or error text.
- **Session lifetime.** An organization sets the idle timeout (`cli_session_ttl`, default 7 days) and the maximum age (`cli_session_max`, default 30 days) in Settings. Logout, revocation in the UI, a password reset or change (other CLI sessions) and removal from the organization end CLI sessions.
- **User CA.** Each organization has its own ed25519 user CA, sealed with `WERBOT_SECRET_KEY`. A gateway accepts only certificates signed by the CA of its pool's organization, with one principal (the login), inside their validity window and without critical options; the server then checks that the certificate's CLI session is live.
- **Certificates.** A certificate lives 1 hour and carries no MFA mark. There is no CRL: revoking a CLI session refuses the next gateway connection made with a still valid certificate, while open connections keep running. To end them too, lock the CLI session on the Locks page or with `LockService` (`target_type = cli_session`): its open connections close within 2 seconds, its certificates open nothing and it gets no new certificate ([jit.md](jit.md#locks)).
- **Profile on disk.** The CLI keeps tokens, its ed25519 key and the certificate in `WERBOT_HOME` with mode `0600` (directories `0700`) and does not use the OS keyring. Protect that directory like `~/.ssh`.

## Just-in-time access

Requests, approvals, break-glass and locks: [jit.md](jit.md).

- **Fail-closed.** A pending request never becomes an approval by timeout, and any policy evaluation error denies.
- **No self-approval.** Nobody decides their own access request; a request no one else can approve is refused.
- **Break-glass.** It needs a `break_glass` policy and a reason, lasts at most 1 hour, emails every owner and admin and stays marked `needs_review` until an owner or admin other than the grant owner reviews it.
- **Gateway clock.** A gateway ends grants and timed locks by its own clock. Keep gateway hosts on NTP: a slow clock keeps a session open past its grant.
- **Tunnel loss.** A revoke or lock reaches a gateway only over its tunnel. With `lock_mode = best_effort` (default) open sessions keep running while the tunnel is down; `strict` ends them with `policy_stale` 60 to 65 seconds after the loss (checked every 5 seconds).
- **User locks.** A `user` lock ends gateway sessions; it does not block the user's API calls. To stop those too, remove the member from the organization.
- **Command text.** Audit data and logs of command approvals never carry the command, only its kind, classes and reasons; the approval stores the command's SHA-256. The approvers see the command in the email and on the Requests page.
