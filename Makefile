# Delegate to the Go implementation under go/ (T29). Override: `make -C go test`.
# Docker (T19): run from repo root (compose files live here).
.PHONY: build test cover cover-func run tidy lint docker-build docker-up docker-down docker-logs

build test cover cover-func run tidy lint:
	$(MAKE) -C go $@

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f
