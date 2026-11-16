.PHONY: build test lint dev-db proto sqlc web

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
