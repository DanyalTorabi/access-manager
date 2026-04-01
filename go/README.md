# access-manager (Go)

HTTP service and Go module for **domain-scoped** access control: users, groups, resources, access-type bits, and permissions (`uint64` masks). SQLite is the default store; the design allows other SQL drivers later.

**Module:** `github.com/dtorabi/access-manager` (this directory is the **module root**).

## Prerequisites

- **Go** 1.25+ (see [`go.mod`](go.mod) for the exact `go` directive).

## Quickstart

From **`go/`** (or use `make run` from the **repository root**):

```bash
go mod download
go run ./cmd/server
```

The server listens on **`127.0.0.1:8080`** by default. Migrations run on startup against the configured SQLite file. **SIGINT** / **SIGTERM** triggers graceful shutdown (see `SHUTDOWN_TIMEOUT_SECONDS` / `shutdown_timeout_seconds` in config).

### Health check

```bash
curl -s http://127.0.0.1:8080/health
```

Example response: `{"status":"ok"}`

### Metrics

Prometheus metrics are served at **`/metrics`** (outside bearer auth). The middleware records `http_requests_total`, `http_request_duration_seconds`, and `authz_checks_total`. See root [README.md — Observability](../README.md#observability) for Grafana/Prometheus compose setup.

## Docker

From the **repository root** (not `go/`): **`make docker-build`**, **`make docker-up`**, **`make docker-logs`**, **`make docker-down`** — see root **[README.md](../README.md#docker)** (section **Docker**).

## Configuration

**Precedence:** built-in defaults → optional YAML file (if `CONFIG_PATH` is set) → **environment variables** (each non-empty env var overrides the corresponding field).

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CONFIG_PATH` | _(unset)_ | Path to YAML file; if unset, only defaults + env apply |
| `DATABASE_DRIVER` | `sqlite` | SQL driver (`sqlite` / `sqlite3` via [internal/database](internal/database/open.go)) |
| `DATABASE_URL` | `file:access.db?_pragma=foreign_keys(1)` | Database DSN |
| `HTTP_ADDR` | `127.0.0.1:8080` | Listen address (**use loopback in dev**; see [AGENTS.md](../AGENTS.md)) |
| `MIGRATIONS_DIR` | `migrations/sqlite` | Migration `.up.sql` directory (relative paths are resolved from the **process working directory** — run from `go/` or set an absolute path) |
| `SHUTDOWN_TIMEOUT_SECONDS` | `30` | Max seconds to wait for graceful shutdown after **SIGINT** / **SIGTERM** |
| `API_BEARER_TOKEN` | _(empty)_ | If set, all **`/api/v1/*`** routes require `Authorization: Bearer <token>`. **`/health`** stays public. Use a strong random value in production; never commit it. |

Copy [`.env.example`](.env.example) to `.env` for local overrides; do **not** commit real secrets.

### YAML file (optional)

See [config.example.yaml](config.example.yaml). Copy to a path outside VCS (e.g. `config.local.yaml`, gitignored at repo root) and set `CONFIG_PATH` to that path.

Loader: [internal/config](internal/config/config.go).

## Development

Run **`make`** from the **repository root** (`make test`, `make lint`, …) or from this directory with the same targets (see [Makefile](Makefile)).

`make lint` runs **golangci-lint v2** via `go run` (first run may download modules). For a faster local binary, install with Go **1.25+** and override: `make lint GOLANGCI_LINT=golangci-lint`.

| Target | Command |
|--------|---------|
| Build binary | `make build` → `bin/server` |
| Tests (race) | `make test` |
| Coverage profile | `make cover` → `coverage.out`, prints total statement coverage; HTML: `go tool cover -html=coverage.out` |
| Coverage by function | `make cover-func` |
| Run server | `make run` |
| E2E smoke | From **repo root**: `make e2e` → `go test -race -count=1 -tags=e2e ./e2e/...` (running server; optional **`API_BEARER_TOKEN`**). Optional curl script: **`make e2e-bash`**. See **[test/e2e/README.md](../test/e2e/README.md)**. |
| Lint | `make lint` |
| Vuln check | `make vuln` → pinned `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...` (same pin as CI) |
| Tidy modules | `make tidy` |

Docker (from **repo root** only): `make docker-build`, `make docker-up`, `make docker-logs`, `make docker-down` — see [root README](../README.md#docker).

Equivalent without Make (from **`go/`**): `go test -race ./...`, `go vet ./...`, `golangci-lint run ./...`. If `go test -cover` fails with `no such tool "covdata"` on Go 1.25+, set `GOTOOLCHAIN` to `go` plus the `go.mod` version and `+auto` (for example `go1.25.0+auto` when `go.mod` says `go 1.25.0`). The [Makefile](Makefile) exports that for `make test` / `make cover`; see [golang/go#75031](https://github.com/golang/go/issues/75031).

## API overview

REST routes live under **`/api/v1`** with domain-scoped segments, for example:

- `GET` / `POST /api/v1/domains`; `GET` / `PATCH` / `DELETE /api/v1/domains/{domainID}`
- Domain-scoped CRUD includes `PATCH` / `DELETE` for users, groups, resources, permissions, and access types (plus `GET` for a single access type). Deletes fail with **400** when SQLite foreign keys block removal.
- `GET /api/v1/domains/{domainID}/authz/check?user_id=&resource_id=&access_bit=`

### Authentication

When **`API_BEARER_TOKEN`** (or YAML **`api_bearer_token`**) is non-empty, clients must send:

`Authorization: Bearer <token>`

on **`/api/v1/*`** requests. **`/health`** is not protected. The service compares the presented token to the configured secret using **SHA-256** digests and a **constant-time** equality check (you still send the plain token in the header). If the token is unset and **`HTTP_ADDR`** binds beyond loopback (for example **`0.0.0.0:8080`** or **`:8080`**), the server logs a one-time warning at startup. JWT/JWKS validation is planned as a future enhancement.

Full HTTP contract: **[`api/openapi.yaml`](../api/openapi.yaml)** (OpenAPI 3) and **[`api/postman/access-manager.postman_collection.json`](../api/postman/access-manager.postman_collection.json)**. Import steps and **`baseUrl`** / **`bearerToken`** variables: **[`api/README.md`](../api/README.md)**.

### Structured logging & audit

Server output is structured JSON via `internal/logger` (wrapping `log/slog`). All mutation handlers (creates, updates, deletes, membership/grant changes) emit audit events with `audit=true`, the action name, and relevant entity IDs. Example:

```json
{"time":"...","level":"INFO","msg":"audit","audit":true,"action":"grant_user_permission","domain_id":"d1","user_id":"u1","permission_id":"p1"}
```

## Package layout

| Path | Purpose |
|------|---------|
| `cmd/server` | Process entry, DB wiring, HTTP server |
| `internal/api` | Chi router and HTTP handlers |
| `internal/access` | Access-mask helpers (library-oriented) |
| `internal/store` | Store interfaces and types |
| `internal/store/sqlite` | SQLite implementation |
| `internal/logger` | Structured JSON logger wrapping `log/slog`; audit events |
| `internal/database` | Driver open + migrations runner |
| `migrations/sqlite` | SQL migrations |

See [AGENTS.md](../AGENTS.md) for contributor rules, security, and **library vs HTTP** boundaries.
