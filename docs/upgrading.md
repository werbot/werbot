# Upgrading

## Pre-1.0 policy

Between minor `v0.x` releases incompatible changes are possible. They are listed in the "Breaking changes" section of the release notes. Only the latest release is supported.

In `v0.x` an upgrade needs a full stop: every server replica and every gateway instance runs the same version, and a rolling upgrade is not supported (it comes with the public beta and expand/contract migrations).

## Backup

Back up before every upgrade. For the Docker Compose quick start (`deploy/`):

```sh
docker compose exec -T postgres pg_dump -U werbot -Fc werbot > werbot-$(date +%F).dump
docker run --rm -v werbot_werbot-data:/data -v "$PWD":/backup busybox \
  tar -czf /backup/werbot-data-$(date +%F).tar.gz -C /data .
```

The cluster example (`deploy/`) keeps recordings in S3 (SeaweedFS), so back up Postgres only:

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
