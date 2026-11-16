.PHONY: build test lint dev-db proto web

VERSION ?= dev

build: web
	go build -ldflags "-X github.com/werbot/werbot/internal/platform/version.Version=$(VERSION) -X github.com/werbot/werbot/internal/platform/version.Commit=$(shell git rev-parse --short HEAD) -X github.com/werbot/werbot/internal/platform/version.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/werbot ./cmd/werbot

test:
	go test -count=1 ./...

lint:
	golangci-lint run ./...

dev-db:
	docker compose -f deploy/docker-compose.test.yaml up -d --wait

proto:
	go tool buf lint && go tool buf generate

web:
	cd web && bun install && VITE_WERBOT_VERSION=$(VERSION) bun run build && touch build/.gitkeep
