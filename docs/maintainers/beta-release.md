# Public beta release

Owner checklist for the public beta `v0.2.0`. Run the steps in order. Every step has a check. Nothing here is automated: tags and registry entries cannot be taken back.

State at the start:

- `main` is the v2 line since `v0.1.0`. The branch switch is done. Nothing is renamed or deleted here.
- v1 lives in the local branch `legacy-main`. Publishing a `legacy` branch is your decision.
- The beta work is on `phase-2` in the draft PR #449 (base `main`).
- `release.yml` accepts a tag only when its commit is in the history of `origin/main`. Merge first, tag second.

Tools: `gh`, `git`, `docker` with Compose v2, `go`, `bun`, `cosign`, `asciinema`, `mcp-publisher`.

## 1. CI on PR #449

- [ ] Open the PR and wait for all checks.

```sh
gh pr checks 449 --watch
```

Required green: `go`, `windows`, `web`, `fuzz`, `e2e`, `e2e-network`, `e2e-sso`, `e2e-cluster`, `redteam`, `release-snapshot` (jobs of `.github/workflows/ci.yml`) and CodeQL. CodeQL has no workflow file in the repo: check that the repository setting runs it on the PR.

Check: `gh pr checks 449` shows no `fail` and no `pending`.

## 2. Local gates

Run on the PR head. `make test`, `make e2e` and `make e2e-sso` need the dev database.

```sh
git fetch origin && git switch phase-2 && git pull --ff-only
make dev-db
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go mod tidy -diff
go mod tidy -modfile=tools/go.mod -diff
make lint
make test
make release-check
make fuzz
make e2e
make e2e-network
make e2e-sso
make e2e-cluster
make redteam
make site
```

Check: `govulncheck` prints `No vulnerabilities found.`; both `go mod tidy -diff` print nothing; every `make` target exits 0.

## 3. Manual checks

### 3.1 L5 on Linux

L5 needs a Linux host: Docker with Compose v2, Go 1.27.4. It reads RSS from `/proc`. Details: [test/load/README.md](../../test/load/README.md#l5-agents-and-relays).

```sh
ulimit -n 65536
make load LOAD_TESTS=L5
```

Check: every `RESULT | L5 | ...` line ends with `PASS` and the exit code is 0. Criteria:

- `AgentExec`: 0 failed calls, every call answered, one `session` row per call, RSS of gateways and replicas within budget in every sample.
- `Relays`: `werbot_relay_streams` equals the terminal count on both replicas, no terminal breaks, `werbot_relay_rejected_total{reason="limit"}` is 0, RSS within budget.

The default profile passed on 8 vCPU and 30 GiB RAM (test/load/README.md, "L5 stand and results"). On a 2 vCPU host the stand's own SSH target drops handshakes at this rate. If it fails on a host that is too small, use a smaller run and write it in the release notes: `make load LOAD_TESTS=L5 LOAD_ARGS='-load.agent-rate=200 -load.agent-duration=1m -load.relays=50'`.

Record the numbers (host, date, commit, `RESULT` lines) in the "Stand" and "Results" sections of `test/load/README.md`. You need them for the release notes.

### 3.2 `--vnet` on macOS

CI does not cover macOS. Follow [Manual check on macOS](../vnet.md#manual-check-on-macos), steps 1 to 5. Use a build of the PR head: `make build`, then `sudo bin/werbot connect --vnet`.

Check: step 4 leaves no `/etc/resolver/<org>.werbot` and no `100.64.0.1` in `ifconfig`; step 5 shows your user as owner of every profile file.

### 3.3 Demo run

Run the demo on an image built from the PR head. The published image does not exist yet. Details: [deploy/demo/README.md](../../deploy/demo/README.md).

```sh
docker build -f deploy/Dockerfile -t werbot:demo .
WERBOT_IMAGE=werbot:demo WERBOT_BIN="$PWD/bin/werbot" deploy/demo/demo.sh --ci
```

Check: the five steps print the results from the table in the README, and the script removes the stack (`docker compose -p werbot-demo ps` is empty).

## 4. Merge PR #449

- [ ] Take the PR out of draft and merge it into `main`. Use the merge method the branch protection allows.

```sh
gh pr ready 449
gh pr merge 449 --merge
```

Check:

```sh
git fetch origin
git merge-base --is-ancestor phase-2 origin/main && echo merged
```

Expected: `merged` (a squash merge prints nothing: check `gh pr view 449 --json state --jq .state` is `MERGED`, then use the merge commit in step 5). CI on `main` must be green before you tag: `gh run list --branch main --workflow ci --limit 1`.

## 5. Tag `v0.2.0`

`server.json` and `deploy/demo/demo.sh` already name `v0.2.0`. `release.yml` runs on `v0.*` tags and marks the release as a pre-release (`.goreleaser.yaml`, `release.prerelease`).

```sh
git switch main && git pull --ff-only
git tag -a v0.2.0 -m "Public beta"
git push origin v0.2.0
```

Check: `git merge-base --is-ancestor v0.2.0 origin/main && echo ok` prints `ok`.

Never move or delete a pushed tag. A broken release gets a new patch tag (`v0.2.1`), see [Rollback](#rollback).

## 6. Watch `release.yml`

```sh
gh run watch "$(gh run list --workflow release --limit 1 --json databaseId --jq '.[0].databaseId')"
```

Jobs: `web`, `release` (govulncheck, GoReleaser, cosign, SBOM, image push), `smoke` (signature checks, compose quick start, Playwright).

Check: all three jobs are green. `gh release view v0.2.0 --json isPrerelease,assets --jq '.isPrerelease, (.assets | length)'` prints `true` and the asset count (6 archives, 6 SBOM files, `checksums.txt`, `checksums.txt.sigstore.json`: 14).

## 7. Verify the release

`smoke` checks the same things, but check by hand from a clean directory. Commands: [docs/security.md](../security.md#verifying-releases). Replace the tag with `v0.2.0`.

```sh
mkdir -p /tmp/werbot-rel && cd /tmp/werbot-rel
gh release download v0.2.0 -R werbot/werbot -p checksums.txt -p checksums.txt.sigstore.json -p 'werbot_0.2.0_linux_amd64.tar.gz' -p 'werbot_0.2.0_linux_amd64.tar.gz.sbom.json'
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/werbot/werbot/\.github/workflows/release\.yml@refs/tags/v0\.' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt   # macOS: shasum -a 256 --check --ignore-missing checksums.txt
cosign verify \
  --certificate-identity-regexp '^https://github\.com/werbot/werbot/\.github/workflows/release\.yml@refs/tags/v0\.' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/werbot/werbot:v0.2.0
docker buildx imagetools inspect ghcr.io/werbot/werbot:v0.2.0 --format '{{ json .SBOM }}'
docker run --rm ghcr.io/werbot/werbot:v0.2.0 version
```

Check: `Verified OK`, `werbot_0.2.0_linux_amd64.tar.gz: OK`, `The cosign claims were validated`, a non-empty SBOM JSON, and `version` prints `werbot 0.2.0 (commit ...`.

The registry needs the image label that proves ownership:

```sh
docker pull ghcr.io/werbot/werbot:v0.2.0
docker inspect ghcr.io/werbot/werbot:v0.2.0 --format '{{ index .Config.Labels "io.modelcontextprotocol.server.name" }}'
```

Expected: `io.github.werbot/werbot`. Check that the package `ghcr.io/werbot/werbot` is public: a pull without `docker login` must work.

## 8. Record `demo.cast`

Record on the published image, as in [Record `demo.cast`](../../deploy/demo/README.md#record-democast).

```sh
cd deploy/demo
asciinema rec --idle-time-limit 2 --command "WERBOT_VERSION=v0.2.0 ./demo.sh" demo.cast
```

Check: play it with `asciinema play demo.cast`. It shows no data from your machine except throwaway passwords. Commit `deploy/demo/demo.cast` to `main` through a PR.

## 9. Publish to the MCP Registry

Only after step 7: the registry needs the image to exist. `server.json` names `ghcr.io/werbot/werbot:v0.2.0` and version `0.2.0`.

```sh
mcp-publisher login github
mcp-publisher validate
mcp-publisher publish
```

Run the commands in the directory of `server.json` (the repo root). The namespace `io.github.werbot` is confirmed by the GitHub login: log in as a member of the `werbot` organization. Run `mcp-publisher --help` if your version has other subcommand names.

Check: `curl -fsS 'https://registry.modelcontextprotocol.io/v0.1/servers?search=werbot'` lists `io.github.werbot/werbot` version `0.2.0`.

## 10. Deploy the docs site

The site is deployed by hand (owner decision): no workflow. Build it with links that point at the tag, then upload `site/dist` to https://docs.werbot.com with your hosting tool. The README on `main` links to https://docs.werbot.com from the merge (step 4) on, so do this step on the same day as the tag.

```sh
git switch main && git pull --ff-only
make site SITE_REF=v0.2.0
```

Check: `site/dist/index.html` exists. After the upload, `curl -fsSI https://docs.werbot.com/` returns `200` and the page of `agents` and `security` open.

## 11. Release notes

Edit the GitHub release (it stays a pre-release): `gh release edit v0.2.0 --notes-file notes.md`. Put these in the notes:

- First line: `Public beta`.
- Links: [docs/agents.md](../agents.md), [docs/jit.md](../jit.md), [docs/upgrading.md](../upgrading.md), [docs/migrate-from-boundary.md](../migrate-from-boundary.md), `deploy/demo/`, https://docs.werbot.com.
- Breaking changes: copy them from [docs/upgrading.md](../upgrading.md). An upgrade needs a full stop in `v0.x`.
- Known limits:
  - The exec decision (analysis, path rules, `Decide`) holds p99 ≤ 5 ms at 1 000 bindings only. At 10 000 bindings p99 is 8.5 to 11.4 ms (development machine). Policies per resource are planned for phase 3. See [test/load/README.md](../../test/load/README.md#exec-decision-at-10-000-bindings-known-limit) and [scaling.md](../scaling.md#resources).
  - The SDK extraction is deferred to the connectors track.
  - L5 numbers: the `RESULT` lines of step 3.1 and the host they ran on.
  - The MCP Registry package is an OCI image with documented limits: uid 65532, and the gateway address must be reachable from inside the container.
  - A rolling upgrade is not supported.
- Verification: link [docs/security.md](../security.md#verifying-releases).

Announce: README and the docs site link to the release.

## Rollback

- Never move, delete or re-push a tag. Never overwrite an image tag.
- A bad release: fix on `main` through a PR, tag `v0.2.1`, repeat steps 5 to 11 for it.
- Mark the bad release as a draft or add a warning at the top of its notes: `gh release edit v0.2.0 --notes-file notes.md`.
- A bad registry entry: publish a new version with a new `server.json`. Do not edit the published one.
- A bad site deploy: build and upload the previous ref again, `make site SITE_REF=<previous tag>`.
