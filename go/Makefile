.PHONY: build test cover cover-func run tidy lint

# golang/go#75031: GOTOOLCHAIN=auto can make `go test -cover` fail with "no such tool covdata".
# Pin the minimum toolchain to the `go` version in go.mod (override: `make test GOTOOLCHAIN=local`).
GOMOD_GO_VERSION := $(shell sed -n 's/^go //p' go.mod | head -1)
export GOTOOLCHAIN := go$(GOMOD_GO_VERSION)+auto

# Override with `make lint GOLANGCI_LINT=golangci-lint` if you have a local v2 binary (faster).
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

build:
	go build -o bin/server ./cmd/server

test:
	go test -race ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -n 1
	@echo "coverage.out written; run: go tool cover -html=coverage.out"

# Per-package function summary (T12); runs tests via dependency on cover.
cover-func: cover
	go tool cover -func=coverage.out

run:
	go run ./cmd/server

tidy:
	go mod tidy

lint:
	$(GOLANGCI_LINT) run ./...
