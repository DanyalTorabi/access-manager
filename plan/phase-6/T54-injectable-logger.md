# T54 — Injectable Server logger for parallel-safe API tests

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

### Phase 1 — Core infrastructure (commit 1)
1. Add `Get() *slog.Logger` to `internal/logger/logger.go` to expose the
   package-level pointer for the Server fallback without coupling `api` to the
   global directly.
2. Add to `server.go`:
   - `Log *slog.Logger` field on `Server` (nil = fall back to `logger.Get()`)
   - `serverLogger() *slog.Logger` — returns `s.Log` when non-nil, else `logger.Get()`
   - `logWith(r *http.Request) *slog.Logger` — returns `serverLogger()` enriched
     with `method` and `path` attributes (deliverable; available for T55)
   - `auditLog(ctx context.Context, action string, attrs ...slog.Attr)` — mirrors
     `logger.Audit` but routes through `s.serverLogger()`

### Phase 2 — Convert server_request.go helpers to Server methods (commit 2)
3. Convert 8 package-level functions to `(s *Server)` methods; replace every
   `logger.Error/Warn` call with `s.serverLogger().LogAttrs(...)`:
   `writeJSON`, `writeErr`, `writeStoreErr`, `writeInternalErr`,
   `logRequestErr`, `logReadJSONErr`, `writeList`, `readJSON`.
   Remove the `// TODO(T54)` comment from `writeJSON`.

### Phase 3 — Update handler files (commit 3)
4. In all 7 handler files add `s.` prefix to the now-method calls and replace
   every `logger.Audit(r.Context(), action, attrs...)` with
   `s.auditLog(r.Context(), action, attrs...)` (26 call sites across
   `server_domains.go`, `server_users.go`, `server_groups.go`,
   `server_resources.go`, `server_permissions.go`, `server_access_types.go`,
   `server_authz.go`).

### Phase 4 — Update test helpers (commit 4)
5. `newTestAPI` signature stays `(*httptest.Server, store.Store)` — avoids
   touching 60+ call sites. Internally set `srv.Log` to a discard logger so
   the global is never touched, making all tests race-safe immediately.
6. Add `newTestAPIWithLog(t) (*httptest.Server, store.Store, *bytes.Buffer)`
   backed by a `slog.LevelDebug` handler over a `bytes.Buffer` — used by the
   handful of tests that assert on log output.
7. Update `newBrokenTestAPIWithRegistry` to inject the discard logger as well.
8. Update the "Do NOT add t.Parallel()" NOTE comment to explain the new
   per-test injection strategy.

### Phase 5 — Migrate tests off logger.Init (commit 5)
9. Tests that call request helpers directly (no HTTP server):
   `TestWriteStoreErr_allCases`, `TestWriteStoreErr_noSQLLeak`,
   `TestWriteInternalErr_generic`, `TestWriteInternalErr_misuse`,
   `TestWriteJSON_encodeErrorLogged`.
   Pattern: `logger.Init(level, &buf)` + package call →
   `s := &Server{Log: slog.New(...)}; s.writeStoreErr(...)`.
10. Tests that used `newTestAPI + logger.Init`, switch to `newTestAPIWithLog(t)`:
    `TestAPI_auditLog_domainCreate`, `TestAPI_auditLog_groupCreate_parentFields`,
    `TestReadJSON_decodeErrors_kindLogged`, `TestReadJSON_bodyTooLargeKindLogged`.
    Drop all `logger.Init(...)` / `t.Cleanup(logger.Init...)` lines and the
    `internal/logger` import where no longer used.

### Phase 6 — Enable t.Parallel() (commit 6)
11. Add `t.Parallel()` to: `TestWriteJSON_encodeErrorLogged`,
    `TestAPI_auditLog_domainCreate`, `TestAPI_auditLog_groupCreate_parentFields`,
    `TestReadJSON_decodeErrors_kindLogged`, `TestReadJSON_bodyTooLargeKindLogged`,
    and the T52 test in `server_authz_test.go` that carried a "t.Parallel()
    intentionally omitted until T54" note.
    Do NOT add `t.Parallel()` to `integration_test.go` (sequential by design).
12. Update all remaining "Do NOT add t.Parallel()" comments in test files.

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
