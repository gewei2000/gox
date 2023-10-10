export NAME ?= gox
export VERSION := $(shell cat VERSION)
export REVISION := $(shell git rev-parse --short=8 HEAD || echo unknown)
export BRANCH := $(shell git show-ref | grep "$(REVISION)" | grep -v HEAD | awk '{print $$2}' | sed 's|refs/remotes/origin/||' | sed 's|refs/heads/||' | sort | head -n 1)
export BUILT := $(shell date +%Y-%m-%dT%H:%M:%S%z)

BUILD_DIR := $(CURDIR)
TARGET_DIR := $(BUILD_DIR)/out
BUILD_OS_ARCH ?= --arch '!386'

COMMON_PACKAGE ?= github.com/mitchellh/gox/pkg/version
GO_LDFLAGS ?= -X $(COMMON_PACKAGE).NAME=$(NAME) \
			  -X $(COMMON_PACKAGE).VERSION=$(VERSION) \
              -X $(COMMON_PACKAGE).REVISION=$(REVISION) \
              -X $(COMMON_PACKAGE).BUILT=$(BUILT) \
              -X $(COMMON_PACKAGE).BRANCH=$(BRANCH)

.PHONY: all
all: install build_all

.PHONY: help
help:
	# Commands:
	# make all => install and build binaries
	# make version - show information about current version
	#
	# Deployment commands:
	# make install - install to local
	# make build_all - build all supported platforms

.PHONY: version
version:
	@echo Current version: $(VERSION)
	@echo Current revision: $(REVISION)
	@echo Current branch: $(BRANCH)
	@echo Build platforms: $(BUILD_PLATFORMS)

install:
	go install .

build_all:
	@cd $(BUILD_DIR)/ && \
	gox $(BUILD_OS_ARCH) \
		--ldflags "$(GO_LDFLAGS)" \
		--gcflags "all=-trimpath=$(GOPATH)" \
	    --asmflags "all=-trimpath=$(GOPATH)" \
		--output="$(BUILD_DIR)/out/$(NAME)_{{.OS}}_{{.Arch}}"

.PHONY: clean
clean:
	-$(RM) -rf $(TARGET_DIR)
