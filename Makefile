.DEFAULT_GOAL := build

GO ?= go
VERSION ?= dev
BIN := bin/yoinkdl$(shell $(GO) env GOEXE)

.PHONY: build test vet race fmt fmt-check check release help

build:
	mkdir -p bin
	$(GO) build -trimpath -ldflags '-X main.version=$(VERSION)' -o $(BIN) ./cmd/yoinkdl

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

race:
	$(GO) test -race ./...

fmt:
	gofmt -w cmd internal tools

fmt-check:
	@set -eu; files=$$(gofmt -l cmd internal tools); if [ -n "$$files" ]; then printf '%s\n' "$$files"; exit 1; fi

check: fmt-check vet test race build

release: export RELEASE_VERSION = $(VERSION)
release: check
	bash tools/release.sh "$$RELEASE_VERSION"

help:
	@printf '%s\n' 'make build      Build bin/yoinkdl' 'make test       Run tests' 'make vet        Run static checks' 'make race       Run tests with the race detector' 'make fmt        Format Go source' 'make fmt-check  Check formatting' 'make check      Run all checks and build' 'make release    Check and package Linux/amd64 (VERSION required)' 'VERSION=...     Override the build version (default: dev)'
