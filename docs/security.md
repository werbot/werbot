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
