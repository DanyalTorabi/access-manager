# T12 — Test coverage

## Ticket

**T12** — Test coverage (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 2** — Tests and coverage

## Goal

Make **coverage visible and repeatable**: profile output, optional threshold, documented in Makefile/README.

## Deliverables

- `make cover` (from T9) producing `coverage.out`; optional HTML report.
- Optional: minimum coverage gate in CI (T13)—document target percentage in this plan’s follow-up.

## Steps

1. Ensure `make cover` runs `go test -race -coverprofile=coverage.out ./...`.
2. Add `coverage.out` and `*.html` to `.gitignore` if generated locally.
3. Identify low-coverage packages (`internal/api`, `cmd/server`); add tests in T10/T11.
4. Optionally add `-covermode=atomic` if concurrent tests warrant it.

## Files / paths

- **Edit:** [Makefile](../../Makefile), [.gitignore](../../.gitignore), [README.md](../../README.md)

## Acceptance criteria

- `make cover` completes and `coverage.out` is non-empty after T10/T11 work.

## Out of scope

- Enforcing 90%+ globally on day one; mutation testing.

## Dependencies

- **T10**, **T11** for meaningful coverage numbers.

## Curriculum link

**Theme 4** — quality gate support for CI.
