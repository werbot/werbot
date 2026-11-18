.PHONY: build test lint dev-db proto sqlc web e2e e2e-network e2e-cluster load

VERSION ?= dev
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
	cd web && bun install && VITE_WERBOT_VERSION=$(VERSION) bun run build

e2e: build
	docker compose -f deploy/docker-compose.test.yaml exec -T testdb psql -U postgres -c 'DROP DATABASE IF EXISTS werbot_e2e WITH (FORCE)' -c 'CREATE DATABASE werbot_e2e'
	cd web && bunx playwright test

E2E_NETWORK = docker compose -p werbot-e2e -f deploy/e2e/docker-compose.yaml

# run.sh tears the stack down on exit; a failed `up` tears it down here;
# the first `down` clears a stack left by a killed run
e2e-network:
	$(E2E_NETWORK) --profile gateway down -v
	CGO_ENABLED=0 GOOS=linux GOARCH=$$(docker version -f '{{.Server.Arch}}') go build -o deploy/e2e/.bin/echotarget ./internal/e2e/echotarget
	$(E2E_NETWORK) up -d --build --wait postgres server target router echo dns || { $(E2E_NETWORK) down -v; exit 1; }
	deploy/e2e/run.sh

CLUSTER_PROJECT ?= werbot-cluster
CLUSTER = docker compose -p $(CLUSTER_PROJECT) -f test/cluster/docker-compose.yaml
CLUSTER_LOG = test/cluster/compose.log
CLUSTER_LOGS = $(CLUSTER) logs --no-color --timestamps

# the harness (test/cluster) brings the stand up; on failure the logs go to
# test/cluster/compose.log; the stand is always removed at the end
e2e-cluster:
	docker build -f deploy/Dockerfile -t werbot:cluster .
	COMPOSE_PROJECT_NAME=$(CLUSTER_PROJECT) go test -tags cluster -count=1 -timeout 40m -v ./test/cluster/... ; rc=$$?; \
	[ $$rc -eq 0 ] || $(CLUSTER_LOGS) > $(CLUSTER_LOG); \
	$(CLUSTER) down -v --remove-orphans; exit $$rc

LOAD_TESTS ?= L1 L2 L3 L4

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
