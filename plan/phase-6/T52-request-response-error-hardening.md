# T52 — API request/response error hardening

## Ticket

**T52** — API request/response error hardening

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Harden the HTTP helper layer so request parsing, response encoding, and handler error handling behave deterministically and do not silently swallow failures.

## Deliverables

- `writeJSON` checks the `json.Encoder.Encode` error path and returns/logs a server-side failure signal when encoding fails.
- `readJSON` rejects trailing JSON tokens after the first decoded value.
- Request parse/logging helpers distinguish parse failures, decoder failures, and range failures in server logs without exposing raw input to clients.
- List handlers either document or explicitly classify store errors rather than relying on incidental generic 500s.
- Unit tests cover the above behavior.

## Deferred from other PRs

- PR #78 / T47: CML-T47-1, CML-T47-2, CML-T47-8, CML-T47-11.
- PR #79 / T48: CML-T48-3, CML-T48-4, CML-T48-8, CML-T48-10.

## Steps

1. Add server-side logging or explicit error handling for `writeJSON` failures.
2. Make `readJSON` reject trailing data after the first JSON value.
3. Decide the intended list-handler store-error contract and encode it explicitly.
4. Add tests that assert the intended HTTP status and stable public body.

## Acceptance criteria

- Response encoding failures are observable to operators.
- Trailing JSON is rejected.
- Public responses remain stable and do not leak raw internal parsing details.
- `make test` and `make lint` pass.

## Related

- T31 (#42) was the original handler error-classification issue, but it is closed.
- T47 (#68), T48 (#69).