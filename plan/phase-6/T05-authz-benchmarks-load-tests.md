# T5 — Authz benchmarks / load tests

## Ticket

**T5** — Authz benchmarks / load tests (GitHub [#16](https://github.com/DanyalTorabi/access-manager/issues/16))

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

## Deferred from other PRs

- **From T44 (#59 / PR #71) review:** add a perf/regression benchmark for `Store.ResourceAuthzUsersList` (sqlite) that simulates large user/membership counts (1000+ users with mixed direct + group-inherited grants on a single resource). Today the implementation uses a per-user `EXISTS` predicate plus two batched `IN` aggregation queries, bounded by `store.MaxLimit` (100). The benchmark should both establish a baseline and let us evaluate a single-query GROUP BY / aggregated bit-OR alternative if numbers warrant it.
- **From T45 (#60 / PR #73) review:** external-agent comments **CML3** and **CML9**.
  - **CML3:** `Store.ResourceAuthzGroupsList` (and the sibling `ResourceAuthzUsersList` / `GroupAuthzResourcesList`) batch-aggregates masks via `IN (?, …)` clauses bounded by `store.MaxLimit` (100). Safe today against SQLite's parameter cap (≥999), but if `MaxLimit` is ever raised significantly the `IN` approach must be reworked (chunk the IDs or fold mask aggregation into the page-select). Add a benchmark that varies the page size near the SQLite parameter cap and document the chunking strategy when triggered.
  - **CML9:** `GroupSetParent`'s parent-chain cycle detection uses a defensive `maxSteps = 1_000_000` loop. Not a hot path today, but for very deep / pathological tenant graphs this could become expensive. Add a benchmark over a deep parent chain (e.g. 10k+ levels) so we can decide whether to lower the bound, switch to recursive CTE, or memoise.
  - **Shared mask-aggregation helper (PR #75 review):** the "select IDs, then `IN(...)` aggregate masks" pattern is repeated across `UserAuthzResourcesList`, `GroupAuthzResourcesList`, `ResourceAuthzUsersList`, and `ResourceAuthzGroupsList`. Each builds the placeholder list, the args slice, and the mask SQL inline. Extract a shared `(query string, args)` helper (similar to the existing `inPlaceholders`) so future edits don't have to keep four sites in sync, and so a single chunking implementation can serve all four when CML3 is acted on.

## Dependencies

- **T4** optional comparison target.

## Curriculum link

**Theme 1/8** — performance exploration.
