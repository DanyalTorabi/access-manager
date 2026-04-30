# T57 — Port SQLite migrations to MySQL/MariaDB dialect

## Ticket

**T57** — Port SQLite migrations to MySQL/MariaDB dialect (GitHub [#92](https://github.com/DanyalTorabi/access-manager/issues/92))

## Parent

**T1** ([#12](https://github.com/DanyalTorabi/access-manager/issues/12)) — [plan/phase-5/T01-per-dialect-migrations.md](../phase-5/T01-per-dialect-migrations.md)

## Phase

**Phase 7** — Multi-DB implementation (sub-tasks of T1)

## Goal

Create the three MySQL/MariaDB migration files that mirror the SQLite set, translating all dialect-specific DDL. Primary targets: MySQL 8+ and MariaDB 10.6+.

## Deliverables

- `go/migrations/mysql/000001_init.up.sql` — initial schema
- `go/migrations/mysql/000001_init.down.sql`
- `go/migrations/mysql/000002_restrict_foreign_keys.up.sql` — `RESTRICT` FK behaviour (MySQL's default, verify and make explicit)
- `go/migrations/mysql/000002_restrict_foreign_keys.down.sql`
- `go/migrations/mysql/000003_composite_fk_cross_domain.up.sql` — composite UNIQUE + composite FKs, with `SIGNAL SQLSTATE` pre-check (T51/#77)
- `go/migrations/mysql/000003_composite_fk_cross_domain.down.sql`

## Steps

1. **Translate `000001_init`**: Use `VARCHAR(255)` or `CHAR(36)` for `id` columns (UUIDs are TEXT in SQLite). Use `BIGINT UNSIGNED` for the `bit` column on `access_types`. Add `ENGINE=InnoDB` to all tables to enable FK support. Use backtick quoting.
2. **Translate `000002_restrict_foreign_keys`**: MySQL FK behaviour is `RESTRICT` by default but it must be `ON DELETE RESTRICT` explicitly for clarity. Use `ALTER TABLE … DROP FOREIGN KEY …; ALTER TABLE … ADD CONSTRAINT … FOREIGN KEY … ON DELETE RESTRICT;` per table.
3. **Translate `000003_composite_fk_cross_domain`** (ported from T51/#77 PR #83):
   - Add composite `UNIQUE KEY uq_domain_id (domain_id, id)` to `users`, `groups`, `permissions`.
   - Drop and re-add composite FKs on the three junction tables.
   - Replace `RAISE(ABORT, …)` with a stored procedure/event-free approach using `SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '…';` inside a before-insert trigger, or a stored procedure pre-check — choose whichever is more compatible across MySQL 8 / MariaDB 10.6.
4. Write corresponding `.down.sql` for each migration.
5. Verify migrations apply against a local MySQL 8 container (or via the test helper in T61).

## Acceptance criteria

- `mysql` CLI can apply all three `.up.sql` files against a fresh MySQL 8 database without error.
- Running `.down.sql` files in reverse order leaves a clean database.
- The composite cross-domain pre-check fires as expected.

## Files / paths

- **Create:** `go/migrations/mysql/000001_init.{up,down}.sql`, `000002_restrict_foreign_keys.{up,down}.sql`, `000003_composite_fk_cross_domain.{up,down}.sql`
- **Remove/Update:** `go/migrations/mysql/README.md` — replace TODO with a reference to this ticket being done.

## Out of scope

- The store implementation (`internal/store/mysql`) — that is T59.
- Wiring `database.Open` — that is T60.

## Dependencies

- **T51 (#77)** — composite FK migration already done for SQLite; this ticket ports it.
- **T61** — docker-compose mysql service used for manual and automated verification.

## Notes on MySQL vs SQLite DDL differences

| Concern | SQLite | MySQL |
|-----|-----|-----|
| String type | `TEXT` | `VARCHAR(255)` or `TEXT` |
| `bit` column type | `INTEGER` | `BIGINT UNSIGNED` |
| Boolean | `INTEGER` (0/1) | `TINYINT(1)` or `BOOLEAN` |
| FK support | Requires `PRAGMA foreign_keys = ON` | Requires `ENGINE=InnoDB`; on by default with InnoDB |
| ADD FK to existing table | Rebuild table | `ALTER TABLE … ADD CONSTRAINT … FOREIGN KEY` |
| Trigger pre-check | `RAISE(ABORT, …)` | `SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '…'` in BEFORE INSERT trigger |
| Identifier quoting | `"…"` or backtick | Backtick |
