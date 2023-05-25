SHELL:= /bin/bash
ROOT_PATH:=$(abspath $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST))))))
GO_PATH:=$(shell go env GOPATH)
CPU_ARCH:=$(shell go env GOARCH)
OS_NAME:=$(shell go env GOHOSTOS)
include $(ROOT_PATH)/.env

DATE=$(shell date '+%Y-%m-%d-%H:%M:%S')
GIT_COMMIT=$(shell cd "${ROOT_PATH}" && git rev-parse --short HEAD)
GIT_DIRTY=$(shell cd "${ROOT_PATH}" && test -n "$(git status --porcelain)" && echo "+CHANGES" || true)
GIT_TAG=$(shell cd "${ROOT_PATH}" && git describe --tags --abbrev=0 2>/dev/null)

VERSION=$(subst v,,${GIT_TAG})

## Output related vars
ifdef TERM
	BOLD:=$(shell tput bold)
	RED:=$(shell tput setaf 1)
	GREEN:=$(shell tput setaf 2)
	YELLOW:=$(shell tput setaf 3)
	RESET:=$(shell tput sgr0)
endif

# $(msg) bla bla bla   instead of   @echo bla bla bla
msg = @echo

# $(call file_exists,file-name)
# Return non-null if a file exists.
file_exists = $(wildcard $1)

# $(call make_target_dir,directory-name-opt)
# Create a directory if it doesn't exist.
make_target_dir = $(if $(call file-exists,$(if $1,$1,$(dir $@))),,mkdir -p $(if $1,$1,$(dir $@)))

# $(call get_latest_release,golangci/golangci-lint)
# Latest version release of package.
get_latest_release = $(shell curl --silent "https://api.github.com/repos/$1/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

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
	@scripts/tools/mods
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
	$(msg) "$(GREEN)Building new version to git$(RESET)"
	$(eval NEW_VERSION=$(shell read -p "Enter new release version (current version ${VERSION}): " enter ; echo $${enter}))
	@if [ ${NEW_VERSION} ]; then\
		#sed -i -e "s/\(Licensed Work:\s*Werbot\s\)v[0-9][0-9.]*/\\1v${NEW_VERSION}/" $(ROOT_PATH)/LICENSE;\
		sed -i -e "s/\(Change Date:\s*\)[-0-9]\+/\\1$(shell date +'%Y-%m-%d' -d '4 years')/" $(ROOT_PATH)/LICENSE;\
		git add .;\
		git commit -a -m "meta: Create release";\
		git tag v${NEW_VERSION};\
		git push origin main;\
		git push --tags origin main;\
	fi
#############################################################################


#############################################################################
.PHONY: build
build: ## Building project in bin folder
	$(msg) "$(GREEN)Building project in bin folder$(RESET)"
	$(eval NAME=$(filter-out $@,$(MAKECMDGOALS)))
	@if [ ${NAME} ]; then\
		if [ -d ${ROOT_PATH}/cmd/${NAME}/ ];then\
			$(MAKE) -s build_go ${NAME}; \
		else \
			echo "error";\
		fi \
	else \
		for entry in ${ROOT_PATH}/cmd/*/; do\
			$(MAKE) -s build_go $$(basename $${entry});\
		done; \
	fi

.PHONY: build_go
build_go:
	$(eval NAME=$(filter-out $@,$(MAKECMDGOALS)))
	@echo "Build" ${NAME} ${VERSION};\
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X github.com/werbot/werbot/internal/version.version=${VERSION} -X github.com/werbot/werbot/internal/version.gitCommit=${GIT_COMMIT} -X github.com/werbot/werbot/internal/version.buildDate=${BUILD_DATE}" -o ${ROOT_PATH}/bin/${NAME} ${ROOT_PATH}/cmd/${NAME};\
	upx -1 -k bin/${NAME} >/dev/null 2>&1;\
	rm -rf bin/${NAME}.~
#############################################################################


#############################################################################
.PHONY: lint
lint: ## Cleaning garbage and inactive containers
	@scripts/srv/lint
#############################################################################


#############################################################################
.PHONY: upd_cdn_ip
upd_cdn_ip:
	@scripts/tools/cdn
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
	$(msg) "$(GREEN)Scan new .env for dev environment$(RESET)"
	$(eval ARG_TYPE = $(filter update,$(MAKECMDGOALS)))
	@if [ $(ARG_TYPE) ]; then\
		ENV_FILE="${ROOT_PATH}/.env";\
		if [ "$(ARG_TYPE)" == "update" ]; then\
			for entry in ${ROOT_PATH}/cmd/*/; do\
				$(call _upd_env_files,$${entry});\
			done \
		fi;\
	else \
		echo "Parameters not passed";\
	fi

define _upd_env_files
	NAME=$$(basename ${1});\
	PARAMETERS=();\
	HEADER=FALSE;\
	ENV_FILE="${ROOT_PATH}/.env";\
	echo "Scan $$NAME $$VERSION parameters";\
	for file in ${ROOT_PATH}/cmd/$$NAME/*.go; do\
		test -f "$$file" || continue;\
		PARAMETERS+="$$(awk '{while (match($$0, /(config.[a-zA-Z]+\("([_A-Z]+)[, "]+(?|[a-zA-Z0-9_:.\/]+|)(?|"\)|\)))/, result)){print result[2] "=" result[3];$$0 = sub($$0, "")}}' $$file) ";\
	done;\
	for i in $$(printf "%s\n" $$PARAMETERS | sort -u); do\
		PARAMETER_NAME=$$(echo $$i | cut -d= -f 1);\
		PARAMETER_ARGUMENT=$$(echo $$i | cut -d= -f 2);\
		if [[ ! $$( grep $$PARAMETER_NAME $$ENV_FILE) ]]; then\
			if [ $$HEADER == FALSE ]; then\
				echo -e "\n\n\n# New parameters from project files:" >>$$ENV_FILE;\
				HEADER=TRUE;\
			fi;\
			echo -e "$$PARAMETER_NAME=$$PARAMETER_ARGUMENT" >> $$ENV_FILE;\
		fi;\
	done
endef
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
