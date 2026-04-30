# T52 — API request/response error hardening

## Ticket

**T52** — API request/response error hardening

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Harden the HTTP helper layer so request parsing, response encoding, and handler error handling behave deterministically and do not silently swallow failures.

## Deliverables

- `writeJSON` checks the `json.Encoder.Encode` error path and logs a server-side failure signal when encoding fails.
- `readJSON` rejects trailing JSON tokens after the first decoded value using `dec.Decode(&extra)` + `io.EOF` check (the `dec.More()` approach is explicitly avoided — its documented scope is array/object iteration, not top-level stream exhaustion).
- A unified `classifyDecodeErr` helper assigns a structured `kind` label to each decode failure (`empty_body`, `json_syntax`, `json_type`, `json_unknown_field`, `json_decode`, `body_too_large`, `trailing_data`) and returns both the kind and the sanitized client/log messages in one step.
- `logReadJSONErr` receives only pre-sanitized strings — attacker-controlled field names (from `DisallowUnknownFields`) are never logged.
- List handlers document the use of `writeInternalErr` as intentional (they expect only unexpected DB errors, not structured store errors).
- Unit tests cover all of the above behavior.

## Status of deferred items from earlier PRs

The T52 plan originally referenced internal conversation review labels from PR #78 (T47) and PR #79 (T48). All GitHub review threads for those PRs were resolved before this PR was opened. The items listed below are summarized from those conversations:

| Item | PR | Topic | Status |
|------|-----|-------|--------|
| CML-T47-1 | #78 | `gofmt` formatting on new test block | Resolved in PR #78 |
| CML-T47-2 | #78 | `parseUint64Validated` base-0 accepted octal inputs | Resolved in PR #78 (base switched to explicit 10 / 16) |
| CML-T47-8 | #78 | (conversation-level, no open GitHub thread) | N/A — no open thread |
| CML-T47-11 | #78 | (conversation-level, no open GitHub thread) | N/A — no open thread |
| CML-T48-3 | #79 | `t.Fatal` captured outer `t` in subtest closure | Resolved in PR #79 |
| CML-T48-4 | #79 | (conversation-level, no open GitHub thread) | N/A — no open thread |
| CML-T48-8 | #79 | (conversation-level, no open GitHub thread) | N/A — no open thread |
| CML-T48-10 | #79 | (conversation-level, no open GitHub thread) | N/A — no open thread |

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