# T55 — Add request context to writeJSON encode-failure logs

## Ticket

**T55** — Add request context (`method`, `path`) to `writeJSON` encode-failure log

GitHub issue: [#88](https://github.com/DanyalTorabi/access-manager/issues/88)

## Phase

**Phase 6** — P3 scale, prod, hardening

## Background

`writeJSON` logs encode failures via:

```go
logger.Error("response encode failed", slog.String("err", err.Error()))
```

Every other error-logging helper in the API (`logRequestErr`, `logReadJSONErr`)
records `method` and `path` from `*http.Request` alongside the error so
operators can correlate a log entry with a specific endpoint. `writeJSON` does
not receive a `*http.Request` and therefore omits both fields.

When an encode failure fires in production the error log entry will show only
the raw encoding error (e.g. `"json: unsupported type: chan int"`) with no
indication of which endpoint triggered it, making the entry difficult to triage.

This was flagged as CML-T52-3 in the PR #86 review. It was deferred from T52
because the fix requires adding `*http.Request` to `writeJSON`, which cascades
to `writeErr` and `writeList` and then to every handler call site (~30+ changes).
That refactor is mechanical but invasive and belongs in its own dedicated PR.

The injectable logger work in T54 (#76) is a prerequisite or companion:
once `Server` has a `Log *slog.Logger` field, `writeJSON` can receive a
pre-enriched `*slog.Logger` (already carrying method/path) rather than a raw
`*http.Request`, and the signature change becomes smaller.

## Goal

Ensure `writeJSON` encode failures always include `method` and `path` in the
server log so operators can identify which endpoint triggered the error.

## Deliverables

- **Preferred approach** (after T54): `writeJSON` receives a `*slog.Logger`
  already enriched with request context (method, path) from the per-server
  accessor introduced in T54. All existing call sites pass `s.logWith(r)`.
- **Alternative approach** (independent of T54): Add an optional
  `*http.Request` parameter to `writeJSON`. Pass `nil` from call sites that
  genuinely have no request context (e.g. unit tests calling `writeJSON`
  directly); log method/path only when non-nil.
- All call sites in `server.go` updated consistently.
- New test: assert that an unserializable value passed to a handler produces an
  ERROR log entry that includes both `method` and `path` fields.

## Implementation (shipped in PR #88)

The **alternative (`*http.Request`) approach** was used because T54 (#76) is
still open. `*http.Request` was added as the second parameter to `writeJSON`,
`writeErr`, and `writeList`; all ~60 handler call sites in `server.go` updated.
The `health` handler's blank `_` identifier was changed to `r` so it can be
passed through. `TestWriteJSON_encodeErrorLogged` was extended to assert that
both `method` and `path` appear in the ERROR log entry.

## Steps

1. Decide approach (preferred: T54 logger injection, or standalone parameter).
2. Update `writeJSON` signature and all call sites in `server.go`.
3. Update unit test helpers that call `writeJSON` directly.
4. Add test asserting method+path appear in the encode-failure log entry.
5. Verify `make test` and `make lint` pass.

## Dependencies

- T54 (#76): injectable `Server.Log` field is the cleanest enabler; T55 can
  proceed independently using the `*http.Request` parameter approach if T54
  is delayed.

## Acceptance criteria

- A `writeJSON` encode failure log entry always contains `method` and `path`.
- No regression in existing encode-failure detection tests.
- `make test` and `make lint` pass.

## Related

- T52 (#80): origin of this deferral.
- T54 (#76): injectable logger (companion/prerequisite).
