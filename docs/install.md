# Install

Werbot ships as a Docker image (`ghcr.io/werbot/werbot`) and as release archives for Linux and macOS. The quickest way is Docker Compose on one host. Gateways on other hosts: [gateway.md](gateway.md). The web UI on its own host: [web.md](web.md).

## Docker Compose

You need Docker with Compose v2. Run in an empty directory:

```sh
export WERBOT_VERSION=v0.1.0
mkdir werbot && cd werbot
curl -fsSLO https://raw.githubusercontent.com/werbot/werbot/$WERBOT_VERSION/deploy/docker-compose.yaml
curl -fsSL https://raw.githubusercontent.com/werbot/werbot/$WERBOT_VERSION/deploy/.env.example -o .env
sed -i.bak \
  -e "s|^WERBOT_VERSION=.*|WERBOT_VERSION=$WERBOT_VERSION|" \
  -e "s|^WERBOT_SECRET_KEY=.*|WERBOT_SECRET_KEY=$(openssl rand -base64 32)|" \
  -e "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$(openssl rand -hex 24)|" \
  .env && rm .env.bak
docker compose up -d --wait
```

On a remote server, read [Remote access](#remote-access) before the first login. On this host open http://localhost:8080: the setup page creates the first account and its organization.

What runs (project `werbot`):

- `postgres`: `postgres:18-alpine`, data in the volume `werbot_postgres-data` (`/var/lib/postgresql`).
- `werbot`: `werbot server --with-gateway` from `ghcr.io/werbot/werbot:${WERBOT_VERSION}`, state in the volume `werbot_werbot-data` (`/var/lib/werbot`). Docker checks it with `werbot healthcheck` (`GET /healthz`).

| Port | Purpose |
| --- | --- |
| `8080` | Web UI and API, `/healthz` and `/readyz`. |
| `2222` | SSH to the built-in gateway (pool `local`, available after Setup). |
| `8443` | mTLS tunnel for gateways on other hosts ([gateway.md](gateway.md)). |

- `docker compose config` and `up` refuse an empty `WERBOT_VERSION`, `WERBOT_SECRET_KEY` or `POSTGRES_PASSWORD` and name the generate command.
- Keep `.env` and a copy of `WERBOT_SECRET_KEY`: it decrypts stored credentials, and a backup is useless without it ([upgrading.md](upgrading.md#backup)).
- The `werbot` service stops within `stop_grace_period: 70s`: the built-in gateway drains its sessions (`WERBOT_GATEWAY_DRAIN_TIMEOUT`, `30s`), then the server stops.
- The image is public and needs no login. If `docker compose up` fails with `denied` from `ghcr.io`, Docker sends a stale stored login: run `docker logout ghcr.io` and retry.
- Pin `WERBOT_VERSION` to a release tag; there is no `latest` tag before 1.0. Upgrades: [upgrading.md](upgrading.md#upgrade-with-docker-compose).
- More settings (SMTP, trusted proxies, S3 for recordings) go into `.env`: [configuration.md](configuration.md).

## Remote access

In production mode the session cookie is `__Host-werbot_session` with the `Secure` flag. Browsers drop such a cookie over plain `http://` on any host but `localhost`, so a login at `http://<server-ip>:8080` does not work: the browser does not keep the session. On `http://localhost` use Chrome or Firefox; other browsers (Safari) may drop the `Secure` cookie there too.

Pick one way:

**SSH tunnel (to try Werbot).** On your machine run:

```sh
ssh -L 8080:localhost:8080 <user>@<server>
```

Keep it open and browse http://localhost:8080.

**TLS reverse proxy (for real use).** Point a DNS name at the server, for example `werbot.example.com`, and put a TLS proxy in front of port 8080. With [Caddy](https://caddyserver.com) the `Caddyfile` is:

```
werbot.example.com {
	reverse_proxy 127.0.0.1:8080
}
```

1. In `docker-compose.yaml` publish the UI on loopback only: `"127.0.0.1:8080:8080"` instead of `"8080:8080"`.
2. In `.env` set `WERBOT_PUBLIC_URL=https://werbot.example.com`.
3. In `.env` set `WERBOT_TRUSTED_PROXIES` to the proxy address as `werbot` sees it, so sessions and audit show the real client IP. For Caddy on the Docker host that is the gateway of the compose network: `docker network inspect werbot_default --format '{{(index .IPAM.Config 0).Gateway}}'` prints for example `172.18.0.1`; set `WERBOT_TRUSTED_PROXIES=172.18.0.1/32`.
4. Apply: `docker compose up -d --wait`.

Leave port `8443` (gateway tunnel) out of the proxy: it is mTLS and needs a direct TCP path ([gateway.md](gateway.md#firewall)).

## Binary

Release archives: `werbot_<version>_<os>_<arch>.tar.gz` for `linux` and `darwin` (`amd64`, `arm64`); the version has no `v`, for example `werbot_0.1.0_linux_amd64.tar.gz`. Windows `.zip` archives are for the client: on Windows only `werbot connect` is supported ([connect.md](connect.md)).

1. From the [releases page](https://github.com/werbot/werbot/releases) download the archive for your platform, `checksums.txt` and `checksums.txt.sigstore.json`.
2. Verify them: [security.md](security.md#verifying-releases).
3. Unpack and install:

   ```sh
   tar -xzf werbot_0.1.0_linux_amd64.tar.gz werbot
   sudo install -m 0755 werbot /usr/local/bin/werbot
   ```

   On macOS the binary is not notarized; remove the quarantine flag first: `xattr -d com.apple.quarantine ./werbot`.

The server needs PostgreSQL 18 and three variables: `WERBOT_DATABASE_URL`, `WERBOT_SECRET_KEY` (`openssl rand -base64 32`) and `WERBOT_PUBLIC_URL`. Start it with `werbot server --with-gateway` (API, UI and a built-in gateway in one process) or `werbot server` (without the gateway). All variables: [configuration.md](configuration.md).

A systemd unit on Linux:

```sh
sudo useradd --system --home-dir /var/lib/werbot --create-home werbot
sudo install -d -m 0700 /etc/werbot
sudo tee /etc/werbot/werbot.env > /dev/null <<EOF
WERBOT_DATABASE_URL=postgres://werbot:<password>@localhost:5432/werbot
WERBOT_SECRET_KEY=$(openssl rand -base64 32)
WERBOT_PUBLIC_URL=https://werbot.example.com
EOF
sudo chmod 0600 /etc/werbot/werbot.env
```

`/etc/systemd/system/werbot.service`:

```ini
[Unit]
Description=Werbot
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
User=werbot
EnvironmentFile=/etc/werbot/werbot.env
WorkingDirectory=/var/lib/werbot
ExecStart=/usr/local/bin/werbot server --with-gateway
Restart=on-failure
LimitNOFILE=65536
TimeoutStopSec=70

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl daemon-reload && sudo systemctl enable --now werbot
curl -fsS localhost:8080/healthz   # ok
```

`WorkingDirectory` holds `./data` (`WERBOT_DATA_DIR`: the built-in gateway and file-system recordings). `TimeoutStopSec=70` leaves time for the gateway drain, and `LimitNOFILE=65536` covers the default session limit ([scaling.md](scaling.md#resources)). Back up `/etc/werbot/werbot.env`: it holds `WERBOT_SECRET_KEY`. The [Remote access](#remote-access) rules apply here too.

## Next steps

- Gateways on other hosts and pools: [gateway.md](gateway.md).
- The web UI on its own host and domain: [web.md](web.md).
- All variables: [configuration.md](configuration.md).
- Upgrades and backups: [upgrading.md](upgrading.md).
- Several server replicas and gateway pools: [scaling.md](scaling.md).
