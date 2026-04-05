# T4 — Materialized `user_resource_mask` hot path

## Ticket

**T4** — Materialized `user_resource_mask` for hot path (GitHub [#15](https://github.com/DanyalTorabi/access-manager/issues/15))

## Phase

**Phase 6** — P3 product options

## Goal

For very large scale, maintain **`(domain_id, user_id, resource_id) → access_mask`** in a table updated **transactionally** on grant/revoke/membership change so reads are O(1) index lookup.

## Deliverables

- New migration: materialized table + triggers or application-level write-through from store methods.
- Invalidation rules documented; backfill job for existing data.

## Steps

1. Benchmark current query path (T5) to justify.
2. Implement write-through in `Grant*`, `Revoke*`, `AddUserToGroup`, etc.
3. Add reconciliation cron or admin endpoint (optional).
4. Feature flag or migration cutover strategy.

## Files / paths

- **Edit:** [internal/store/sqlite/store.go](../../go/internal/store/sqlite/store.go), migrations, possibly [PLAN.md](../../PLAN.md)

## Acceptance criteria

- Effective mask matches non-materialized path on randomized property tests.

## Out of scope

- Cross-region eventual consistency.

## Dependencies

- **T5** benchmarks; stable evaluator semantics.

## Curriculum link

— (performance)

**Suggested P3 order:** often **before T5** optimization loop or **after** benchmarks justify it.
