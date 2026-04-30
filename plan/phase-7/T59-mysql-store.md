# T59 — Implement `internal/store/mysql` (Store interface)

## Ticket

**T59** — Implement `internal/store/mysql` satisfying `store.Store` (GitHub [#94](https://github.com/DanyalTorabi/access-manager/issues/94))

## Parent

**T1** ([#12](https://github.com/DanyalTorabi/access-manager/issues/12)) — [plan/phase-5/T01-per-dialect-migrations.md](../phase-5/T01-per-dialect-migrations.md)

## Phase

**Phase 7** — Multi-DB implementation (sub-tasks of T1)

## Goal

Implement a MySQL-backed `store.Store` in `go/internal/store/mysql/`, following exactly the same interface contract (signatures, error sentinel values, pagination behaviour) as `go/internal/store/sqlite/`.

## Deliverables

- `go/internal/store/mysql/db.go` — `Open(dsn string) (*sql.DB, error)` using `github.com/go-sql-driver/mysql`
- `go/internal/store/mysql/migrate.go` — `MigrateUp(db *sql.DB, dir string) error`
- `go/internal/store/mysql/store.go` — full `Store` implementation mirroring `sqlite/store.go`
- `go/internal/store/mysql/store_test.go` — integration tests guarded by `DATABASE_DSN_MYSQL` env

## Steps

1. **Dependency**: Add `github.com/go-sql-driver/mysql` to `go/go.mod`.
2. **Open + migrate**: Port `sqlite/db.go` → `mysql/db.go`. The DSN format is `user:password@tcp(host:port)/dbname?parseTime=true&multiStatements=true`. Set sensible pool parameters. Port the file-based migrator; MySQL supports multi-statement execution if `multiStatements=true` is in the DSN.
3. **Store implementation**: Port all methods from `sqlite/store.go`. Key translation points:
   - `?` placeholders are the same as SQLite — no change needed.
   - `INSERT … ON DUPLICATE KEY UPDATE id=id` is a common no-op upsert guard; prefer returning `store.ErrConflict` via error mapping.
   - Error mapping: `mysql.MySQLError` number `1062` → `store.ErrConflict`, number `1451`/`1452` → `store.ErrFKViolation`, `sql.ErrNoRows` → `store.ErrNotFound`.
   - `LAST_INSERT_ID()` is available but IDs are TEXT (UUIDs), so `LastInsertId` is not used — IDs are generated in Go before insertion (same as SQLite).
   - `sql.NullString` for nullable `parent_group_id`.
   - Cycle detection in `GroupSetParent`: same bounded loop, dialect-agnostic.
   - `BIGINT UNSIGNED` for the `bit` column — map to `uint64` in Go (same as SQLite `INTEGER` → `uint64`).
4. **Tests**: Mirror `sqlite/store_test.go` with `newTestStore(t)` that creates a fresh database per run (drop + recreate using a unique name from `t.Name()`). Skip if `DATABASE_DSN_MYSQL` env is not set. Tag with `//go:build integration`.
5. Enable FK checks explicitly: `SET FOREIGN_KEY_CHECKS=1` in the `Open` connection setup (InnoDB has FKs enabled by default, but be explicit for clarity).

## Acceptance criteria

- `go test -race -tags=integration ./internal/store/mysql/...` passes against a running MySQL 8 instance.
- All `store.Store` methods are implemented.
- Error sentinels are correctly mapped and verified by test assertions (conflict, FK violation, not-found, invalid-input).
- `make test` (non-integration) still passes without a running MySQL.

## Files / paths

- **Create:** `go/internal/store/mysql/db.go`, `migrate.go`, `store.go`, `store_test.go`
- **Edit:** `go/go.mod`, `go/go.sum` (add MySQL driver)

## Out of scope

- Wiring `database.Open` — that is T60.
- Migration files — that is T57.

## Dependencies

- **T57** — migrations must exist before integration tests can run.
- **T61** — docker-compose mysql service for automated integration-test runs.

## Test strategy

- Use `//go:build integration` build tag so the CI unit job skips these.
- Add a `make test-integration-mysql` target in `go/Makefile` (see T61).
- Create a unique database per test suite; drop it in `t.Cleanup`.
