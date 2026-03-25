# T2 — Optional group ancestor permission inheritance

## Ticket

**T2** — Optional group ancestor permission inheritance (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 6** — P3 product options

## Goal

When enabled, users inherit **group_permissions** from **ancestor groups** (walk `parent_group_id` chain), not only direct membership—**OR** with existing direct grants.

## Deliverables

- Product flag or versioned behavior documented in PLAN.
- Evaluator changes in [internal/access](../../go/internal/access) + store query or precomputed closure.
- Migration if storing closure table; or recursive CTE per dialect.

## Steps

1. Specify semantics: union of all ancestor groups’ permissions vs transitive membership only.
2. Implement efficient query or materialized closure; add tests for deep trees.
3. Update `EffectiveMask` path and document breaking change if any.

## Files / paths

- **Edit:** [internal/store/sqlite/store.go](../../go/internal/store/sqlite/store.go), migrations, [PLAN.md](../../PLAN.md)

## Acceptance criteria

- User in child group receives permission attached only to parent group when feature on.

## Out of scope

- Cross-domain inheritance.

## Dependencies

- Stable v1 authz tests (**T10/T11**).

## Curriculum link

— (product feature)

**Suggested P3 order:** pick with product priority vs T3/T4.
