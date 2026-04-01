# T16 — E2E / smoke tests

## Ticket

**T16** — E2E / smoke tests

## Phase

**Phase 5** — P2 polish and multi-DB

## Goal

Run **full user journeys** against a live server (local or CI): create domain → entities → grant → authz check—using shell, **httpexpect**, or **Newman**.

## Deliverables

- [x] Script or Go test that assumes server is up (or starts it via `exec` in test with caution).
- [x] Optional: dedicated `make e2e` target.

## Steps

1. [x] Reuse OpenAPI/Postman from **T17** or write shell with `curl`.
2. [x] Capture exit codes for CI.
3. [x] Wire optional job in **T13** after integration tests.

## Files / paths

- **Create:** `test/e2e/bash/run.sh` (optional) and/or `go/e2e/*` with `go test -race -count=1 -tags=e2e ./e2e/...`

## Acceptance criteria

- One E2E path green against docker-compose stack (T19) or documented local steps.

## Out of scope

- Load testing (T5); full browser UI tests.

## Dependencies

- **T11** integration patterns; **T17** optional; **T19** for compose target.

## Curriculum link

**Theme 4** — confidence before release.
