# Upgrading

## Pre-1.0 policy

Between minor `v0.x` releases incompatible changes are possible. They are listed in the "Breaking changes" section of the release notes. Only the latest release is supported.

In `v0.x` an upgrade needs a full stop: every server replica and every gateway instance runs the same version, and a rolling upgrade is not supported (it comes with the public beta and expand/contract migrations).

## Release notes

### PostgreSQL connector

This release adds the `postgres` resource type ([databases.md](databases.md)). Check these changes before you upgrade:

- **Preset renamed.** `postgres` is now a built-in type name, so migration `00262` renames every resource preset named `postgres` to `postgres-<first 8 hex digits of its id>`. Resources keep their preset (the link is the id) and the display name does not change. Create a resource of the built-in `postgres` type for database proxying; the renamed preset keeps working as a `tcp` preset.
- **Migrations.** `00260` to `00262` add the query log recording format and the `session.queries` counter, the credential slot and the `client_cert` credential kind, and the preset rename. They run at server start as usual.
- **Gateways.** Upgrade standalone gateways with the server: older ones do not serve `postgres` resources.

### Web terminal, live view and terminate

This release adds a web terminal, a live view and Terminate ([web-terminal.md](web-terminal.md)). Check these changes before you upgrade:

- **New listener `:8444`.** Every server replica opens `WERBOT_INTERNAL_ADDR` (default `:8444`), an mTLS-only listener that replicas use to bridge browser streams. It accepts only client certificates of the internal CA. Keep it on the private network and open it between the replicas. With several replicas set `WERBOT_INTERNAL_ADDR` and `WERBOT_INTERNAL_URL` (an address the other replicas can reach); with one replica no setting is needed ([scaling.md](scaling.md#internal-listener)).
- **Load balancer.** The balancer in front of the server or `werbot web` must pass WebSocket upgrades and keep idle connections at least 60 seconds ([scaling.md](scaling.md#websocket-streams)). Without it the web terminal and the live view do not connect; nothing else is affected.
- **UI origin.** The server accepts a WebSocket only from the origin of `WERBOT_WEB_URL` (it defaults to `WERBOT_PUBLIC_URL`). Set `WERBOT_WEB_URL` to the address users open in the browser ([web.md](web.md#websocket)).
- **Gateways.** Upgrade standalone gateways with the server: older ones do not serve browser streams.
- **Migrations.** `00180` to `00182` add the server replica registry, stream tickets, and the session columns for the entry (`front`) and terminate requests. They run at server start as usual.
- **Dependency.** The server includes `github.com/coder/websocket` v1.8.15; the UI includes `@xterm/xterm` and `@xterm/addon-fit`.

### AI agents

This release adds AI agents ([agents.md](agents.md)). Check these changes before you upgrade:

- **Stricter command classes, for everyone.** Forms of programs that used to be `read` but write files or run programs now get a stricter class ([policies.md](policies.md#stricter-classes)):
  - `awk` is `opaque` when its program calls `system` or `getline`, contains `@` (gawk directives such as `@include` or `@load`), has `|` or `>` after `print` or `printf`, comes from `-f` or is not literal, or when an option other than `-F`/`-v` (`--field-separator`/`--assign`) is given.
  - `sed` is `opaque` with an `e`, `w` or `W` command or an `e` or `w` flag of `s`, with `-f`, with a script that does not parse and without a script.
  - `less -o`/`-O`/`--log-file`; `curl -o`/`-O`/`--output`/`--remote-name`/`--remote-name-all`/`-D`/`-c`/`--trace`/`--trace-ascii`/`--stderr`/`--libcurl`/`--etag-save`/`--hsts`/`--alt-svc`; `wget -O`/`-o`/`-a`/`-P` and their long forms are `write`. A bare `wget <url>` is `write`; it stays `read` only with `-O -`, `-O /dev/null` or `--spider`. The values `-` and `/dev/null` stay `read`.
  - `git diff`, `git log` and `git show` with `--ext-diff` or `--textconv` are `opaque`.
  - `less` with options the classifier does not parse or a non-literal first argument is `opaque`.

  Rules on `write` or `opaque` and the **Only allowed commands** preset therefore deny these forms or ask approval for them, where they ran as `read` before. Expect more refusals and approval requests for people under such rules; give a trusted script a [class override](policies.md#class-overrides) if needed.
- **Custom `permit` policies apply to agents.** A `permit (principal, …)` without a principal type now also matches AI agents and can widen what they may do. Review your custom policies and write `principal is Werbot::User` where a policy is meant for people only ([policies.md](policies.md#custom-cedar)).
- **`Approved` line.** After an approved command approval the gateway writes `Approved` on stderr (or the terminal) before the command runs. Scripts that read the stderr of a waiting exec see this extra line.
- **No exit status for a session the gateway ends.** A command or terminal that a lock, a revoke, a cancel, the idle timeout, the max duration or a gateway shutdown ends now closes without an exit status (OpenSSH exits with 255) instead of exit 0, for people too. Scripts that read such a cut-short command as a success now see a failure.
- **Locks on agents.** The lock targets `agent`, `agent_run`, `agents_of_user` and `all_agents` are now accepted ([jit.md](jit.md#locks)).
- **Migrations.** `00160` to `00164` add the agent tables, link sessions and approvals to agent runs and add agent profiles (`cli_session.agent_id`). They run at server start as usual.
- **New dependency.** The CLI includes the MCP Go SDK (`github.com/modelcontextprotocol/go-sdk` v1.8.0) for `werbot mcp`, with its modules `google/jsonschema-go`, `segmentio/asm`, `segmentio/encoding`, `yosida95/uritemplate/v3` and `golang.org/x/time`. No new environment variable.

## Backup

Back up before every upgrade. For the Docker Compose quick start, run in its directory (`werbot/`, with `docker-compose.yaml`):

```sh
docker compose exec -T postgres pg_dump -U werbot -Fc werbot > werbot-$(date +%F).dump
docker run --rm -v werbot_werbot-data:/data -v "$PWD":/backup busybox \
  tar -czf /backup/werbot-data-$(date +%F).tar.gz -C /data .
```

The cluster example (`docker-compose.cluster.yaml`, run in its directory) keeps recordings in S3 (SeaweedFS), so back up Postgres only:

```sh
docker compose -f docker-compose.cluster.yaml exec -T postgres pg_dump -U werbot -Fc werbot > werbot-$(date +%F).dump
```

Store `WERBOT_SECRET_KEY` separately: without it the backup cannot be decrypted.

## Upgrade with Docker Compose

1. Read the release notes.
2. Make a [backup](#backup).
3. Set the new tag in `.env`: `WERBOT_VERSION=v0.2.0`.
4. Run:

```sh
docker compose pull && docker compose up -d --wait
```

With one container, `up -d` is a full stop: the old container stops before the new one starts.

## Upgrade a cluster

Use this order for a setup with several server replicas, `werbot web` and standalone gateways:

1. Stop every gateway instance on every host. Active SSH sessions end, so plan a window.
2. Stop every server replica and `werbot web`.
3. Make a [backup](#backup).
4. Start the new server version. One replica applies migrations under an advisory lock; the others wait.
5. Start `werbot web` of the same version. A different version makes the UI show a version-mismatch banner.
6. Update and start the gateways.

For the example in `deploy/`, stop everything except Postgres (a plain `stop` with the `gateways` profile would stop Postgres too and the backup would fail):

```sh
docker compose -f docker-compose.cluster.yaml stop gateway-1 gateway-2 web server
docker compose -f docker-compose.cluster.yaml exec -T postgres pg_dump -U werbot -Fc werbot > werbot-$(date +%F).dump
# set WERBOT_VERSION in .env
docker compose -f docker-compose.cluster.yaml --profile gateways pull
docker compose -f docker-compose.cluster.yaml --profile gateways up -d --wait
```

Stop first: `up -d` recreates containers one by one, so the old and the new version would run together.

## Upgrade the binary

Replace the `werbot` file and restart the process. Verify the download first: [security.md](security.md#verifying-releases).

## Migrations

`werbot server` applies migrations (schema and River job queue) at start, under an advisory lock, so replicas do not race. To run them by hand: `werbot migrate`.

## Downgrade

Not supported. Set the previous `WERBOT_VERSION` and restore the backup:

```sh
docker compose exec -T postgres pg_restore -U werbot -d werbot --clean --if-exists < werbot-2026-11-18.dump
```

Restore into a stopped stack where only Postgres runs.

## Gateways

A gateway runs the same version as the server. Stop it before the server upgrade and start it after (see [Upgrade a cluster](#upgrade-a-cluster)). A standalone gateway offline for more than 24h needs a new join token ([configuration.md](configuration.md#gateway-werbot-gateway)).

## Web

`werbot web` runs the same version as the server and is upgraded with it (see [Upgrade a cluster](#upgrade-a-cluster)).
