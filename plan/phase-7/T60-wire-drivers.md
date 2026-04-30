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
- `go/internal/database/open.go` — `MigrateUp` must dispatch to the correct dialect-specific migrator when called
- `go/.env.example` — add commented-out Postgres and MySQL DSN examples
- `go/config.example.yaml` — add commented DSN options for each driver
- `go/README.md` — document the supported drivers and sample DSN format
- Root `README.md` — update the Database section if it mentions only SQLite

## Steps

1. **Extend `Open`**: Add `"postgres"` and `"mysql"` cases that call `pgstore.Open(dsn)` and `mysqlstore.Open(dsn)` respectively, returning the appropriate `*sql.DB` and migrations directory path (`"migrations/postgres"` / `"migrations/mysql"`).
2. **`MigrateUp` dispatch**: The current `MigrateUp` calls `sqlstore.MigrateUp`. Either:
   - Accept the `*sql.DB` and dialect string and route to the correct package's `MigrateUp`, or
   - Unify migration logic into a single file-based runner that works across dialects (preferred, since all three dialects now share the same runner contract — see T58/T59 for notes on multi-statement MySQL support).
3. **Error messages**: Extend the "unsupported driver" error message to list all three now-supported drivers.
4. **Config docs**: Update `go/.env.example` with:
   ```
   # DATABASE_DRIVER=postgres
   # DATABASE_DSN=postgres://localhost:5432/access_manager?sslmode=disable
   # DATABASE_DRIVER=mysql
   # DATABASE_DSN=root:@tcp(localhost:3306)/access_manager?parseTime=true
   ```
5. **README**: Add a "Supported databases" section or update the existing one with driver names and minimum version requirements (PostgreSQL 15+, MySQL 8+ / MariaDB 10.6+).
6. **Tests**: Add a unit test in `go/internal/database/` that verifies `Open` returns a non-nil error for `"baddriver"` and a different branch test that exercises `Open` with `"sqlite"` still works (existing test should be extended, not replaced).

## Acceptance criteria

- `DATABASE_DRIVER=postgres DATABASE_DSN=…` starts the server, runs migrations, and responds to `GET /domains` with `[]`.
- `DATABASE_DRIVER=mysql DATABASE_DSN=…` does the same.
- `DATABASE_DRIVER=unknown` returns a clear error listing all supported drivers.
- `make test` passes (the `database` package unit test does not require running DB instances).

## Files / paths

- **Edit:** `go/internal/database/open.go`
- **Edit:** `go/.env.example`, `go/config.example.yaml`, `go/README.md`, `README.md`

## Out of scope

- Store implementations — T58, T59.
- Migration SQL files — T56, T57.
- CI integration test targets — T61.

## Dependencies

- **T56, T57** — Postgres/MySQL migration dirs must exist.
- **T58, T59** — Store packages must exist for `Open` to import.
