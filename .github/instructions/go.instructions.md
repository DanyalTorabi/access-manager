---
applyTo: "go/**/*.go"
---

# Go Code Review Rules

## Error Handling

- HTTP handlers: return 400 for validation/client errors, 404 for not-found, 500 for unexpected/internal errors.
- `store.ErrNotFound` → 404; `store.ErrFKViolation` (invalid reference / FK violation) → 400; `store.ErrConflict` → 409; `store.ErrInvalidInput` → 400; everything else → 500. Use the shared `writeStoreErr` helper in handlers — do not roll a new mapping.
- `groupSetParent`: 404 when the group/parent is not found, 400 for self-parent / cycle / domain-mismatch validation errors, otherwise routed through `writeStoreErr`.
- `addUserToGroup`, `grantUserPermission`, `grantGroupPermission`: also use `writeStoreErr`, so FK violations → 400, duplicates → 409, broken-DB / unexpected → 500.
- When deferring an issue, reference the matching plan file (`// TODO(T<NN>): ...`) where `T<NN>` is the actual plan you mean — do **not** copy `T31` as a placeholder. T31 is closed (#42) and refers specifically to handler error classification.

## Concurrency

- Goroutines that perform I/O must send a result (error or nil) on a channel so callers can synchronize and never miss errors.
- `signal.Notify` channels: always `defer signal.Stop(ch)`.
- `net.Listener` + `http.Server.Serve(ln)` preferred over `ListenAndServe` for testability.

## Testing Patterns

- Use `mustPostJSON201(t, url, body)` or similar helpers that assert status before returning body.
- Empty-list responses: decode JSON into a slice and assert `len == 0`; do not compare raw bytes like `"[]\n"`.
- Store tests: use `newTestStore(t)` with `t.TempDir()` DB path and real migrations.
- API tests: use `newTestAPI(t)` / `newBrokenTestAPI(t)` for error-path coverage.
- Use `errors.Is(err, store.ErrNotFound)` for not-found assertions.
- Table-driven tests with `t.Run` for related scenarios (e.g., GroupSetParent subtests).

## SQL / Store Layer

- Use `sql.NullString` for nullable columns; check `.Valid` before dereferencing.
- Foreign key enforcement: `PRAGMA foreign_keys = ON` is set in `sqlite.Open`.
- Cycle detection in `GroupSetParent`: walk parent chain with bounded loop.

## Config

- `config.Load()` already prefixes errors with `config:`; callers should not double-wrap.
- Validation: reject whitespace-only values via `strings.TrimSpace`.
