# T10 — Unit tests (expand)

## Ticket

**T10** — Unit tests (expand) (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 2** — Tests and coverage

## Goal

Increase **fast, isolated** test coverage for HTTP handlers and store behavior beyond current `internal/access` and sqlite integration tests.

## Deliverables

- Table-driven tests for **API handlers** using `httptest` + mock or real small store where practical.
- Unit tests for **store error paths** (`ErrNotFound`, invalid FK scenarios if testable without DB).

## Steps

1. Add `internal/api/server_test.go` (or per-handler files): build `Server` with a **fake** implementing `store.Store` or use sqlite `:memory:`/temp for handler tests.
2. Cover: `GET /health`, one CRUD success + 404 path, `GET .../authz/check` happy path.
3. Add sqlite unit tests for `GroupSetParent` cycle rejection, `Revoke*` not found.
4. Keep tests fast; no network except localhost if using test server.

## Files / paths

- **Create:** `internal/api/*_test.go`, optional `internal/store/sqlite/*_test.go` additions
- **Edit:** existing tests as needed

## Acceptance criteria

- `go test -short ./...` (or full `./...`) passes; new tests run under `-race`.

## Out of scope

- Full docker-compose integration (T11); E2E scripts (T16).

## Dependencies

- **Phase 0–1** for stable entrypoints and context.

## Curriculum link

**Theme 1 (Go)** — tests and race detector discipline.
