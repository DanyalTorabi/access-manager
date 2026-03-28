---
applyTo: "go/**/*.go"
---

# Go Code Review Rules

## Error Handling

- HTTP handlers: return 400 for validation/client errors, 500 for unexpected/internal errors. Do not map all store errors to 400.
- Grant/membership handlers that call store INSERT: FK violations are client errors (400), but closed-DB or unexpected errors should be 500.
- `store.ErrNotFound` → 404, not 400 or 500.

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
