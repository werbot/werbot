# werbot CLI

`werbot login` signs the CLI in to a server. Afterwards `werbot ssh`, `werbot ssh-config` and `werbot connect` use a short-lived SSH certificate instead of a key you register by hand.

## Log in

```sh
werbot login --server https://werbot.example.com
```

The CLI opens the browser (the authorization code flow with PKCE and a redirect to `127.0.0.1`). The browser shows a consent page; approve it. On success the CLI prints `Logged in to <server> as <login>; certificate valid until <time>`. If the browser cannot be opened the URL is printed.

- `--device` uses the device flow (RFC 8628) for a machine without a browser: the CLI prints a code and a URL to open on another device.
- The server comes from `--server`, else `WERBOT_SERVER`, else the server of the last login. Without any of them the command fails with `--server is required`.
- Use an `https://` server URL in production. Over `http://` the tokens travel in clear text ([security.md](security.md#cli-sessions-and-user-certificates)).

## Profiles

Profiles live in `WERBOT_HOME` (default `~/.werbot`, Windows `%USERPROFILE%\.werbot`), one directory per server:

| File | Content |
|---|---|
| `profile.json` | server, login, access and refresh token |
| `id_ed25519`, `id_ed25519-cert.pub` | the CLI key and its user certificate |
| `known_hosts` | host keys of the gateways, written by `ssh`, `ssh-config` and `connect` |
| `current` (in the home) | the server of the last login |

Files are `0600`, directories `0700`. There is no keyring: protect the home directory like `~/.ssh`. Set `WERBOT_SERVER` to use another logged-in server for one command.

## Commands

| Command | Does |
|---|---|
| `werbot ls` | Table `NAME TYPE KINDS GATEWAY` of the resources you can open. |
| `werbot ssh <resource> [-- command...]` | Runs the system `ssh` with the profile key, certificate and `known_hosts` as `<login>+<resource>`. The exit code of `ssh` is the exit code of `werbot`. |
| `werbot ssh-config` | Prints an OpenSSH config block `Host <resource>.<server-host>` for each resource with the `terminal`, `exec` or `stream` kind, and refreshes the certificate and `known_hosts`. |
| `werbot connect <resource> ...` | As in [connect.md](connect.md), with the login, gateway, key and `known_hosts` taken from the profile. |
| `werbot status` | Server, user, organization, token expiry and certificate. |
| `werbot logout` | Revokes the session on the server and removes the profile. |

A resource without a gateway address fails with `gateway address for <resource> is not set; ask an admin to set it on the Gateways page` ([gateway.md](gateway.md#ssh-address)).

## Certificates

The server signs an SSH user certificate for the CLI key and the gateways accept it directly.

- A certificate is valid for 1 hour. Any command renews it when less than 10 minutes remain.
- When the server cannot be reached, a certificate that is still valid is used as it is.
- If the clock of the computer differs from the server clock by more than 1 minute, renewal fails with a `clock skew` error: sync the system clock.

## OpenSSH config

```sh
werbot ssh-config >> ~/.ssh/config.d/werbot   # or Include it from ~/.ssh/config
ssh db.werbot.example.com
```

The block sets `User <login>+<resource>`, `IdentityFile`, `CertificateFile`, `UserKnownHostsFile` and `IdentitiesOnly yes`. The certificate in it is renewed by any later `werbot` command, so run `werbot status` (or any command) before a long-idle `ssh`. A resource without a gateway address gets a comment line. Resources that only have the `datagram` kind are skipped: use `werbot connect`.

## CLI sessions

Every login is a CLI session. An organization sets its lifetime in Settings (idle timeout and maximum age). A user sees the sessions in Profile, CLI sessions and revokes one there; `logout` revokes the current one. A revoked session stops working at the next API request (`unauthenticated`) and the next gateway connection, and a refresh fails with `session expired or revoked; run werbot login`. Open connections keep running.

Each refresh rotates the refresh token. If a refresh response is lost (for example, the network drops) and the CLI retries with the old token, the server treats it as reuse and revokes the session: run `werbot login` again. Security details: [security.md](security.md#cli-sessions-and-user-certificates).

## Windows

`werbot login`, `ls`, `status`, `ssh-config` and `connect` work on Windows. `werbot ssh` needs the OpenSSH client (`ssh.exe`) in `PATH`.
