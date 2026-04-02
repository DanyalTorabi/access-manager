# T37 — Delete and update (PATCH) endpoints

## Ticket

**T37** — Delete and update (PATCH) endpoints (GitHub [#48](https://github.com/DanyalTorabi/access-manager/issues/48))

## Phase

**Phase 5** — P2 polish

## Goal

Add `DELETE` and `PATCH` (partial update) routes for all CRUD entities plus `GET /domains/{domainID}` and `GET /access-types/{accessTypeID}`, with audit logging, OpenAPI/Postman alignment, and tests.

## Deliverables

- **Store interface** (`internal/store`): `Delete*`, `Patch*`, and `AccessTypeGet` methods with `*PatchParams` types.
- **SQLite store** implementation of the above; not-found → `ErrNotFound`, FK/constraint → `wrapConstraintError`.
- **API routes** in `internal/api/server.go`:
  - `GET /domains/{domainID}`, `PATCH /domains/{domainID}`, `DELETE /domains/{domainID}`
  - `PATCH /users/{userID}`, `DELETE /users/{userID}`
  - `PATCH /groups/{groupID}`, `DELETE /groups/{groupID}`
  - `PATCH /resources/{resourceID}`, `DELETE /resources/{resourceID}`
  - `PATCH /access-types/{accessTypeID}`, `DELETE /access-types/{accessTypeID}`
  - `PATCH /permissions/{permissionID}`, `DELETE /permissions/{permissionID}`
- **Audit** log entries (`logger.Audit`) on all new mutation handlers.
- **OpenAPI** (`api/openapi.yaml`) and **Postman** collection updated with the new operations.
- **API tests** (`server_test.go`) for happy and error paths; **store tests** (`store_test.go`) for delete/patch edge cases.

## Steps

1. Add `*PatchParams` structs and new method signatures to `go/internal/store/store.go`.
2. Implement in `go/internal/store/sqlite/store.go`; use `wrapConstraintError` on deletes.
3. Register flat routes in `server.go` under `/api/v1` (avoid nested `Route` shadowing); add handlers with audit.
4. Extend `server_test.go` (happy path CRUD, FK-blocked delete, patch empty body, not-found, broken-store table).
5. Extend `store_test.go` (delete with/without dependents, patch fields, empty patch → `ErrInvalidInput`).
6. Update `api/openapi.yaml` and `api/postman/access-manager.postman_collection.json`.
7. Add `CHANGELOG.md` Unreleased entry.
8. Run `make test`, `make lint`, `make cover`.

## Files / paths

- **Edit:** `go/internal/store/store.go`, `go/internal/store/sqlite/store.go`, `go/internal/api/server.go`, `go/internal/api/server_test.go`, `go/internal/store/sqlite/store_test.go`, `api/openapi.yaml`, `api/postman/access-manager.postman_collection.json`, `CHANGELOG.md`

## Acceptance criteria

- All entity types support `GET` (by id), `PATCH`, and `DELETE` via the API.
- PATCH with empty body → 400; PATCH with unknown fields → 400 (`DisallowUnknownFields`).
- DELETE of entity with dependents (RESTRICT FKs from T33) → 400.
- DELETE of entity without dependents → 204.
- Audit log emitted for every mutation.
- `make test` and `make lint` pass; coverage ≥80% per file.

## Out of scope

- Bulk delete / bulk patch.
- List pagination (T34) and filtering (T35).

## Dependencies

- **T33** (RESTRICT FKs — same PR).
- **T31** (`writeStoreErr`, error classification).
- **T32** (`ErrConflict` for duplicate grants — already done).
