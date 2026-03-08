# Go command to use for build
GO ?= go
INSTALL ?= install

LDFLAGS := $(shell go run -mod=readonly go.zoe.im/x/version/gen)

ifneq "$(strip $(shell command -v $(GO) 2>/dev/null))" ""
	GOOS ?= $(shell $(GO) env GOOS)
	GOARCH ?= $(shell $(GO) env GOARCH)
else
	ifeq ($(GOOS),)
		ifeq ($(OS),Windows_NT)
			GOOS = windows
		else
			UNAME_S := $(shell uname -s)
			ifeq ($(UNAME_S),Linux)
				GOOS = linux
			endif
			ifeq ($(UNAME_S),Darwin)
				GOOS = darwin
			endif
			ifeq ($(UNAME_S),FreeBSD)
				GOOS = freebsd
			endif
		endif
	else
		GOOS ?= $$GOOS
		GOARCH ?= $$GOARCH
	endif
endif

ifndef GODEBUG
	EXTRA_LDFLAGS += -s -w
	DEBUG_GO_GCFLAGS :=
	DEBUG_TAGS :=
else
	DEBUG_GO_GCFLAGS := -gcflags=all="-N -l"
	DEBUG_TAGS := static_build
endif

GO_BUILD_FLAGS = --ldflags '${LDFLAGS}'
GO_GCFLAGS=$(shell				\
	set -- ${GOPATHS};			\
	echo "-gcflags=-trimpath=$${1}/src";	\
	)

WHALE = "🇩"
ONI = "👹"

# Project packages.
PACKAGES=$(shell $(GO) list ${GO_TAGS} ./... | grep -v /vendor/ | grep -v /integration)

# Project binaries.
COMMANDS ?= $(shell ls -d ./cmd/*/ | sed 's|./cmd/||'|sed 's|/||')

BINARIES=$(addprefix bin/,$(COMMANDS))

FORCE:

define BUILD_BINARY
@echo "$(WHALE) $@"
$(GO) build ${DEBUG_GO_GCFLAGS} ${GO_GCFLAGS} ${GO_BUILD_FLAGS} -o $@ ${GO_LDFLAGS} ${GO_TAGS}  ./$<
endef

.PHONY: all build binaries test clean docker help release
.DEFAULT: default
.DEFAULT_GOAL := all

# Build a binary from a cmd.
bin/%: cmd/% FORCE
	$(call BUILD_BINARY)

all: binaries

build: binaries ## alias for binaries

binaries: $(BINARIES) ## build binaries
	@echo "$(WHALE) $@"

test:
	@echo "Execute test"
	@go test ./...

release: ## create release with goreleaser
	goreleaser release --clean

clean:
	rm -rf bin/ dist/

docker-sandbox:
	docker build -t agentbox-sandbox:latest -f deploy/sandbox/Dockerfile .

help:
	@echo "make                  build all binaries"
	@echo "make build            build all binaries"
	@echo "make test             execute tests"
	@echo "make release          create release with goreleaser"
	@echo "make clean            remove build artifacts"
	@echo "make docker-sandbox   build sandbox container image"
