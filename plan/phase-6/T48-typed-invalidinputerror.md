# T48 — Typed InvalidInputError and stable public invalid-input messages

## Ticket

**T48** — Typed InvalidInputError and stable public invalid-input messages

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Replace fragile string-prefix extraction used by `publicInvalidInputMsg` with a typed `InvalidInputError` (or similar) so store-layer validation details can be safely and reliably presented to API clients.

## Deliverables

- A typed error type (e.g. `InvalidInputError`) located in `go/internal/store` that carries a machine-safe `Detail` string.
- Store methods use the typed error for invalid-input cases instead of wrapping the `ErrInvalidInput` sentinel with human text.
- `writeStoreErr` updated to use `errors.As` to extract the typed error and present its `Detail` to clients in a stable way.
- Unit tests for store and handler paths verifying message extraction and public messages.

## Steps

1. Define `type InvalidInputError struct { Detail string }` (implements `error`) and an accessor or constructor in `go/internal/store`.
2. Replace instances where `store.ErrInvalidInput` is wrapped with `fmt.Errorf("%w: ...")` in `internal/store` and `internal/store/sqlite` with the typed error (or return `errors.Join(ErrInvalidInput, InvalidInputError{...})` depending on style).
3. Update `publicInvalidInputMsg`/`writeStoreErr` in `go/internal/api/server.go` to use `errors.As` to extract `InvalidInputError` and return its `Detail`, falling back to a stable message otherwise.
4. Add unit tests asserting that the API returns the expected client-facing detail when a store method returns an `InvalidInputError`.

## Files / paths

- `go/internal/store/store.go` (new typed error / constructors)
- `go/internal/store/sqlite/store.go` (use typed error in validation branches)
- `go/internal/api/server.go` (update message extraction, tests)
- `go/internal/api/server_test.go`, `go/internal/store/sqlite/store_test.go` (tests)

## Acceptance criteria

- store-level invalid-input cases surface a stable, extractable `Detail` at the API boundary
- `publicInvalidInputMsg` no longer relies on brittle string-prefix parsing
- tests confirm expected behavior

## Dependencies

- Related to **T31** (handler error classification); coordinate with that plan if overlapping work is scheduled.
