# T53 — Control authz metric cardinality

## Ticket

**T53** — Control authz metric cardinality  
**GitHub Issue:** [#81](https://github.com/DanyalTorabi/access-manager/issues/81)

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Reduce the operational risk of the authz metric by removing the unbounded `domain_id` label from `authz_checks_total`. Domain IDs are UUIDs, giving O(domains) time series with no natural bound — unsafe in production Prometheus deployments.

## Decision

**Remove the `domain_id` label.** The metric retains only `{result: ok|err}`, bounding cardinality to exactly 2 active time series regardless of tenant count. This aligns with how `http_requests_total` is already bounded (route pattern, not per-entity URLs). The `domain_id` field is available in the structured audit log for per-domain debugging; it does not need to be in a counter metric.

Cardinality budget after this change: **2 series** (`result=ok`, `result=err`).

## Deliverables

- [x] Remove `domain_id` label from `authz_checks_total` definition in `metrics.go`.
- [x] Remove `domainID` parameter from `recordAuthz` helper in `server.go`.
- [x] Update all metric test assertions in `metrics_test.go` to match the new shape.
- [x] Update Grafana dashboard panel query and legend format.
- [x] Document the cardinality budget in the `AuthzTotal` field comment.

## Deferred from other PRs

- PR #79 / T48: CML-T48-7, CML-T48-9 — addressed by this decision.

## Steps

1. ~~Review expected domain cardinality and Prometheus cost.~~ ✓ Unbounded UUIDs, cost O(domains).
2. ~~Choose the metric shape and update code/docs/tests.~~ ✓ Remove `domain_id`.
3. ~~Verify dashboards and alerts still make sense with the chosen design.~~ ✓ Dashboard updated to `sum by (result)`.

## Acceptance criteria

- [x] The metric shape is deliberate and documented (cardinality = 2).
- [x] The implementation no longer leaves the cardinality choice implicit.
- [x] Tests assert the new `{result}` label shape.
- [x] Grafana dashboard updated; no stale `domain_id` label references.

## Related

- T50 (#74) — fixed the double-increment bug on the same metric family.
- T48 (#79) — deferred CML-T48-7, CML-T48-9 resolved here.