SHELL:= /bin/bash
.DEFAULT_GOAL:=help

#############################################################################
.PHONY: help
help:
	@grep --no-filename -E '^[a-zA-Z_/-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
#############################################################################


#############################################################################
.PHONY: gen_key_aes
gen_key_aes: ## Generating AES key
	@scripts/gen/key aes
#############################################################################


#############################################################################
.PHONY: gen_key_server
gen_key_server: ## Generating ssh server key
	@scripts/gen/key server
#############################################################################


#############################################################################
.PHONY: gen_key_jwt
gen_key_jwt: ## Generating JWT key
	@scripts/gen/key jwt
#############################################################################


#############################################################################
# run:
# make gen_protos - recreate all protofiles
# make gen_protos user - recreate protofile user from folder /internal/grpc/
.PHONY: gen_protos
gen_protos: ## Generating protos files
	@scripts/gen/protos $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: upd_tools
upd_tools: ## Install/Update tools and mods
	@scripts/tools/apps
#############################################################################


#############################################################################
.PHONY: gen_key_grpc
gen_key_grpc: ## Generating TLS keys for gRPC
	@scripts/gen/key grpc
#############################################################################


#############################################################################
.PHONY: upd_geolite
upd_geolite: ## Updating and install GeoLite database to the latest version
	@scripts/tools/geolite
#############################################################################


#############################################################################
.PHONY: upd_golang
upd_golang: ## Updating and install Go to the latest version
	@scripts/tools/golang
#############################################################################


#############################################################################
.PHONY: release
release: ## Building new release
	@scripts/build/release $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: build
build: ## Building project in bin folder
	@scripts/build/build $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: lint
lint: ## Cleaning garbage and inactive containers
	@scripts/srv/lint
#############################################################################


#############################################################################
.PHONY: upd_cdn_ip
upd_cdn_ip:
	@scripts/tools/haproxy cdn $(wordlist 1,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
# install latest version goose - go install github.com/pressly/goose/v3/cmd/goose@latest
.PHONY: srv_migration
srv_migration: # Migration sql
	@scripts/srv/migration $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: srv_migration_dev
srv_migration_dev: # Dev migration sql
	@scripts/srv/migration dev $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
#############################################################################


#############################################################################
.PHONY: env_dev
env_dev: ## Scan new .env for dev environment
	@scripts/tools/env_scan update
#############################################################################


#############################################################################
.PHONY: upd_install
upd_install:
	@scripts/build/install
#############################################################################


#############################################################################
.PHONY: clean
clean: ## Cleaning garbage and inactive containers
	@scripts/srv/clean
#############################################################################


#############################################################################
%: ## A parameter
	@true
#############################################################################
