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
# make protos user - recreate protofile user from folder /internal/grpc/
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
.PHONY: release
release:
	@scripts/release $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: build
build: ## Building project in bin folder
	@scripts/build $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
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
%: ## A parameter
	@true
#############################################################################
