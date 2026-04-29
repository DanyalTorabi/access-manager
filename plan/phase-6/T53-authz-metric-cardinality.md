# T53 — Control authz metric cardinality

## Ticket

**T53** — Control authz metric cardinality

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Reduce the operational risk of the authz metric by avoiding unbounded label cardinality on `domainID`, or document and enforce a strict cardinality budget if the label is retained.

## Deliverables

- Decide whether `authz_checks_total` should keep the `domainID` label or move to a lower-cardinality design.
- If the label stays, document the cardinality budget and the operational assumptions.
- If the label changes, update dashboards / alerting / tests accordingly.
- Add tests or checks that cover the chosen metric shape.

## Deferred from other PRs

- PR #79 / T48: CML-T48-7, CML-T48-9.

## Steps

1. Review expected domain cardinality and Prometheus cost.
2. Choose the metric shape and update code/docs/tests.
3. Verify dashboards and alerts still make sense with the chosen design.

## Acceptance criteria

- The metric shape is deliberate and documented.
- The implementation no longer leaves the cardinality choice implicit.

## Related

- T50 (#74) fixes the double-increment bug on the same metric family.