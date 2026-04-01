# T5 — Authz benchmarks / load tests

## Ticket

**T5** — Authz benchmarks / load tests

## Phase

**Phase 6** — P3 product options

## Goal

Measure **authz check** latency and throughput under realistic data sizes: **Go benchmarks** on store layer and/or **k6** HTTP load against running server.

## Deliverables

- `internal/store/sqlite/bench_test.go` or dedicated `bench/` package.
- Optional `k6` script in `test/load/authz.js` with env-driven base URL.
- Document how to run in README; optional CI job (nightly, not every PR).

## Steps

1. Seed large fixture or generate synthetic domain/users/groups/permissions.
2. Benchmark `EffectiveMask` and full HTTP `/authz/check`.
3. Compare before/after **T4** if implemented.

## Files / paths

- **Create:** `*_test.go` with `func Benchmark*`, optional `test/load/*`

## Acceptance criteria

- Reproducible numbers on a reference machine documented in results appendix.

## Out of scope

- Production load testing customer data.

## Dependencies

- **T4** optional comparison target.

## Curriculum link

**Theme 1/8** — performance exploration.
