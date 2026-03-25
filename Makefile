.PHONY: build test cover run tidy lint

# Override with `make lint GOLANGCI_LINT=golangci-lint` if you have a local v2 binary (faster).
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

build:
	go build -o bin/server ./cmd/server

test:
	go test -race ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	@echo "coverage.out written; run: go tool cover -html=coverage.out"

run:
	go run ./cmd/server

tidy:
	go mod tidy

lint:
	$(GOLANGCI_LINT) run ./...
