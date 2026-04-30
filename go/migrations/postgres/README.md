# PostgreSQL migrations (future)

Add versioned DDL here when implementing `internal/store/postgres` and wiring `internal/database.Open` for driver `postgres`.

TODO(T01): Port the SQLite migrations from `../sqlite/`, including
`000003_composite_fk_cross_domain.up.sql` (T51, #77 / PR #83) — composite
UNIQUE on `users`/`groups`/`permissions` and composite FKs on the three
junction tables, with a pre-check that `RAISE EXCEPTION`s on cross-domain
rows (typically inside a `DO` block). See
`plan/phase-5/T01-per-dialect-migrations.md`.
