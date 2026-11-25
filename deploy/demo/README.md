# Demo: an AI agent in production without keys

A throwaway stack shows what Werbot does between an agent and a server. The agent has no SSH key and no password. It gets a short-lived certificate for one run, and the gateway checks every command.

| Step | The agent runs | Result |
| --- | --- | --- |
| 1 | `ls /etc` on `prod-web` | Runs: `read` is allowed by the baseline. |
| 2 | `rm -rf ~/site` | Refused (`Access denied`): `destructive` is never allowed to agents. |
| 3 | `touch ~/deployed` | Waits for an approval (rule `prod-write-approval`); the demo approves it with `werbot approve`, then it runs. |
| 4 | `cat ~/.ssh/id_ed25519` | Refused: `secrets` is never allowed to agents; the sponsor gets an alert. |
| 5 | `werbot agents` | The run and its transcript, with a link to the page of the run. |

Background: [docs/agents.md](../../docs/agents.md), [docs/policies.md](../../docs/policies.md#command-approvals).

## Run it

You need Docker with Compose v2, `curl`, `jq`, `openssl`, `ssh` and a `werbot` CLI on `PATH` (or `WERBOT_BIN=/path/to/werbot`).

```sh
WERBOT_VERSION=v0.2.0 ./demo.sh        # one step at a time, Enter continues
WERBOT_VERSION=v0.2.0 ./demo.sh --ci   # no pauses
```

- The compose project is `werbot-demo`. It publishes only `127.0.0.1:18090` (UI and API) and `127.0.0.1:12222` (SSH gateway), so it can run next to a normal install.
- Every secret (encryption key, database and target passwords, owner password) is generated per run and never stored. The owner password is printed at the end, so you can open the run in the UI.
- Run from source: `docker build -f deploy/Dockerfile -t werbot:demo . && WERBOT_IMAGE=werbot:demo ./demo.sh`.
- The script starts the stack and removes it with `docker compose down -v` when it exits, also after a failure or Ctrl-C.

## Record `demo.cast`

The cast is recorded by the maintainer at release time on the published image, and committed as `deploy/demo/demo.cast`.

The current cast shows `v0.2.0` (110×32, about 1 minute). Play it with `asciinema play deploy/demo/demo.cast`.

```sh
asciinema rec --idle-time-limit 2 --command "WERBOT_VERSION=v<release> ./demo.sh" demo.cast
```

Check the cast before you commit it: the generated passwords are throwaway, but the file must show nothing else from your machine.
