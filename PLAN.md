# Access manager — project plan

Living summary aligned with the bootstrap design (see repository layout and code).

## Goals

- Go module with **pluggable SQL** (`database/sql`); **SQLite** first via `modernc.org/sqlite`.
- **Domain** scopes all entities (`domain_id` on rows and composite indexes).
- **Permission** = resource + `uint64` access mask (OR of access-type bits). **AccessType** rows define named single-bit flags.
- Users via **groups** only (`group_members`); **group** optional `parent_group_id` (cycle check on write).
- **V1**: No permission inheritance from parent groups; optional later.
- HTTP API on **127.0.0.1** by default; `DATABASE_DRIVER` + `DATABASE_URL` for DSN.

## Layout

- `cmd/server` — config, migrate, HTTP
- `internal/access` — bitmask helpers
- `internal/store` — interfaces
- `internal/store/sqlite` — SQLite implementation
- `internal/api` — HTTP handlers
- `migrations/sqlite` — golang-migrate SQL

## Authz hot path

Effective mask for `(domain_id, user_id, resource_id)`: query all matching `access_mask` rows (direct user permissions + permissions via groups), **OR in Go**. Indexes lead with `domain_id`. Optional future: materialized table or cache.

## Milestones

1. Phase 1 (current): SQLite, CRUD, authz check, tests.
2. Add postgres/mysql drivers + dialect migrations.
3. Optional inheritance and resource trees.
