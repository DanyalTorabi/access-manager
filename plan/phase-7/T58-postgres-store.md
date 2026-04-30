# T58 — Implement `internal/store/postgres` (Store interface)

## Ticket

**T58** — Implement `internal/store/postgres` satisfying `store.Store` (GitHub [#93](https://github.com/DanyalTorabi/access-manager/issues/93))

## Parent

**T1** ([#12](https://github.com/DanyalTorabi/access-manager/issues/12)) — [plan/phase-5/T01-per-dialect-migrations.md](../phase-5/T01-per-dialect-migrations.md)

## Phase

**Phase 7** — Multi-DB implementation (sub-tasks of T1)

## Goal

Implement a PostgreSQL-backed `store.Store` in `go/internal/store/postgres/`, following exactly the same interface contract (signatures, error sentinel values, pagination behaviour) as `go/internal/store/sqlite/`.

## Deliverables

- `go/internal/store/postgres/db.go` — `Open(dsn string) (*sql.DB, error)` using `lib/pq` or `pgx/stdlib`
- `go/internal/store/postgres/migrate.go` — `MigrateUp(db *sql.DB, dir string) error` (may reuse the same file-based runner from sqlite or a postgres-aware variant)
- `go/internal/store/postgres/store.go` — full `Store` implementation (~1400 lines, mirroring `sqlite/store.go`)
- `go/internal/store/postgres/store_test.go` — integration tests using a real Postgres DB (via `DATABASE_DSN_POSTGRES` env or `postgres://localhost/access_test`)

## Steps

1. **Dependency**: Add the PostgreSQL driver to `go/go.mod`. Prefer `github.com/lib/pq` (pure Go) unless the team has a preference for `pgx`. Avoid adding both.
2. **Open + migrate**: Port `sqlite/db.go` → `postgres/db.go`. The `Open` function sets sensible connection pool parameters (`SetMaxOpenConns`, `SetConnMaxLifetime`). Port `sqlite/migrate.go` → `postgres/migrate.go`; if the existing runner works with postgres (`pq`), reuse it, otherwise write a minimal file-scanning loop.
3. **Store implementation**: Port all methods from `sqlite/store.go`. Key translation points:
   - `$1, $2, …` placeholders instead of `?`.
   - `INSERT INTO … RETURNING id` for cases where SQLite uses `LastInsertId` (not available in postgres).
   - `SERIAL` / `GENERATED ALWAYS AS IDENTITY` is not used here (IDs are UUIDs/TEXT).
   - Error mapping: `pq.Error` codes → `store.ErrConflict` (code `23505`), `store.ErrFKViolation` (code `23503`), `store.ErrNotFound` (sql.ErrNoRows).
   - `sql.NullString` for nullable `parent_group_id`.
   - Cycle detection in `GroupSetParent`: same bounded loop, dialect-agnostic Go code.
4. **Tests**: Mirror `sqlite/store_test.go` with `newTestStore(t)` that spins up a temporary Postgres schema (use `t.Cleanup` to drop). Skip if `DATABASE_DSN_POSTGRES` env is not set (so unit-only CI still passes). Tag them `//go:build integration` or guard with env skip.
5. Update `go/internal/store/postgres/README.md` (or create one) with a quick "setup for dev" note.

## Acceptance criteria

- `go test -race -tags=integration ./internal/store/postgres/...` passes against a running Postgres 15 instance.
- All `store.Store` methods are implemented (no method stubs returning `ErrNotFound` unconditionally).
- Error sentinels (`ErrConflict`, `ErrFKViolation`, `ErrNotFound`, `ErrInvalidInput`) are correctly mapped from postgres driver errors — confirmed by test assertions.
- `make test` (non-integration) still passes without a running Postgres.

## Files / paths

- **Create:** `go/internal/store/postgres/db.go`, `migrate.go`, `store.go`, `store_test.go`
- **Edit:** `go/go.mod`, `go/go.sum` (add postgres driver)

## Out of scope

- Wiring `database.Open` — that is T60.
- Migration files — that is T56.

## Dependencies

- **T56** — migrations must exist before integration tests can run.
- **T61** — docker-compose postgres service for automated integration-test runs.

## Test strategy

- Use `//go:build integration` build tag so the CI unit job skips these.
- Add a separate `make test-integration-postgres` target in `go/Makefile` (see T61).
- Use `t.TempDir()` is not applicable here; instead create/drop a unique schema per test run using `uuid.New()` or `t.Name()`.
