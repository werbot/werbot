<p align="center">
    <a href="https://werbot.com" target="_blank" rel="noopener">
        <img src="https://github.com/werbot/.github/raw/main/img/werbot.png" alt="Werbot is a single sign-on solution for easy and secure sharing of servers, databases or app access" />
    </a>
</p>

<p align="center">
    <a href="https://github.com/werbot/werbot/releases">
    <img src="https://img.shields.io/github/v/release/werbot/werbot?sort=semver&label=Release&color=651FFF" />
    </a>
    &nbsp;
    <a href="/LICENSE"><img src="https://img.shields.io/badge/license-BUSL--1.1-green.svg"></a>
    &nbsp;
    <a href="https://goreportcard.com/report/github.com/werbot/werbot"><img src="https://goreportcard.com/badge/github.com/werbot/werbot"></a>
    &nbsp;
    <a href="https://www.codefactor.io/repository/github/werbot/werbot"><img src="https://www.codefactor.io/repository/github/werbot/werbot/badge" alt="CodeFactor" /></a>
    &nbsp;
    <a href="https://github.com/werbot/werbot"><img src="https://img.shields.io/badge/backend-go-orange.svg"></a>
    &nbsp;
    <a href="https://github.com/werbot/werbot/blob/main/go.mod"><img src="https://img.shields.io/github/go-mod/go-version/werbot/werbot?color=7fd5ea"></a>
    &nbsp;
    <a href="https://twitter.com/werbot_"><img src="https://img.shields.io/twitter/follow/werbot_?style=social"></a>
</p>

<p align="center">
    <a href="https://www.linkedin.com/company/werbot/"><img height="20" src="https://github.com/werbot/.github/raw/main/img/social/linkedin.svg" alt="LinkedIn"></a>
    &nbsp;
    <a href="https://twitter.com/werbot_"><img height="20" src="https://github.com/werbot/.github/raw/main/img/social/twitter.svg" alt="Twitter"></a>
    &nbsp;
    <a href="https://www.youtube.com/channel/UCQk0_i0h-xB9s9sv4R7HX2g"><img height="20" src="https://github.com/werbot/.github/raw/main/img/social/youtube.svg" alt="Youtube"></a>
    &nbsp;
    <a href="https://dev.to/werbot"><img height="20" src="https://github.com/werbot/.github/raw/main/img/social/dev.svg" alt="Dev"></a>
    &nbsp;
    <a href="https://stackoverflow.com/questions/tagged/werbot"><img height="20" src="https://github.com/werbot/.github/raw/main/img/social/stack-overflow.svg" alt="StackOverflow"></a>
</p>


---

## <img width="24" src="https://github.com/werbot/.github/raw/main/img/yellow/logo.svg">&nbsp;&nbsp;What is Werbot?

Werbot is an open-source solution allowing users to securely share access to servers, data bases, web applications, desktops, containers and clouds; providing full-fledged options for controlling and auditing of the work performed on them.

> ⚠️&nbsp;&nbsp;Current major version is zero (`v0.x.x`) to accommodate rapid development and fast iteration while getting early feedback from users. Please keep in mind that Werbot is still under active development and therefore full backward compatibility is not guaranteed before reaching v1.0.0.


## 🏆&nbsp;&nbsp;Features

- Werbot works with dedicated, VPS, and cloud servers
- Manages servers from different providers in one account
- Doesn’t require any additional agent to be installed on the server
- Records every server session and collects logs
- Provides a single sign-on

Werbot is written in golang, runs in Docker containers, and works as microservices. It requires little processing power, scales easily, and can be implemented in the workflow of any company within 1 hour.

**Supported technology:**

- _Protocols_ - SSH and Telnet are proxied and recorded today. Access profiles for RDP, VNC, databases and cloud providers exist in the data model; their proxy connectors are not implemented yet.
- _Containers_ - Docker, Kubernetes (access metadata)
- _Databases_ - MySQL, Maria, PostgresQL, Redis, MongoDB, Elasticsearch, and other (access metadata)

### Telnet access

Telnet schemes can be reached in two ways:

1. **SSH bridge** - connect to avocado over SSH, then select a `server_telnet` scheme from the shell or use its direct-access alias. Avocado logs in to the target automatically and records the session.
2. **Telnet listener** - set `TELNETSERVER_BIND_ADDRESS` (for example `:2323`) and start avocado. Request a one-time token with `POST /v1/tokens/scheme/{scheme_id}/connect`, connect to the listener, and enter the token at the `Token:` prompt.

The listener is disabled by default. The client-to-avocado leg of the listener and the avocado-to-target leg are plaintext because telnet has no encryption; the connection token is single-use and expires after 5 minutes. Prefer the SSH bridge when the network is untrusted.




## 🔥&nbsp;&nbsp;Why Werbot?

#### Problem

- Unsafely kept server access, passwords, and keys
- Difficult server access management
- Uncontrolled work on servers
- Unwanted connections on servers
- Expensive and limited in functionality server monitoring tools

<img src="https://github.com/werbot/.github/raw/main/img/promo/werbot_problem.png">

#### Solution

Werbot users connect to all accessible servers with a single sign-on using their login and private key. All work performed on servers connected to Werbot is logged and recorded as a screencast.

<img src="https://github.com/werbot/.github/raw/main/img/promo/werbot_solution.png">

#### Competition

Identity and Access Management solutions existing today can have limited functionalities or work with only one server provider, and support few protocols.

There are also complex Enterprise solutions that are quite expensive solutions and not suitable for everyone.

## 🚀&nbsp;&nbsp;Why did we build Werbot?

The prototype of Werbot was developed for internal use firstly. Over time, the prototype was refined and developed into a full-fledged platform available to everyone.

SaaS version is currently working on the site werbot.com. There we are offering a ready-made solution that is suitable for most companies and does not need to be configured by a specialist, so it can be used even without the involvement of cybersecurity specialists.

Werbot covers 3 of the most important cybersecurity challenges:

- Helps to manage server access
- Helps to control users’ activity on servers
- Gathers evidence to show security certification compliances

**Recently we decided to rewrite the code of the SaaS version and make it open source. We are currently working on this.**


## 🧬&nbsp;&nbsp;Project components

Here is a list of modules that are included within the `Werbot`.

| Component                                        | Description                                                                                                                                                         |
| :------------------------------------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [web](https://github.com/werbot/werbot.web)     | 🖥 Werbot web dashboard |
| [ee](https://github.com/werbot/werbot.ee)       | 🏭 Enterprise add-on placeholder (private; no backend code yet) |
| [saas](https://github.com/werbot/werbot.saas)   | 💳 SaaS add-on: subscriptions, licenses, billing, updates |
| [install](https://github.com/werbot/install.werbot.com) | 🚀 The script for installing Werbot |
| [agent.windows](https://github.com/werbot/agent.windows) | 👮‍♂️ Windows agent |
| [agent.unix](https://github.com/werbot/agent.unix) | 👮‍♂️ Unix agent |
| [pam](https://github.com/werbot/pam-nix)        | 🔐 Pluggable Authentication Module for native two factor authentication agents for *nix platforms |
 


## 📚&nbsp;&nbsp;Documentation

Documentation for Werbot is available in the [`docs/`](docs/) directory. Currently available:

- [Token Package Documentation](docs/packages/token.md) - Comprehensive guide to the token management system

Additional documentation is being actively developed. For the latest updates, please check the [docs/](docs/) directory.

## 🏁&nbsp;&nbsp;Installation

### Prerequisites

- Docker and Docker Compose
- Domain name with DNS access (Cloudflare recommended)
- Go 1.25+ (for development)
- Make (for build automation)

### Quick Start

1. **Clone the repository:**
   ```bash
   git clone https://github.com/werbot/werbot.git
   cd werbot
   ```

2. **Configure environment:**
   - Copy `.env.example` to `.env` (if available) or create `.env` file
   - Configure required environment variables:
     - `GEOLITE_LICENSE` - Geolite key for downloading the latest geolite database
     - `DOMAIN` - Second-level domain in format `*.domain.com`
     - `DNS_CLOUDFLARE_EMAIL` - Your Cloudflare email
     - `DNS_CLOUDFLARE_API_KEY` - Your Cloudflare API key

3. **Set up DNS records:**
   - Add DNS A records pointing to your server:
     - `api.domain.com` → Your server IP
     - `app.domain.com` → Your server IP

4. **Initialize the environment:**
   ```bash
   make init
   ```
   This script will:
   - Validate DNS configuration
   - Generate secure passwords for PostgreSQL and Redis
   - Generate encryption keys
   - Create necessary configuration files

5. **Start services:**
   ```bash
   cd docker
   docker-compose up -d
   ```

### Development Setup

1. **Install development tools:**
   ```bash
   make tools
   ```

2. **Build the project:**
   ```bash
   make build
   ```
   This will build all services into the `bin/` directory.

   For local run or debugging, build native host binaries instead:
   ```bash
   make build-local            # all services
   make build-local taco       # one service
   ```

3. **Run database migrations:**
   ```bash
   make migration
   ```

4. **Update GeoLite database:**
   ```bash
   make geolite
   ```

### Available Make Commands

- `make help` - Show all available commands
- `make init` - Initialize development environment
- `make build [service]` - Build project or specific service
- `make build-local [service]` - Build native host binaries into `bin/` for local run or debug
- `make tools` - Install/update development tools
- `make migration` - Run database migrations
- `make geolite` - Update GeoLite database
- `make clean` - Clean up temporary files and containers
- `make key` - Generate encryption keys
- `make protos` - Generate protobuf files

### Build editions

The default build is the community core. Editions are compiled with a build tag:

```bash
TAGS=saas make build taco   # SaaS add-on
TAGS=ee make build taco     # Enterprise skeleton (future commercial modules)
```

Release images for `taco` are built with `saas` enabled (see `.goreleaser.yaml`). Add-ons live in private git submodules under `add-on/`; initialize them with `git submodule update --init --recursive`.

### Submodules

`web`, `add-on/ee`, and `add-on/saas` are git submodules. The parent `main` must pin each module's remote `main` tip.

**Automatic synchronization**

- **After a parent `main` merge**: job `Submodule reconcile` merges every pinned module commit into the module `main`, pushes it, updates the parent gitlinks, and commits them back to `main`. That commit triggers a normal CI run.
- **Every six hours and on manual dispatch**: the same job catches a module `main` that advanced independently or a bypassed local hook.
- The merge commit can show a red `Submodule synchronization` check for a short window before the reconcile commit lands. The reconcile commit restores green; no action is needed.
- There is no pre-merge module merge and no automatic PR merge. Closing a parent PR cannot leak its code into a module `main`.

The workflow needs the repository secret `SUBMODULE_TOKEN` with **Contents: Read and write** on the parent repository and on `werbot/werbot.ee`, `werbot/werbot.saas`, and `werbot/werbot.web`. It verifies push access before writing and fails with `SUBMODULE_TOKEN lacks push access to <repo>` otherwise. Parent and module `main` branches must allow the token to push directly; configure a ruleset bypass if protection is enabled.

**What you do manually**

1. Commit and push the module change. The commit must exist on the module remote.
2. Commit the parent gitlink, push the branch, open the PR, and merge it when ready.

Branch names do not need to match. The local hook and CI accept any pushed module branch tip, and the reconcile job works from the pinned SHA alone.

**Local setup and diagnostics**

```bash
make hooks              # once per clone: install .githooks/pre-push
make submodules status  # pin, remote main, state
make submodules-check   # strict main check
make submodules-test    # offline regression suite (33 cases)
```

`make submodules-sync` remains available for a manual repair from a feature branch. It pushes the branch, merges it into module `main`, pushes, and commits the pins.

**What is enforced**

- **Push to parent `main`**: every pin equals the module remote `main` tip. The reconcile job repairs a violation after the push.
- **Push to a feature branch**: a changed pin equals the tip of any remote branch in the module. A module branch with unpushed commits blocks the push.
- **Pull request**: if no gitlink changed, CI runs the strict `main` check; otherwise it checks that every changed pin is a pushed branch tip.
- **CI**: job `Submodule synchronization` runs before `Build, test and lint`; the build job cannot start when the gate fails.
- **Dirty module worktrees** produce a warning; uncommitted changes are never pushed.

**Manual repair**

If automation is unavailable, update pins or run the local sync:

```bash
git submodule update --remote --checkout add-on/ee add-on/saas web
git add add-on/ee add-on/saas web
git commit -m "🔧 chore(submodules): bump modules to main"
```

Bypass the local hook with `SKIP_SUBMODULE_CHECK=1 git push` only for emergencies; CI and the reconcile job still enforce the invariant.

### Tests

Integration tests need PostgreSQL 18 on port `5433`:

```bash
docker compose -f docker/docker-compose-test.yaml up -d testdb
make test
```

The same database settings (`localhost:5433`, user `postgres`, password `password`) are used by `go test ./...` locally and in CI.

### Docker Services

The project runs as microservices in Docker containers:

- **taco** - Main API service
- **buffet** - gRPC service
- **avocado** - Authentication service
- **ghost** - Background worker service
- **app** - Web frontend
- **postgres** - PostgreSQL 18 database (`postgres:18-alpine`)
- **redis** - Redis cache
- **haproxy** - Load balancer and reverse proxy
- **acme** - SSL certificate management (Let's Encrypt)

For more details, see [`docker/docker-compose.yaml`](docker/docker-compose.yaml).

> **Upgrading from PostgreSQL 16:** the `postgres_data` volume is now mounted at `/var/lib/postgresql` (PostgreSQL 18 stores data in `/var/lib/postgresql/18/docker`). A PostgreSQL 16 data directory is not compatible with PostgreSQL 18. Dump the database with `pg_dumpall` before upgrading, start PostgreSQL 18 on a fresh volume, then restore the dump.

## 👑&nbsp;&nbsp;Community

Join our growing community around the world, for help, ideas, and discussions regarding Werbot.

- Follow us on [Twitter](https://twitter.com/werbot_)
- Connect with us on [LinkedIn](https://www.linkedin.com/company/werbot)
- Visit us on [YouTube](https://www.youtube.com/channel/UCQk0_i0h-xB9s9sv4R7HX2g)
- Join our [Dev community](https://dev.to/werbot)
- Questions tagged #werbot on [Stack Overflow](https://stackoverflow.com/questions/tagged/werbot)

## 👍&nbsp;&nbsp;Contribute

We would for you to get involved with Werbot development! If you want to say **thank you** and/or support the active development of `Werbot`:

1. Add a [GitHub Star](https://github.com/werbot/werbot/stargazers) to the project.
2. Tweet about the project [on your Twitter](https://twitter.com/intent/tweet?text=Werbot%20is%20an%20%221Password%22%20for%20servers%20and%20teams%20-%20open%20source%20solution%20with%20single%20sign-on%20for%20easy%20and%20secure%20sharing%20of%20servers%2C%20databases%2C%20or%20app%20access.%20https%3A%2F%2Fgithub.com%2Fwerbot%2Fwerbot).
3. Write a review or tutorial on [Medium](https://medium.com/), [Dev.to](https://dev.to/) or personal blog.

You can learn more about how you can contribute to this project in the [contribution guide](CONTRIBUTING.md).

## 🚨&nbsp;&nbsp;Security

For security issues, view our [vulnerability policy](https://github.com/werbot/werbot/security/policy), view our [security policy](https://werbot.com/legal/security), and kindly email us at [security@werbot.com](mailto:security@werbot.com) instead of posting a public issue on GitHub.

## 📜&nbsp;&nbsp;License

Source code for Werbot, located in [this repository](https://github.com/werbot/werbot), is released under the [Business Source License 1.1](/LICENSE).

All content that resides under the "**add-on/\*/**" directory of this repository, if that directory exists, is licensed under the license defined in "**add-on/\*/LICENSE**".

All content that resides under the "**web/**" directory of this repository, if that directory exists, is licensed under the license defined in "**web/LICENSE**".

All third party components incorporated into the Werbot Software are licensed under the original license provided by the owner of the applicable component.

## ❓&nbsp;&nbsp;License FAQ

**What is the license?**

The Business Source [License](/LICENSE) is identical to Apache 2.0 with the only exception being that you can't use the code to create a cloud service or, in other words, resell the product to others.

BSL is adopted by MariaDB, Sentry, CockroachDB, Couchbase and many others. In most cases, it is a more permissive license than, for example, AGPL, because it allows you to make private changes to the code.

In three years, the code also becomes available under Apache 2.0 license. You can learn more about BSL [here](https://mariadb.com/bsl-faq-adopting/).

**Why BSL license**?

We picked the license to allow users to share access to their servers, databases, app, or application access features or access monitoring features using Werbot, but forbidding other companies to create a cloud service using the code.

We provide a [application access service](https://werbot.com/) ourselves in order to monetize our work and sustain development efforts.

**Are you open-source?**

Technically, the BSL license is classified as source-available, but we continue to use the term open-source on the basis that the source code is open.

Existing SEO practices don't leave us much choice and our competitors do more or less the same.

