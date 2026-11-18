.PHONY: build test lint dev-db proto sqlc web e2e e2e-network e2e-cluster

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

# the harness (test/cluster) brings the stand up; on failure the logs go to
# test/cluster/compose.log; the stand is always removed at the end
e2e-cluster:
	docker build -f deploy/Dockerfile -t werbot:cluster .
	COMPOSE_PROJECT_NAME=$(CLUSTER_PROJECT) go test -tags cluster -count=1 -timeout 40m -v ./test/cluster/... ; rc=$$?; \
	[ $$rc -eq 0 ] || $(CLUSTER) logs --no-color --timestamps > test/cluster/compose.log; \
	$(CLUSTER) down -v --remove-orphans; exit $$rc
