# T56 — Port SQLite migrations to PostgreSQL dialect

## Ticket

**T56** — Port SQLite migrations to PostgreSQL dialect (GitHub [#91](https://github.com/DanyalTorabi/access-manager/issues/91))

## Parent

**T1** ([#12](https://github.com/DanyalTorabi/access-manager/issues/12)) — [plan/phase-5/T01-per-dialect-migrations.md](../phase-5/T01-per-dialect-migrations.md)

## Phase

**Phase 7** — Multi-DB implementation (sub-tasks of T1)

## Goal

Create the three PostgreSQL migration files that mirror the SQLite set, translating all dialect-specific DDL. The migration tool should remain the same file-based runner already used for SQLite (or a compatible postgres-aware variant).

## Deliverables

- `go/migrations/postgres/000001_init.up.sql` — initial schema
- `go/migrations/postgres/000001_init.down.sql`
- `go/migrations/postgres/000002_restrict_foreign_keys.up.sql` — ALTER TABLE / recreate tables with RESTRICT
- `go/migrations/postgres/000002_restrict_foreign_keys.down.sql`
- `go/migrations/postgres/000003_composite_fk_cross_domain.up.sql` — composite UNIQUE + composite FKs, with `DO $$...RAISE EXCEPTION` pre-check (T51/#77)
- `go/migrations/postgres/000003_composite_fk_cross_domain.down.sql`

## Steps

1. **Translate `000001_init`**: Replace SQLite-ism `TEXT PRIMARY KEY` with `TEXT PRIMARY KEY` (same, fine), ensure `INTEGER` → `BIGINT` for `bit` column on `access_types`, keep the nine indexes.
2. **Translate `000002_restrict_foreign_keys`**: PostgreSQL allows `ALTER TABLE … ALTER CONSTRAINT` or `DROP CONSTRAINT … ADD CONSTRAINT … ON DELETE RESTRICT`. Use `ALTER TABLE` rather than the SQLite rebuild-and-copy pattern (SQLite cannot `ALTER` FKs, PostgreSQL can).
3. **Translate `000003_composite_fk_cross_domain`** (ported from T51/#77 PR #83):
   - Add composite `UNIQUE (domain_id, id)` to `users`, `groups`, `permissions` via `ALTER TABLE`.
   - Drop and re-add composite FKs on the three junction tables referencing `(domain_id, user_id/group_id/permission_id)`.
   - Replace `RAISE(ABORT, …)` SQLite trigger with a PostgreSQL `DO $$ BEGIN … IF EXISTS(…) THEN RAISE EXCEPTION '…'; END IF; END $$;` pre-check block.
4. Write a corresponding `.down.sql` for each migration that reverses the changes cleanly.
5. Verify migrations apply in order with `psql` against a local Postgres container (or via the test helper in T61).

## Acceptance criteria

- `psql` can apply all three `.up.sql` files against a fresh PostgreSQL 15+ database without error.
- Running them twice (idempotent via `IF NOT EXISTS` guards where applicable) does not fail.
- Running `.down.sql` files in reverse order leaves an empty database without error.
- The composite cross-domain pre-check fires as expected (insert a cross-domain row → error).

## Files / paths

- **Create:** `go/migrations/postgres/000001_init.{up,down}.sql`, `000002_restrict_foreign_keys.{up,down}.sql`, `000003_composite_fk_cross_domain.{up,down}.sql`
- **Remove/Update:** `go/migrations/postgres/README.md` — replace TODO with a reference to this ticket being done.

## Out of scope

- The store implementation (`internal/store/postgres`) — that is T58.
- Wiring `database.Open` — that is T60.

## Dependencies

- **T51 (#77)** — composite FK migration already done for SQLite; this ticket ports it.
- **T61** — docker-compose postgres service used for manual and automated verification.

## Notes on Postgres vs SQLite DDL differences

| Concern | SQLite | PostgreSQL |
|-----|-----|-----|
| Auto-increment PK | `INTEGER PRIMARY KEY` | `BIGSERIAL PRIMARY KEY` or `GENERATED ALWAYS AS IDENTITY` |
| `bit` column type | `INTEGER` | `BIGINT` |
| Boolean | `INTEGER` (0/1) | `BOOLEAN` |
| ADD FK to existing table | Rebuild table (no ALTER) | `ALTER TABLE … ADD CONSTRAINT … FOREIGN KEY` |
| Trigger-like pre-check | `RAISE(ABORT, …)` in trigger | `DO $$ BEGIN RAISE EXCEPTION '…'; END $$` |
| FK `RESTRICT` | Default with pragma | Explicit `ON DELETE RESTRICT` |
