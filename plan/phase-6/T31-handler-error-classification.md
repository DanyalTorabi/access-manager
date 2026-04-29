# T31 — Handler error classification

## Ticket

**T31** — Handler error classification (GitHub [#42](https://github.com/DanyalTorabi/access-manager/issues/42))

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Handlers that mutate relationships (`addUserToGroup`, `grantUserPermission`, `grantGroupPermission`) currently map **all** store errors to `400 Bad Request`. Internal failures (closed DB, I/O errors, context cancellation) must return `500`; only client-caused errors (FK violation, duplicate, invalid reference) should return `400`. `groupSetParent` has a partial fix (`ErrNotFound` → `404`) but other non-validation errors still return `400`.

## Deliverables

- **Typed store errors** in `internal/store`: `ErrFKViolation` (or `ErrInvalidReference`) for referencing non-existent entities.
- **SQLite store** wraps constraint-violation errors into the new sentinel(s).
- **Affected handlers** classify errors: FK/constraint → `400`, not-found → `404`, everything else → `500`.
- **Tests** covering each branch: FK violation → `400`, not-found → `404`, internal error → `500`.
- Maintain ≥90% total coverage, ≥80% per file (T30).

## Steps

1. Add sentinel error(s) to `internal/store` (e.g. `ErrFKViolation`).
2. Update `internal/store/sqlite` implementations of `AddUserToGroup`, `GrantUserPermission`, `GrantGroupPermission`, and `GroupSetParent` to detect SQLite FK/constraint failures and wrap them as the new sentinel.
3. Update handlers in `internal/api/server.go`:
   - `addUserToGroup`: `ErrFKViolation` → `400`, else → `500`.
   - `grantUserPermission`: `ErrFKViolation` → `400`, else → `500`.
   - `grantGroupPermission`: `ErrFKViolation` → `400`, else → `500`.
   - `groupSetParent`: confirm `ErrNotFound` → `404`, `ErrFKViolation` → `400`, else → `500`.
4. Add/update unit tests in `server_test.go` and `store_test.go` for each error path.
5. Run `make test`, `make lint`, `make cover`; verify thresholds.

## Files / paths

- **Edit:** `go/internal/store/store.go` (new sentinel), `go/internal/store/sqlite/store.go` (wrap errors), `go/internal/api/server.go` (handler classification), `go/internal/api/server_test.go`, `go/internal/store/sqlite/store_test.go`
- **No new files** expected.

## Acceptance criteria

- `AddUserToGroup` with a non-existent user/group → `400`.
- `GrantUserPermission` with a non-existent user/permission → `400`.
- `GrantGroupPermission` with a non-existent group/permission → `400`.
- Any of the above with a closed/broken DB → `500`.
- Existing `ErrNotFound` paths (`RemoveUserFromGroup`, `RevokeUserPermission`, `RevokeGroupPermission`, `GroupSetParent`) still return `404`.
- `make test` and `make lint` pass; coverage ≥90% total, ≥80% per file.

## Out of scope

- `ErrConflict` / duplicate-key handling (deferred to **T32**).
- Changing the `*Create` handlers (they already handle JSON decode errors as `400`).
- Postgres/MySQL store implementations (T1 deferred).

## Dependencies

- **T10** (unit test patterns), **T30** (coverage thresholds).

## Deferred from other PRs

- **From T45 (#60 / PR #73) review:** external-agent comments **CML4** and **CML6**.
  - **CML4:** `wrapConstraintError` in `go/internal/store/sqlite/store.go` falls back to substring matching of error strings (e.g. `"foreign key constraint failed"`). This is fragile across driver/message changes. Once typed sentinels exist (this ticket + T48), centralise the mapping and remove the substring fallback where the driver exposes a numeric code.
  - **CML6:** delete/exec helpers are inconsistent — some use `wrapConstraintError`, others (e.g. `RemoveUserFromGroup`, `RevokeUserPermission`, `RevokeGroupPermission`) return the raw `err`. Sweep these as part of the typed-error work so all mutating helpers classify errors uniformly.
