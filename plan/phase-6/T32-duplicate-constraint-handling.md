# T32 — Duplicate constraint handling (ErrConflict)

## Ticket

**T32** — Duplicate constraint handling

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

`AddUserToGroup`, `GrantUserPermission`, and `GrantGroupPermission` can hit UNIQUE/PK constraint errors when the same membership or grant already exists. After T31, these non-FK constraint errors fall through to `500 Internal Server Error`. They should return `409 Conflict` (or `400`) since they are client-caused.

## Deliverables

- **`store.ErrConflict`** sentinel in `internal/store`.
- **SQLite store** detects UNIQUE/PK constraint violations (`SQLITE_CONSTRAINT_PRIMARYKEY` = 1555, `SQLITE_CONSTRAINT_UNIQUE` = 2067) and wraps them as `ErrConflict`.
- **`writeStoreErr`** maps `ErrConflict` → `409 Conflict`.
- **Tests** for duplicate insert paths in both store and API layers.

## Steps

1. Add `ErrConflict` sentinel to `internal/store/store.go`.
2. Extend `wrapFKError` (or add a sibling helper) in `internal/store/sqlite/store.go` to also detect PK/UNIQUE constraint codes and wrap as `ErrConflict`.
3. Add `ErrConflict` → `409` case to `writeStoreErr` in `internal/api/server.go`.
4. Add store tests: duplicate `AddUserToGroup`, `GrantUserPermission`, `GrantGroupPermission` → `ErrConflict`.
5. Add API tests: duplicate grant/membership → `409`.
6. Run `make test`, `make lint`, `make cover`; verify thresholds.

## Files / paths

- **Edit:** `go/internal/store/store.go`, `go/internal/store/sqlite/store.go`, `go/internal/api/server.go`, `go/internal/api/server_test.go`, `go/internal/store/sqlite/store_test.go`

## Acceptance criteria

- Duplicate `AddUserToGroup` → `409`.
- Duplicate `GrantUserPermission` → `409`.
- Duplicate `GrantGroupPermission` → `409`.
- FK violations still → `400`; internal errors still → `500`.
- `make test` and `make lint` pass; coverage ≥90% total, ≥80% per file.

## Out of scope

- Duplicate handling on `*Create` endpoints (domains, users, groups, resources, etc.).

## Dependencies

- **T31** (handler error classification, `writeStoreErr`, `wrapFKError`).
