# T1 — Per-dialect migrations (Postgres / MySQL)

## Ticket

**T1** — Per-dialect migrations (see [TICKETS.md](../../TICKETS.md))

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

## Dependencies

- **T19/T13** for CI services; **T26** for DSN config.

## Curriculum link

**Theme 3** — pluggable database backing.
