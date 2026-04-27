# T47 — API: normalize numeric parse errors and return stable 400

## Ticket

**T47** — API: normalize numeric parse errors and return stable 400

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Normalize parsing and validation errors for numeric fields at the API boundary so clients always receive a stable, database-agnostic `400 Bad Request` message. This includes `bit` and `access_mask` parsing (decimal or 0x hex) and range checks that mirror store-side limitations.

## Deliverables

- API-level parsing helper that returns normalized errors (no raw `strconv` messages returned to clients).
- API handlers (`access_type` create/patch, `permission` create/patch) use the helper and perform range checks consistent with store limits (reject values > math.MaxInt64).
- Unit tests in `go/internal/api` asserting stable `400` responses for malformed numeric input and out-of-range values.
- Update CHANGELOG/plan references if needed.

## Steps

1. Add a parsing helper in `go/internal/api` (e.g. `parseUint64Validated`) that:
   - Accepts a string, attempts `strconv.ParseUint`, and returns either a `uint64` or a well-formed `error` whose message is safe for clients (e.g. "invalid numeric value" or "value out of allowed range").
   - Optionally accepts max allowed value to enforce API-side range checks.
2. Replace direct calls to `parseUint64` in handlers that accept `bit` / `access_mask` (create/patch) with the validated helper.
3. Add unit tests in `go/internal/api/server_test.go` asserting `400` for: non-numeric input, malformed hex, and values above the signed-64 limit.
4. Run `make test` and `make lint`; update documentation if needed.

## Files / paths

- `go/internal/api/server.go` (add helper, update handlers)
- `go/internal/api/server_test.go` (add tests)
- `go/internal/api/helpers_test.go` (helpers for e2e tests may be extended)

## Acceptance criteria

- malformed numeric input to `bit` / `access_mask` results in HTTP 400 with a stable message (no raw strconv text)
- out-of-range numeric inputs (values > math.MaxInt64) are rejected at API with HTTP 400
- unit tests assert stable messages and range rejection

## Dependencies

- Align with **T46** (limit 63-bit mask) for range semantics and with **T31** for broader handler error classification if applicable.