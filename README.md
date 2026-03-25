# access-manager

HTTP service and Go module for **domain-scoped** access control: users, groups, resources, access-type bits, and permissions (resource + `uint64` mask). SQLite is the default store; the design allows other SQL drivers later (**T1**).

## Prerequisites

- **Go** 1.25+ (see [`go.mod`](go.mod) for the exact `go` directive).

## Quickstart

```bash
git clone <repository-url>
cd access-manager
go mod download
go run ./cmd/server
```

The server listens on **`127.0.0.1:8080`** by default. Migrations run on startup against the configured SQLite file.

### Health check

```bash
curl -s http://127.0.0.1:8080/health
```

Example response: `{"status":"ok"}`

## Configuration (environment)

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_DRIVER` | `sqlite` | SQL driver name (`sqlite` / `sqlite3` via [internal/database](internal/database/open.go)) |
| `DATABASE_URL` | `file:access.db?_pragma=foreign_keys(1)` | DSN for the database |
| `HTTP_ADDR` | `127.0.0.1:8080` | Listen address (**use loopback in dev**; see [AGENTS.md](AGENTS.md)) |
| `MIGRATIONS_DIR` | `migrations/sqlite` | Path to migration `.up.sql` files (resolved relative to process working directory if not absolute) |

Copy [`.env.example`](.env.example) to `.env` for local overrides; do **not** commit real secrets.

**Structured config file** (YAML/TOML + env overrides) is planned under **T26**—until then, use environment variables only.

## Development

`make lint` runs **golangci-lint v2** via `go run` (first run may download modules). For a faster local binary, install with Go **1.25+** and override: `make lint GOLANGCI_LINT=golangci-lint`.

| Target | Command |
|--------|---------|
| Build binary | `make build` → `bin/server` |
| Tests (race) | `make test` |
| Coverage profile | `make cover` → `coverage.out` |
| Run server | `make run` |
| Lint | `make lint` |
| Tidy modules | `make tidy` |

Equivalent without Make: `go test -race ./...`, `go vet ./...`, `golangci-lint run ./...`.

## API overview

REST routes live under **`/api/v1`** with domain-scoped segments, for example:

- `GET /api/v1/domains`
- `POST /api/v1/domains`
- `GET /api/v1/domains/{domainID}/authz/check?user_id=&resource_id=&access_bit=`

Full contract documentation is **T17** (OpenAPI / Postman).

## Repository layout

| Path | Purpose |
|------|---------|
| `cmd/server` | Process entry, DB wiring, HTTP server |
| `internal/api` | Chi router and HTTP handlers |
| `internal/access` | Access-mask helpers (library-oriented) |
| `internal/store` | Store interfaces and types |
| `internal/store/sqlite` | SQLite implementation |
| `internal/database` | Driver open + migrations runner |
| `migrations/sqlite` | SQL migrations |

See [AGENTS.md](AGENTS.md) for contributor rules, security, and **library vs HTTP** boundaries.

## Docs and planning

- [PLAN.md](PLAN.md) — product goals and milestones  
- [TICKETS.md](TICKETS.md) — backlog and curriculum alignment table  
- [plan/README.md](plan/README.md) — phased implementation plans per ticket  

## License

Add a `LICENSE` file when you publish the repository.