# T33 — Restrict cascading deletes

## Ticket

**T33** — Restrict cascading deletes (GitHub [#47](https://github.com/DanyalTorabi/access-manager/issues/47) scope, paired with T37)

## Phase

**Phase 5** — P2 polish

## Goal

Replace the initial `ON DELETE CASCADE` foreign keys with `ON DELETE RESTRICT` so that deleting an entity while it is still referenced returns a clear error instead of silently removing dependents.

## Deliverables

- **Migration `000002_restrict_foreign_keys`** that rebuilds affected tables with `RESTRICT` FKs (SQLite requires table recreation).
- **`wrapConstraintError`** in `internal/store/sqlite` handles FK failures from the driver even when the error does not unwrap to `*sqlite.Error` (string fallback for `database/sql` wrapper).
- **Store tests** proving that deleting a domain/group/resource/user/access-type/permission with live dependents returns `store.ErrFKViolation`.

## Steps

1. Write `go/migrations/sqlite/000002_restrict_foreign_keys.up.sql` — for each table with cascading FKs, `CREATE TABLE … _new` with `ON DELETE RESTRICT`, copy data, drop old, rename.
2. Extend `wrapConstraintError` in `go/internal/store/sqlite/store.go` with a `strings.Contains` fallback for `"foreign key constraint failed"` (covers cases where `errors.As(*sqlite.Error)` fails through `database/sql`).
3. Add store-level tests in `go/internal/store/sqlite/store_test.go`: delete with dependents → `store.ErrFKViolation`; delete without dependents → success.
4. Run `make test`, `make lint`.

## Files / paths

- **New:** `go/migrations/sqlite/000002_restrict_foreign_keys.up.sql`
- **Edit:** `go/internal/store/sqlite/store.go`, `go/internal/store/sqlite/store_test.go`

## Acceptance criteria

- Deleting a domain that has users/groups/resources → 400 (`ErrFKViolation`), not silent cascade.
- Deleting a user/group/resource/permission/access-type with dependents → same.
- Deleting entities with no remaining dependents → success.
- `make test` and `make lint` pass; coverage ≥80% per file.

## Dependencies

- **T31** (`wrapConstraintError`, `ErrFKViolation` sentinel).
