# Delegate to the Go implementation under go/ (T29). Override: `make -C go test`.
# Docker (T19): run from repo root (compose files live here).
.PHONY: build test cover cover-func run tidy lint e2e e2e-bash docker-build docker-up docker-down docker-logs

build test cover cover-func run tidy lint:
	$(MAKE) -C go $@

# T16 — same as go/Makefile e2e: go test -race -count=1 -tags=e2e ./e2e/... (BASE_URL default http://127.0.0.1:8080).
e2e:
	$(MAKE) -C go e2e

# Same journey as e2e, using curl+jq (optional; pick one long-term).
e2e-bash:
	bash ./test/e2e/bash/run.sh

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f
