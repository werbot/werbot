.PHONY: build test lint dev-db proto sqlc web e2e e2e-network

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
	$(E2E_NETWORK) up -d --build --wait postgres server target || { $(E2E_NETWORK) down -v; exit 1; }
	deploy/e2e/run.sh
