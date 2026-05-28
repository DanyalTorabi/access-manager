# T63 — Concurrent HTTP request tests for the API

## Ticket

**T63** — Concurrent HTTP request tests (GitHub [#99](https://github.com/DanyalTorabi/access-manager/issues/99))

## Phase

**Phase 8** — API hardening (concurrency)

## Goal

Add tests that hammer the API server with concurrent in-process HTTP requests to surface race conditions, deadlocks, and inconsistent reads. Run them with `-race` so the Go race detector flags any unsynchronised access in handlers, store, metrics, or shared state.

## Background

Today's tests include sequential and a few concurrent scenarios in `integration_test.go`, but these do not cover all concurrency patterns needed to validate the server under realistic load. The `-race` flag is on, but races only fire when goroutines actually overlap. The new tests in `concurrent_test.go` add targeted, higher-load scenarios that the existing integration tests do not cover.

## Deliverables

- `go/internal/api/concurrent_test.go` — concurrent tests using `httptest.NewServer` + `sync.WaitGroup` + buffered-channel error collection.
- A small helper `runConcurrent(t *testing.T, n int, fn func(ctx context.Context, i int) error)` that:
  - Launches `n` goroutines each receiving the test context (`t.Context()`, Go 1.21+).
  - Collects errors via a buffered channel (per the project's "goroutines must send a result" rule).
  - Reports all errors via `t.Errorf` from the *test goroutine* (spawned goroutines must never call `t.Fatal`/`t.Error` directly), then calls `t.FailNow` if any errors were found.
- Coverage of at least these scenarios:
  - **Read-mostly**: 50 goroutines × 20 iterations each calling `GET /authz/check` against a pre-seeded domain — verifies no races in read-path handlers.
  - **Write contention**: 30 goroutines each creating a distinctly-titled user in the same domain — confirms all IDs are unique (no UUID collision) and all requests succeed.
  - **Mixed read/write**: 10 writer goroutines adding/removing group memberships concurrently with 10 reader goroutines listing users — asserts no 500s and no race-detector hits.
  - **Cycle-detection thrash**: 20 goroutines racing to set `g1.parent=g2` and `g2.parent=g1` concurrently — at most one direction can be committed; others must return 400 (cycle detected) and never 500.
  - **Metrics correctness**: 40 goroutines each calling `authz/check` once against a metrics-enabled server; after all finish, `authz_checks_total{result="ok"}` must equal exactly 40 — verifies the T50 single-increment invariant under concurrency.

## Steps

1. Add `go/internal/api/concurrent_test.go` with the helper and the scenarios above. Use `httptest.NewServer(srv.Router(nil, nil))` (matching existing `newTestAPI`/`newTestAPIWithMetrics` helpers) so all tests exercise the real chi router and HTTP stack.
2. Guard every test with `if testing.Short() { t.Skip(...) }` so `go test -short` (fast unit-only CI pass) skips them; the default `make test` and `make test-concurrent` runs keep them enabled.
3. Add a `make test-concurrent` Makefile target running just the `TestConcurrent_*` tests with `-race -count=3` to repeatedly flush flaky races.
4. Existing `make test` already includes these tests (no build tag required); race-detector value comes from running them often.
5. Document the `runConcurrent` helper and failure modes in the file's package-level doc comment.

## Acceptance criteria

- New tests pass with `-race -count=1` and again with `-count=3` locally (no flaky races).
- Tests do not use `time.Sleep` to wait for goroutines; they use `sync.WaitGroup` / channels.
- All goroutines are cancelled via `t.Context()` when the test times out.
- `make test` passes.
- If a race is found during development, fix it in the same PR (or open a follow-up ticket and reference from a `// TODO(T<NN>): ...` comment).

## Files / paths

- **Create:** `go/internal/api/concurrent_test.go`
- **Edit:** `go/Makefile` (new `test-concurrent` target)
- **Edit:** `go/README.md` (mention `make test-concurrent`)

## Dependencies

- **T11 / #22** — integration tests provide `newTestAPI(t)` / `newTestAPIWithMetrics(t)` helpers reused here.
- **T50 / #74** — single-increment invariant on `authz_checks_total` verified under concurrency.

## Out of scope

- Stress / soak / k6 load testing — covered by **T5 / #16**.
- Concurrent E2E tests against a real (non-in-memory) server — future addition.

## Risk notes

- SQLite uses `SetMaxOpenConns(1)` which serialises all DB operations through a single connection. Concurrent HTTP requests will queue at the `database/sql` pool level rather than hitting SQLite directly; `SQLITE_BUSY` is therefore not expected. If it does appear, add retry-with-backoff to the store layer in a separate ticket.
- Goroutine counts are intentionally moderate (20–50) to keep `-count=3` runs fast while still exercising overlapping requests.
