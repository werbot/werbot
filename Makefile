.PHONY: build test lint dev-db proto sqlc web e2e e2e-network e2e-sso e2e-cluster e2e-databases redteam redteam-go redteam-sso load fuzz bench-hot site release-check release-snapshot

VERSION ?= dev
GORELEASER ?= go run github.com/goreleaser/goreleaser/v2@v2.20.0
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)

build: web
	go build -ldflags "-X github.com/werbot/werbot/internal/platform/version.Version=$(VERSION) -X github.com/werbot/werbot/internal/platform/version.Commit=$(COMMIT) -X github.com/werbot/werbot/internal/platform/version.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/werbot ./cmd/werbot

test:
	go test -count=1 ./...

lint:
	golangci-lint run ./...

dev-db:
	docker compose -f deploy/docker-compose.test.yaml up -d --wait

proto:
	go tool -modfile=tools/go.mod buf lint && go tool -modfile=tools/go.mod buf generate

sqlc:
	go tool -modfile=tools/go.mod sqlc generate

web:
	cd web && bun install --frozen-lockfile && VITE_WERBOT_VERSION=$(VERSION) bun run build

e2e: build
	docker compose -f deploy/docker-compose.test.yaml exec -T testdb psql -U postgres -c 'DROP DATABASE IF EXISTS werbot_e2e WITH (FORCE)' -c 'CREATE DATABASE werbot_e2e'
	cd web && bunx playwright test

E2E_NETWORK = docker compose -p werbot-e2e -f deploy/e2e/docker-compose.yaml

# run.sh tears the stack down on exit; a failed `up` tears it down here;
# the first `down` clears a stack left by a killed run
e2e-network:
	$(E2E_NETWORK) --profile gateway --profile vnet down -v
	CGO_ENABLED=0 GOOS=linux GOARCH=$$(docker version -f '{{.Server.Arch}}') go build -o deploy/e2e/.bin/echotarget ./internal/e2e/echotarget
	CGO_ENABLED=0 GOOS=linux GOARCH=$$(docker version -f '{{.Server.Arch}}') go build -o deploy/e2e/.bin/werbot ./cmd/werbot
	$(E2E_NETWORK) up -d --build --wait postgres server target router echo dns || { $(E2E_NETWORK) down -v; exit 1; }
	deploy/e2e/run.sh

KEYCLOAK = docker compose -f deploy/e2e/keycloak.yaml

# SSO tests against the Keycloak stand (needs make dev-db); the stand is
# always removed at the end
e2e-sso:
	$(KEYCLOAK) up -d --wait || { $(KEYCLOAK) down -v; exit 1; }
	go test -race -count=1 -run Keycloak -v ./internal/app/ -args -keycloak=http://127.0.0.1:18180; rc=$$?; \
	$(KEYCLOAK) down -v; exit $$rc

DATABASES = docker compose -f deploy/e2e/databases.yaml

# connector tests against the databases stand (needs a free port 15432); the
# stand is always removed at the end
e2e-databases:
	sh deploy/e2e/databases/gencerts.sh
	$(DATABASES) up -d --wait || { $(DATABASES) down -v; exit 1; }
	WERBOT_TEST_POSTGRES=127.0.0.1:15432 go test -count=1 -run Real -v ./internal/gateway/pgconnector/; rc=$$?; \
	$(DATABASES) down -v; exit $$rc

CLUSTER_PROJECT ?= werbot-cluster
CLUSTER = docker compose -p $(CLUSTER_PROJECT) -f test/cluster/docker-compose.yaml
CLUSTER_LOG = test/cluster/compose.log
CLUSTER_LOGS = $(CLUSTER) logs --no-color --timestamps

# the harness (test/cluster) brings the stand up; on failure the logs go to
# test/cluster/compose.log; the stand is always removed at the end
e2e-cluster:
	docker build -f deploy/Dockerfile -t werbot:cluster .
	COMPOSE_PROJECT_NAME=$(CLUSTER_PROJECT) go test -tags cluster -count=1 -timeout 50m -v ./test/cluster/... ; rc=$$?; \
	[ $$rc -eq 0 ] || $(CLUSTER_LOGS) > $(CLUSTER_LOG); \
	$(CLUSTER) down -v --remove-orphans; exit $$rc

REDTEAM = docker compose -p werbot-redteam -f test/cluster/docker-compose.yaml

# red-team scenarios (test/redteam) on the cluster stand under their own
# project; on failure the logs go to test/redteam/compose.log; the stand is
# always removed at the end
redteam-go:
	docker build -f deploy/Dockerfile -t werbot:cluster .
	COMPOSE_PROJECT_NAME=werbot-redteam go test -tags redteam -count=1 -timeout 45m -v ./test/redteam/... ; rc=$$?; \
	[ $$rc -eq 0 ] || $(REDTEAM) logs --no-color --timestamps > test/redteam/compose.log; \
	$(REDTEAM) down -v --remove-orphans; exit $$rc

# red-team browser scenario (web/redteam): the Keycloak stand and the built server on
# :18080 (needs make dev-db and a free port 18080); the Keycloak stand is always removed
redteam-sso: build
	docker compose -f deploy/docker-compose.test.yaml exec -T testdb psql -U postgres -c 'DROP DATABASE IF EXISTS werbot_redteam WITH (FORCE)' -c 'CREATE DATABASE werbot_redteam'
	$(KEYCLOAK) up -d --wait || { $(KEYCLOAK) down -v; exit 1; }
	cd web && bunx playwright install chromium && WERBOT_E2E_DATABASE_URL='postgres://postgres:password@localhost:27432/werbot_redteam?sslmode=disable' bunx playwright test -c playwright.redteam.config.ts; rc=$$?; \
	cd .. && $(KEYCLOAK) down -v; exit $$rc

redteam: redteam-go redteam-sso

LOAD_TESTS ?= L1 L2 L3 L4 L5

# manual load tests (test/load), Linux only: RSS and listeners come from /proc.
# Each load test runs on a fresh stand, removed after it; the next one does not
# inherit its RSS baseline or data. The stand logs of each failed test go to
# test/cluster/compose.log before its stand is removed. Smaller run: LOAD_ARGS='-load.sessions=100 -load.duration=1m -load.instances=20 -load.resources=500'
load:
	@[ "$$(uname -s)" = Linux ] || { echo "load tests need Linux (/proc)"; exit 1; }
	docker build -f deploy/Dockerfile -t werbot:cluster .
	@rm -f $(CLUSTER_LOG); rc=0; for l in $(LOAD_TESTS); do \
	  COMPOSE_PROJECT_NAME=$(CLUSTER_PROJECT) go test -tags load -count=1 -timeout 2h -v -run "^Test$$l$$" ./test/load/ -args $(LOAD_ARGS) || \
	    { rc=1; echo "== $$l" >> $(CLUSTER_LOG); $(CLUSTER_LOGS) >> $(CLUSTER_LOG); }; \
	  $(CLUSTER) down -v --remove-orphans; \
	done; exit $$rc

# Short fuzz runs of the parser targets; a crasher is written under testdata/fuzz and committed.
FUZZTIME ?= 30s
FUZZ_TARGETS = ./internal/platform/shellclass:FuzzClassify ./internal/platform/shellclass:FuzzAnalyze ./internal/platform/sshdgram:FuzzReadFrame ./internal/tunnel:FuzzStreamFrame ./internal/gateway/pgconnector:FuzzPGMessage ./internal/platform/sqlclass/pg:FuzzSQLClass
fuzz:
	@for t in $(FUZZ_TARGETS); do pkg=$${t%%:*}; fn=$${t##*:}; echo "== $$fn"; \
	  go test -run '^$$' -fuzz "^$$fn$$" -fuzztime $(FUZZTIME) $$pkg || exit 1; done

# Hot-path budgets (TestHotPath* of each package) against the test database
# on localhost:27432 (make dev-db); run alone, other load skews them.
HOT_PKGS ?= ./internal/gateway/pgconnector/ ./internal/platform/sqlclass/pg/
bench-hot:
	HOT_BUDGET=1 go test -count=1 -run '^TestHotPath' -v $(HOT_PKGS)

# docs site (site/): Starlight pages synced from docs/; SITE_REF is the git ref the links to repo files point at
SITE_REF ?= main
site:
	SITE_REF=$(SITE_REF) site/sync.sh && cd site && bun install --frozen-lockfile && bun --bun run build

release-check:
	$(GORELEASER) check

release-snapshot:
	$(GORELEASER) release --snapshot --clean --skip=sign
