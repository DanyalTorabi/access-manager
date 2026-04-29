# T50 — Fix double increment of `authz_checks_total` (and add per-request test)

**Issue:** [#74](https://github.com/DanyalTorabi/access-manager/issues/74)

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Make the `authz_checks_total` Prometheus counter accurately reflect **one increment per request**, with a label that distinguishes success from failure. Today both `authzCheck` and `authzMasks` increment the counter twice on success and once on failure, which breaks dashboards/alerts.

## Background

In [`go/internal/api/server.go`](../../go/internal/api/server.go), `authzCheck` (around lines 968–984) and `authzMasks` (around lines 996–1006) each call:

```go
if s.metrics != nil {
    s.metrics.AuthzTotal.WithLabelValues(domainID).Inc()
}
```

**twice** — once before the store call and once after. Effects:

- Successful requests are double-counted.
- Failing requests are single-counted (the post-call `Inc()` is skipped because of the early `return`).
- The counter cannot be used for "success rate" or "total throughput" without ad-hoc compensation.

## Deliverables

- `authzCheck` and `authzMasks` each increment the counter **exactly once per request**.
- A `result` label (`"ok"` / `"err"`) is added to `AuthzTotal` (or, equivalently, two separate counters), recorded once at the end of the handler (e.g. via a `defer` or after the success/error branch).
- Unit tests assert the increment count for both success and error paths (validation error, closed/broken DB).
- Grafana dashboard / Prometheus query examples updated if the metric shape changes (label addition is backwards-compatible if existing queries sum across labels, but the dashboard JSON should be reviewed).
- README / observability docs updated to describe the new `result` label.

## Steps

1. Refactor `internal/api/metrics.go` so `AuthzTotal` carries the `result` label (or split into `authz_checks_success_total` + `authz_checks_error_total` — pick one and document the rationale).
2. Update `authzCheck` and `authzMasks` to record the metric exactly once at the end of the request, including in early-return error paths (parse error, missing required params, store error).
3. Add unit tests in `internal/api/metrics_test.go` (or `server_test.go`) that:
   - Issue one successful `authzCheck` / `authzMasks` request and assert the corresponding metric value increases by exactly 1 with `result="ok"`.
   - Issue one failing request (e.g. via `newBrokenTestAPI`) and assert the metric increases by exactly 1 with `result="err"`.
4. Grep the codebase for any other handlers that double-increment metrics (defensive sweep).
5. Update Grafana dashboard JSON under `observability/` if needed; update `go/README.md` and any observability docs.
6. Run `make test`, `make lint`, `make cover`.

## Files / paths

- **Edit:** `go/internal/api/metrics.go`, `go/internal/api/server.go`, `go/internal/api/metrics_test.go` (or new `*_test.go`), `observability/grafana/dashboards/*.json`, `go/README.md`.

## Acceptance criteria

- Each `authzCheck` / `authzMasks` request produces exactly one increment of the relevant counter.
- Tests fail if the double-increment regression is reintroduced.
- `make test`, `make lint` pass.

## Out of scope

- Adding new authz metrics beyond fixing the double-increment + result label.
- Changing other counters (e.g. `http_requests_total`) unless the defensive sweep reveals the same bug.

## Dependencies

- **T23** (Observability) — original metric setup.

## Discovered in

- **PR [#73](https://github.com/DanyalTorabi/access-manager/pull/73) (T45 / #60) review:** external-agent comments **CML1**, **CML2**, **CML10** flagged the double-increment, the missing `result` label, and the lack of a per-request increment test. Out of scope for T45 (handler change, not authz listing) and tracked here.
- **PR #75 (T46 / #67) review:** while reviewing the bit-63 mask limit, an external agent noted that `maskFromSQL` silently coerces negative DB values to `0` and only emits a `slog.Warn`. Negative legacy rows can only exist from out-of-band inserts (PermissionCreate validates the range), so the runtime behaviour is correct, but operators have no machine-readable signal. While this metric ticket is open, also add a small counter (e.g. `store_negative_mask_observed_total`) bumped from `maskFromSQL` so dashboards can alert when corrupted/legacy state appears. Keep the current `slog.Warn` for the human-readable trail.
