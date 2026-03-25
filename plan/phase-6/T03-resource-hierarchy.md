# T3 — Resource hierarchy

## Ticket

**T3** — Resource hierarchy (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 6** — P3 product options

## Goal

Add optional **`parent_resource_id`** on resources (same domain), cycle prevention, and define whether permissions on a parent **imply** access to descendants (document and implement one model).

## Deliverables

- Schema migration; API updates for create/patch parent.
- Authz evaluator: e.g. permission on resource R applies to subtree, or only exact match—**choose one** and test.

## Steps

1. Add nullable FK `parent_resource_id` + indexes.
2. Cycle check on write (like groups).
3. Extend `EffectiveMask` SQL or application logic for descendant resources.
4. Update OpenAPI (T17) when done.

## Files / paths

- **Edit:** migrations, [internal/store/store.go](../../go/internal/store/store.go), [internal/api/server.go](../../go/internal/api/server.go)

## Acceptance criteria

- Tree of resources behaves per chosen inheritance rule; cycles rejected.

## Out of scope

- Arbitrary DAG resources (tree only v1).

## Dependencies

- **T1** if multi-DB must get same DDL.

## Curriculum link

— (product feature)
