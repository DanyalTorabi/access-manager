# T60 — Wire Postgres & MySQL into `database.Open`; DSN config and README

## Ticket

**T60** — Wire Postgres & MySQL into `database.Open`; update DSN config and docs (GitHub [#95](https://github.com/DanyalTorabi/access-manager/issues/95))

## Parent

**T1** ([#12](https://github.com/DanyalTorabi/access-manager/issues/12)) — [plan/phase-5/T01-per-dialect-migrations.md](../phase-5/T01-per-dialect-migrations.md)

## Phase

**Phase 7** — Multi-DB implementation (sub-tasks of T1)

## Goal

Extend `go/internal/database/open.go` to dispatch to `internal/store/postgres` and `internal/store/mysql` when those drivers are selected via `DATABASE_DRIVER`, and update all DSN configuration docs and `.env.example` so users can start either backend.

## Deliverables

- `go/internal/database/open.go` — add `"postgres"` and `"mysql"` cases to the `Open` switch
- `go/internal/database/open.go` — `MigrateUp` signature extended to `MigrateUp(db *sql.DB, migrationsDir, driver string) error` to dispatch to the correct dialect-specific migrator
- `go/cmd/server/main.go` — update `setup()` to pass driver to `MigrateUp` and select the correct store implementation (postgres/mysql/sqlite) based on `cfg.DatabaseDriver`
- `go/.env.example` — add commented-out Postgres and MySQL DSN examples
- `go/config.example.yaml` — add commented DSN options for each driver
- `go/README.md` — document the supported drivers and sample DSN format
- Root `README.md` — update the Database section if it mentions only SQLite

## Steps

1. **Extend `Open`**: Add `"postgres"` and `"mysql"` cases that call `pgstore.Open(dsn)` and `mysqlstore.Open(dsn)` respectively, returning the appropriate `*sql.DB` and migrations directory path (`"migrations/postgres"` / `"migrations/mysql"`).
2. **`MigrateUp` dispatch**: Change `MigrateUp` to `MigrateUp(db *sql.DB, migrationsDir, driver string) error` and route to `sqlstore.MigrateUp`, `pgstore.MigrateUp`, or `mysqlstore.MigrateUp` based on the driver string. Update all callers (`cmd/server/main.go`, `open_test.go`).
3. **`cmd/server` store wiring**: Update `setup()` to select `pgstore.New(db)`, `mysqlstore.New(db)`, or `sqlstore.New(db)` based on `cfg.DatabaseDriver`, and wire `SetNegativeMaskHook` via a local `maskHookSetter` interface type assertion so all three store types are supported.
4. **Error messages**: Extend the "unsupported driver" error message to list all three now-supported drivers.
5. **Config docs**: Update `go/.env.example` with:
   ```
   # DATABASE_DRIVER=postgres
   # DATABASE_URL=postgres://localhost:5432/access_manager?sslmode=disable
   # DATABASE_DRIVER=mysql
   # DATABASE_URL=root:@tcp(localhost:3306)/access_manager?parseTime=true
   ```
6. **README**: Add a "Supported databases" section or update the existing one with driver names and minimum version requirements (PostgreSQL 15+, MySQL 8+ / MariaDB 10.6+).
7. **Tests**: Update existing `TestMigrateUp_sqlite` to pass the driver argument. Add tests for `Open("postgres", ...)` and `Open("mysql", ...)` returning ping errors (no running DB needed — just verify routing). Extend `TestOpen_unsupportedDriver` to verify the error lists all supported drivers.

## Acceptance criteria

- `DATABASE_DRIVER=postgres DATABASE_DSN=…` starts the server, runs migrations, and responds to `GET /domains` with `[]`.
- `DATABASE_DRIVER=mysql DATABASE_DSN=…` does the same.
- `DATABASE_DRIVER=unknown` returns a clear error listing all supported drivers.
- `make test` passes (the `database` package unit test does not require running DB instances).

## Files / paths

- **Edit:** `go/internal/database/open.go`
- **Edit:** `go/cmd/server/main.go`
- **Edit:** `go/.env.example`, `go/config.example.yaml`, `go/README.md`, `README.md`

## Out of scope

- Store implementations — T58, T59.
- Migration SQL files — T56, T57.
- CI integration test targets — T61.

## Deferred follow-ups

- **Driver-dispatch consolidation (three switches):** `database.Open`, `database.MigrateUp`, and `setup()` each independently switch on the driver string. Adding a new dialect requires changes in three places. Introducing a shared `driverConfig` struct or collapsing store selection into `database.Open` would create a single authoritative list. Deferred — track in a new issue.
- **`internal/database` layering violation:** `open.go` imports all three store packages (`internal/store/sqlite`, `internal/store/postgres`, `internal/store/mysql`), which registers all three drivers at `init()` time regardless of which driver is configured. This inflates the binary and couples the low-level database package to higher-level store implementations. Fixing this would require a refactor (e.g., moving driver-registration-only sub-packages or separating `sql.Open` from store construction). Deferred — track in a new issue.

## Dependencies

- **T56, T57** — Postgres/MySQL migration dirs must exist.
- **T58, T59** — Store packages must exist for `Open` to import.
