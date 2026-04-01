# T27 — Graceful shutdown & concurrency safety

## Ticket

**T27** — Graceful shutdown & concurrency safety

## Phase

**Phase 1** — Runtime configuration and safe shutdown

## Goal

Stop the HTTP server **cleanly** on SIGINT/SIGTERM, respect **context** on outbound work, and keep **`go test -race`** as part of normal development (Makefile + later CI).

## Deliverables

- `http.Server` with **idle timeouts** where appropriate.
- **Signal handling** + `Shutdown(ctx)` with bounded wait.
- **Context** passed from HTTP handlers into store methods (refactor store interface if needed).
- Confirm no data races in tests under `-race`.

## Steps

1. Replace bare `ListenAndServe` with `server.ListenAndServe` in a goroutine; main waits on signal.
2. On signal: call `Shutdown` with e.g. 30s context; then close DB.
3. Thread `r.Context()` into store calls from handlers (incremental refactor).
4. Run `go test -race ./...`; fix races (mutexes or channel discipline as needed).
5. Ensure `make test` uses `-race` (T9).

## Files / paths

- **Edit:** [cmd/server/main.go](../../go/cmd/server/main.go), [internal/api/server.go](../../go/internal/api/server.go), [internal/store/store.go](../../go/internal/store/store.go) and sqlite impl as needed

## Acceptance criteria

- Sending SIGTERM during load drains in-flight requests without panic.
- `go test -race ./...` passes.

## Out of scope

- Multi-binary producer/consumer (out of scope for this product); Kubernetes preStop beyond HTTP drain (T21 can reference same pattern).

## Dependencies

- **T26** optional but useful for shutdown timeout/config.

## Curriculum link

**Theme 6 (Concurrency)** — graceful termination and race-free code.
