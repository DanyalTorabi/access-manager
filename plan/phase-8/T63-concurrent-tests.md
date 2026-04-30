# T63 — Concurrent HTTP request tests for the API

## Ticket

**T63** — Concurrent HTTP request tests (GitHub [#99](https://github.com/DanyalTorabi/access-manager/issues/99))

## Phase

**Phase 8** — API hardening (concurrency)

## Goal

Add tests that hammer the API server with concurrent in-process HTTP requests to surface race conditions, deadlocks, and inconsistent reads. Run them with `-race` so the Go race detector flags any unsynchronised access in handlers, store, metrics, or shared state.

## Background

Today's tests are mostly sequential. The `-race` flag is on, but races only fire when goroutines actually overlap. Without concurrent test traffic we cannot prove that the server is goroutine-safe under realistic load patterns.

## Deliverables

- `go/internal/api/concurrent_test.go` — concurrent table-driven tests using `httptest.Server` + `errgroup` / `sync.WaitGroup`.
- A small helper `runConcurrent(t, n int, fn func(i int) error)` that:
  - Launches `n` goroutines.
  - Collects errors via a buffered channel (per the project's "goroutines must send a result" rule).
  - Asserts no error and uses `t.Fatal` from the *test goroutine* (not the spawned ones — they `t.Errorf` or push errors to the channel).
- Coverage of at least these scenarios:
  - **Read-mostly mixed**: 50 goroutines × 100 iterations each calling `GET /authz/check` against a pre-seeded domain.
  - **Write contention**: N goroutines creating distinct users in the same domain in parallel — confirms unique IDs are not collided and `ErrConflict` is correctly returned for true duplicates.
  - **Mixed read/write**: half goroutines reading lists while half mutate group memberships; assert no 500s and no race detector hits.
  - **Cycle-detection thrash**: parallel `groupSetParent` calls trying to introduce a cycle — at most one should succeed; others must return 400.
  - **Metrics correctness**: after the run, `authz_checks_total{result="ok"}` equals exactly the number of successful authz calls (verifies the T50 single-increment invariant under concurrency).

## Steps

1. Add `go/internal/api/concurrent_test.go` with the helper and the scenarios above. Use `httptest.NewServer(s.Handler())` (or whatever the existing test setup uses) so the in-memory server runs on a real socket — that exercises the chi router and HTTP stack, not just handler functions.
2. Make tests skippable with `-short` (`if testing.Short() { t.Skip(...) }`) so unit-only CI runs (`go test -short`) stay fast; the default CI run keeps them.
3. Add a `make test-concurrent` Makefile target for explicitly running just these tests with `-race -count=3` (rerun a few times to flush flaky races).
4. Confirm the existing `make test` already runs these (no opt-in tag); they should be part of the baseline because race-detector value comes from running them often.
5. Document the helper and the failure modes in `go/internal/api/concurrent_test.go` doc comment.

## Acceptance criteria

- New tests pass with `-race -count=1` and again with `-count=5` locally (no flaky races).
- Tests do not use `time.Sleep` to wait for goroutines; they use `sync.WaitGroup` / channels with a deadline.
- All goroutines either finish before the test returns or are cancelled via `t.Context()` (Go 1.24+).
- `make test` passes.
- If a race is found, fix it (or open a follow-up ticket and reference it from a `// TODO(T<NN>): ...` comment).

## Files / paths

- **Create:** `go/internal/api/concurrent_test.go`
- **Edit:** `go/Makefile` (new `test-concurrent` target)
- **Edit:** `go/README.md` (mention `make test-concurrent`)

## Dependencies

- **T11 / #22** — integration tests already provide `newTestAPI(t)` helpers we can reuse.
- **T50 / #74** — single-increment invariant on `authz_checks_total` (one of the scenarios verifies this under concurrency).

## Out of scope

- Stress / soak / k6 load testing — covered by **T5 / #16** (see updated plan with the Stress subsection).
- Concurrent E2E tests against a real (non-in-memory) server — can be a future addition.

## Risk notes

- SQLite in WAL mode allows concurrent readers but serialises writers. If write-contention scenarios start hitting `SQLITE_BUSY`, the store layer should retry with backoff or the test should expect a small fraction of `409`/`503` results. Track any retry logic added in a separate follow-up rather than smuggling it into this ticket.
