# T38 — E2E journey tests

## Ticket

**T38** — E2E journey tests (GitHub [#53](https://github.com/DanyalTorabi/access-manager/issues/53))

## Phase

**Phase 5** — P2 polish and multi-DB

## Goal

Redesign the E2E test suite (`go/e2e/`) from a single smoke test into comprehensive journey tests that exercise the full API surface against a live server.

## Deliverables

- [x] Refactored e2e helpers (`mustPATCH`, `mustDELETE`, list decoder, error helpers, seed helpers).
- [x] Full CRUD lifecycle tests for every entity type (domains, users, groups, resources, access types, permissions).
- [x] Authz challenge scenarios (direct, group-inherited, no permission, multiple masks, revoke-recheck).
- [x] Pagination journey (page-through, past-total, default params).
- [x] Error path journeys (invalid UUIDs, missing fields, duplicates, referential integrity, bad pagination).
- [x] Auth journey (missing/wrong/valid bearer token; health bypass).
- [x] Deprecation notice on `test/e2e/bash/run.sh`.

## Steps

1. Extract and expand helpers into `go/e2e/helpers_test.go`.
2. Write `go/e2e/crud_test.go` — full CRUD lifecycle for all 6 entity types.
3. Write `go/e2e/authz_test.go` — authz challenge scenarios.
4. Write `go/e2e/pagination_test.go` — pagination journey.
5. Write `go/e2e/errors_test.go` — error path journeys.
6. Write `go/e2e/auth_test.go` — bearer auth journey.
7. Deprecate bash script, update docs.

## Files / paths

- **Modify:** `go/e2e/smoke_test.go`, `go/e2e/doc.go`, `test/e2e/bash/run.sh`, `test/e2e/README.md`
- **Create:** `go/e2e/helpers_test.go`, `go/e2e/crud_test.go`, `go/e2e/authz_test.go`, `go/e2e/pagination_test.go`, `go/e2e/errors_test.go`, `go/e2e/auth_test.go`

## Acceptance criteria

- `go test -tags=e2e -race -count=1 ./e2e/...` passes against a live server.
- All entity CRUD, authz, pagination, error, and auth journeys covered.
- `make test` and `make lint` pass (no regressions).

## Out of scope

- Concurrent / performance testing (see T5).
- `httptest`-based integration tests (see T39).

## Dependencies

- **T34** (pagination) — done.
- **T35** (filtering) — optional; add filter tests when it lands.
- **T16** (original smoke tests) — extended by this ticket.
