# T54 — Injectable logger for parallel-safe API tests and structured per-request logging

## Ticket

**T54** — Injectable logger / parallel-safe observable logging

GitHub issue: [#76](https://github.com/DanyalTorabi/access-manager/issues/76)

## Phase

**Phase 6** — P3 scale, prod, hardening

## Background

The `internal/logger` package exposes a package-level `*slog.Logger` pointer
initialised in `init()`. Tests that want to capture log output call
`logger.Init(level, &buf)` to redirect it, relying on the fact that the full
API test suite runs sequentially (`t.Parallel()` is prohibited in
`internal/api`). The existing code comment documents this explicitly:

> *"This is safe only because no test in this file uses t.Parallel(). Do NOT
> add t.Parallel() without first switching to a logger-injectable Server field
> or an atomic pointer."*

This is a known technical debt item originating from T36 (#47). Additional
pain points introduced since then:

- `writeJSON` cannot log encode failures with request context (method/path)
  because it receives no `*http.Request`. Passing request context requires
  surface-level changes rather than a global logger.
- Any future test that mutates `logger.Init` concurrently (e.g. a parallel
  subtest or, more likely, a test helper added by accident) will cause a data
  race on the global pointer.
- High-concurrency load tests (T05) and future integration test suites will
  stall if they cannot run API subtests in parallel.

## Goal

Make the `Server` use an injectable `*slog.Logger` instead of the global from
`internal/logger`, so:

1. Tests can capture or suppress log output per-test without mutating global
   state.
2. Tests can call `t.Parallel()` safely.
3. A future `writeJSON`-with-context change (T55) can log through the per-request
   logger if needed.
4. Production behaviour is unchanged: the default logger is still the
   JSON-over-stderr logger initialised by `cmd/server`.

## Deliverables

- Add an optional `Log *slog.Logger` field to `Server`. When nil, fall back to
  the package-level `logger` global (backward-compatible with existing callers
  including `cmd/server`).
- Expose a `logWith(r *http.Request) *slog.Logger` helper on `Server` that
  returns the per-server logger (or the global) enriched with request context
  attributes; replace current `logger.Error(...)` / `logger.Warn(...)` call
  sites in handlers with this helper.
- Update `newTestAPI(t)` and `newBrokenTestAPI(t)` to inject a
  `*slog.Logger` backed by a per-test `slog.NewJSONHandler(&buf, ...)` so
  each test gets an isolated log buffer.
- Explicitly annotate CML-T52-9-origin tests with `t.Parallel()` once
  injection is in place and verify the race detector passes (`go test -race`).
- Update the API test-file comment explaining the logger isolation strategy.

## Out of scope

- Changing `internal/logger` global package interface (keep for `cmd/server`
  and `internal/store` usage).
- Migrating store or migration log calls.

## Steps

1. Add `Log *slog.Logger` field to `Server`; add `serverLogger() *slog.Logger`
   accessor that returns `s.Log` when non-nil and falls back to the package-level
   logger.
2. Replace all `logger.Error(...)` / `logger.Warn(...)` calls in `server.go`
   handlers with `serverLogger()` (or `logWith(r)`).
3. Update `newTestAPI(t)` to inject a per-test logger.
4. Iteratively add `t.Parallel()` to API unit tests and run race detector.
5. Verify `make test -race` passes.

## Acceptance criteria

- No test in `internal/api` mutates the package-level `logger.Init`.
- `go test -race ./internal/api/...` passes.
- Parallel API subtests (where applicable) run without data races.
- `make test` and `make lint` pass.

## Related

- T36 (#47): original parallel-test follow-up.
- T52 (#80): T52's test suite still mutates `logger.Init`; T54 resolves this.
- T55 (#TBD): add request context to `writeJSON` encode-failure logs (depends
  on the logger injection foundation established here).
