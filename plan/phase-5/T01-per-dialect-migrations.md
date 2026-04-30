# T1 — Per-dialect migrations (Postgres / MySQL)

## Ticket

**T1** — Per-dialect migrations (GitHub [#12](https://github.com/DanyalTorabi/access-manager/issues/12))

## Phase

**Phase 5** — P2 polish and multi-DB

## Goal

Extend [internal/database/open.go](../../go/internal/database/open.go) and add **Postgres** and/or **MySQL** drivers, **dialect-specific migrations**, and store implementations that satisfy [internal/store/store.go](../../go/internal/store/store.go).

## Deliverables

- `migrations/postgres/*.sql`, `migrations/mysql/*.sql` (or tool-based migrations).
- `internal/store/postgres/` and/or `internal/store/mysql/` packages.
- `database.Open` switch extended; DSN documented in README.

## Steps

1. Port SQLite DDL with dialect tweaks (SERIAL/BIGINT, boolean, FK syntax).
2. Reuse same `store.Store` interface; factor shared SQL where possible or duplicate with tests.
3. Run integration tests against each dialect (compose services in T19/T13).
4. Update `.env.example` with sample DSNs (no real credentials).

## Files / paths

- **Create:** `internal/store/postgres/`, `internal/store/mysql/`, migration files
- **Edit:** [internal/database/open.go](../../go/internal/database/open.go), [README.md](../../README.md)

## Acceptance criteria

- `DATABASE_DRIVER=postgres` (or mysql) runs migrations and passes store integration tests.

## Out of scope

- Cockroach-specific quirks unless required; read replicas.

## Pending follow-ups from other tickets

- **T51 (#77 / PR #83):** Composite-FK migration for cross-domain
  protection. Implemented in `go/migrations/sqlite/000003_composite_fk_cross_domain.up.sql`;
  the MySQL/PostgreSQL equivalents are deferred here. When this ticket
  lands, port the migration (composite UNIQUE on `users`/`groups`/
  `permissions` and composite FKs on the three junction tables) and
  carry over the `RAISE(ABORT, ...)` pre-check in dialect-appropriate
  form (PostgreSQL: `RAISE EXCEPTION` in a DO block; MySQL: `SIGNAL
  SQLSTATE`).

## Dependencies

- **T19/T13** for CI services; **T26** for DSN config.

## Curriculum link

**Theme 3** — pluggable database backing.
