SHELL:= /bin/bash
ROOT_PATH:=$(abspath $(patsubst %/,%,$(dir $(abspath $(lastword ../$(MAKEFILE_LIST))))))
GO_PATH:=$(shell go env GOPATH)
CPU_ARCH:=$(shell go env GOARCH)
OS_NAME:=$(shell go env GOHOSTOS)
include $(ROOT_PATH)/.vscode/config/.env

.DEFAULT_GOAL:=help

#############################################################################
.PHONY: help
help: 
	@grep --no-filename -E '^[a-zA-Z_/-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
#############################################################################


#############################################################################
.PHONY: go
go: 
	@echo ${CPU_ARCH} ${OS_NAME}
#############################################################################
