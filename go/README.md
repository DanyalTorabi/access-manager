# access-manager (Go)

HTTP service and Go module for **domain-scoped** access control: users, groups, resources, access-type bits, and permissions (`uint64` masks). SQLite is the default store; the design allows other SQL drivers later (**T1**).

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

## Docker

From the **repository root** (not `go/`): **`make docker-build`**, **`make docker-up`**, **`make docker-logs`**, **`make docker-down`** — see root **[README.md](../README.md#docker-t19)** (section **Docker (T19)**).

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
| Lint | `make lint` |
| Tidy modules | `make tidy` |

Docker (from **repo root** only): `make docker-build`, `make docker-up`, `make docker-logs`, `make docker-down` — see [root README](../README.md#docker-t19).

Equivalent without Make (from **`go/`**): `go test -race ./...`, `go vet ./...`, `golangci-lint run ./...`. If `go test -cover` fails with `no such tool "covdata"` on Go 1.25+, set `GOTOOLCHAIN` to `go` plus the `go.mod` version and `+auto` (for example `go1.25.0+auto` when `go.mod` says `go 1.25.0`). The [Makefile](Makefile) exports that for `make test` / `make cover`; see [golang/go#75031](https://github.com/golang/go/issues/75031).

## API overview

REST routes live under **`/api/v1`** with domain-scoped segments, for example:

- `GET /api/v1/domains`
- `POST /api/v1/domains`
- `GET /api/v1/domains/{domainID}/authz/check?user_id=&resource_id=&access_bit=`

Full contract documentation is **T17** (OpenAPI / Postman).

## Package layout

| Path | Purpose |
|------|---------|
| `cmd/server` | Process entry, DB wiring, HTTP server |
| `internal/api` | Chi router and HTTP handlers |
| `internal/access` | Access-mask helpers (library-oriented) |
| `internal/store` | Store interfaces and types |
| `internal/store/sqlite` | SQLite implementation |
| `internal/database` | Driver open + migrations runner |
| `migrations/sqlite` | SQL migrations |

See [AGENTS.md](../AGENTS.md) for contributor rules, security, and **library vs HTTP** boundaries.
