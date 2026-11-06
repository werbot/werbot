SHELL:= /bin/bash
.DEFAULT_GOAL:=help

#############################################################################
.PHONY: help
help:
	@grep --no-filename -E '^[a-zA-Z_/-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
#############################################################################


#############################################################################
.PHONY: key
key: ## Generating key
	@scripts/key $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
# run:
# make protos - recreate all protofiles
# make protos user - recreate protofile user from folder /internal/core/
.PHONY: protos
protos: ## Generating protos files
	@scripts/proto $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: tools
tools: ## Install/Update tools and mods
	@scripts/tools $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: geolite
geolite: ## Updating and install GeoLite database to the latest version
	@scripts/geolite
#############################################################################


#############################################################################
.PHONY: build
build: ## Building project in bin folder
	@scripts/build $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: build-local
build-local: ## Building project natively in bin folder
	@scripts/build --local $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: prod_build
prod_build: build ## Alias for build (kept for existing deployment scripts)
#############################################################################


#############################################################################
.PHONY: test
test: ## Running tests
	@go test -count=1 ./...
#############################################################################


#############################################################################
.PHONY: lint
lint: ## Running linters
	@golangci-lint run ./...
#############################################################################


#############################################################################
.PHONY: submodules
submodules: ## Submodule status, check, sync and tests
	@scripts/submodules $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
SUBMODULES_CHECK_ARGS ?= --mode main
SUBMODULES_SYNC_ARGS ?= --commit

.PHONY: submodules-check
submodules-check: ## Check submodule pins against remote main
	@scripts/submodules check $(SUBMODULES_CHECK_ARGS)
#############################################################################


#############################################################################
.PHONY: submodules-sync
submodules-sync: ## Merge module branches into main and commit pins
	@scripts/submodules sync $(SUBMODULES_SYNC_ARGS)
#############################################################################


#############################################################################
.PHONY: submodules-test
submodules-test: ## Run offline submodule tool tests
	@scripts/submodules.test.sh
#############################################################################


#############################################################################
.PHONY: hooks
hooks: ## Install repository git hooks
	@scripts/submodules install-hook
#############################################################################


#############################################################################
.PHONY: haproxy
haproxy:
	@scripts/haproxy $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: init
init: ## Initialization develop environment
	@scripts/init
#############################################################################


#############################################################################
# install latest version goose - go install github.com/pressly/goose/v3/cmd/goose@latest
.PHONY: migration
migration: # Migration sql
	@scripts/migration $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: env_tools
env_tools: ## Tools for .env
	@scripts/env_tools $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: clean
clean: ## Cleaning garbage and inactive containers
	@scripts/clean
#############################################################################


#############################################################################
# Catch-all target. It is required so that extra arguments (e.g. `make protos user`
# or `make build taco`) reach the scripts instead of failing with "No rule to make target".
%: ## A parameter
	@true
#############################################################################
