# T5 — Authz benchmarks / load tests

## Ticket

**T5** — Authz benchmarks / load tests (GitHub [#16](https://github.com/DanyalTorabi/access-manager/issues/16))

## Phase

**Phase 6** — P3 product options

## Goal

Measure **authz check** latency and throughput under realistic data sizes: **Go benchmarks** on store layer and **k6** HTTP load against a running server. Also extract a shared mask-aggregation helper to deduplicate repeated `IN(?,…)` arg-building across four store listing methods.

## Deliverables

- `go/internal/store/sqlite/store_authz_helpers.go` — shared `buildInQueryAndArgs` helper.
- `go/internal/store/sqlite/bench_test.go` — Go benchmarks for the SQLite authz store layer.
- `go/internal/access/mask_bench_test.go` — Go benchmarks for `access.CombineMasks`.
- `test/load/authz.js` — k6 HTTP load script with env-driven base URL.
- `test/load/RESULTS.md` — results placeholder for post-merge runs.
- Updated `go/README.md` — benchmark and load-test usage docs.
- Updated `CHANGELOG.md` — Unreleased entry.

## Steps

1. **Branch setup** — create `danyal/feature/t05-authz-benchmarks` from main; update this plan file. *(Commit 1)*
2. **Shared helper** — extract `buildInQueryAndArgs(baseSQL string, baseArgs []any, ids []string) (string, []any, error)` in `store_authz_helpers.go`; update `GroupAuthzResourcesList`, `ResourceAuthzGroupsList`, and `ResourceAuthzUsersList` to use it. `buildUserAuthzMaskQueryAndArgs` in `store_authz_user_listing.go` delegates ID-list arg building to `buildInQueryAndArgs` as well. *(Commit 2)*
3. **Go benchmarks** — `bench_test.go` for the SQLite store layer (`EffectiveMask`, `ResourceAuthzUsersList`, `UserAuthzResourcesList`, `ResourceAuthzGroupsList`, `GroupSetParent` deep-chain); `mask_bench_test.go` for `access.CombineMasks`. *(Commit 3)*
4. **k6 load script** — `test/load/authz.js` with ramp-up stages, p99 < 50ms / error < 1% thresholds, and `setup()` seeding domain/user/resource/permission. Add `test/load/RESULTS.md` placeholder. *(Commit 4)*
5. **Docs** — update `go/README.md` (Benchmarks + Load Tests sections) and `CHANGELOG.md` (Unreleased). *(Commit 5)*

## Stress / soak

Beyond micro-benchmarks and short k6 load runs, this ticket also covers **stress and soak** scenarios that intentionally push the server past its happy-path operating point to find breakage modes:

- **Saturation sweep**: increase k6 VUs (e.g. 10 → 50 → 200 → 500) until p99 latency degrades sharply or error rate exceeds 1%. Record the inflection point.
- **Sustained soak**: run a steady moderate load (e.g. 50 VUs) for 30 minutes — watch for memory growth (`go_memstats_alloc_bytes`), goroutine leaks (`go_goroutines`), and SQLite WAL growth.
- **Write-heavy stress**: many parallel writers issuing `POST /users`, `POST /permissions`, `PATCH ...` — confirm the store handles `SQLITE_BUSY` (or surfaces `503` / retry hints) instead of returning 500s.
- **Mixed workload under failure injection**: kill the DB file mid-run (or `chmod 000` it briefly) and confirm the server returns 5xx cleanly and recovers when access is restored, without panicking or hanging.
- Record results in `test/load/RESULTS.md` with date, hardware, server version, and parameters so regressions are visible release-over-release. Tie the release-cadence run into the **T66 / #102** release workflow as an optional pre-release gate.

These stress runs are **not** in CI; they are run on demand or nightly via the optional CI job mentioned above.

## Acceptance criteria

- `make test && make lint` pass with zero errors.
- All benchmarks produce output without panics when run with `-bench=. -run='^$'`.
- Reproducible numbers on a reference machine documented in `test/load/RESULTS.md`.

## Out of scope

- Production load testing customer data.
- Lowering `maxSteps` bound or switching `GroupSetParent` to recursive CTE (deferred; track in #16 follow-up).

## Background: deferred items absorbed into Steps above

The following items from previous PR reviews are addressed in this ticket:

- **From T44 (#59 / PR #71) review (Step 3):** perf/regression benchmark for `Store.ResourceAuthzUsersList` simulating large user/membership counts (1000+ users with mixed direct + group-inherited grants). See `BenchmarkResourceAuthzUsersList` sub-cases Users100/Users500/Users1000.
- **From T45 (#60 / PR #73) review — CML3 (Step 3):** benchmark that varies the page size near the SQLite parameter cap (`BenchmarkResourceAuthzGroupsList_PageNearParamCap`). The TODO comment in `store_authz_helpers.go` documents the chunking strategy for when `MaxLimit` is raised.
- **From T45 (#60 / PR #73) review — CML9 (Step 3):** deep parent-chain benchmark (`BenchmarkGroupSetParent_DeepChain`) over chains of depth 100, 1000, and 10000.
- **From PR #75 review — shared helper (Step 2):** `buildInQueryAndArgs` extracts the "select IDs, then `IN(...)` aggregate masks" pattern from `UserAuthzResourcesList`, `GroupAuthzResourcesList`, `ResourceAuthzUsersList`, and `ResourceAuthzGroupsList`.

## Dependencies

- **T4** optional comparison target.

## Curriculum link

**Theme 1/8** — performance exploration.
