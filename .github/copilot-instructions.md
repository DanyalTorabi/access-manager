# Code Review Standards — access-manager

## Architecture

- **Module root is `go/`**. Run `go` commands from there.
- `internal/access` and `internal/store` are library-oriented: **no** chi/HTTP imports.
- HTTP-only code belongs in `internal/api`; wiring in `cmd/server`.
- Future `go/pkg/...` for importable APIs; avoid coupling core logic to the router.

## Security (Critical)

- Never commit secrets, API keys, tokens, or credentials.
- Default HTTP bind to `127.0.0.1`. Flag any `0.0.0.0` without auth.
- Use parameterized queries; no string concatenation into SQL.
- Bearer tokens must not appear in process argv; use stdin or env.

## Error Handling

- Distinguish client errors (400) from internal errors (500) in handlers.
- Use `errors.Is(err, store.ErrNotFound)` for not-found → 404.
- Wrap errors with `fmt.Errorf("context: %w", err)`; avoid double-prefixing.

## Testing

- Non-trivial behavioral changes require automated tests in the same PR.
- Always assert HTTP status code **before** decoding JSON response body.
- Use `t.TempDir()` for temp paths; never hard-code `/tmp/` or `/dev/null`.
- Use `t.Setenv()` for env vars in tests; never mutate global state.
- Prefer polling with a deadline over `time.Sleep` in tests that wait for servers.
- Goroutines in production code must be synchronized; never silently drop errors.

## Go Style

- Match existing code style; avoid unrelated refactors.
- No comments that just narrate what code does ("increment counter").
- Deferred work must reference a ticket: `// TODO(T14): ...`.
- Keep functions focused; extract testable helpers from `main()`.

## Pull Request Expectations

- `make test` and `make lint` must pass.
- No secrets or real `.env` values in the diff.
- Docs updated if behavior or setup changed.
