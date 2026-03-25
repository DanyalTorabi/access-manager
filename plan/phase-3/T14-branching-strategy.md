# T14 — Branching strategy

## Ticket

**T14** — Branching strategy (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 3** — GitHub, Docker, CI

## Goal

Document how work flows into **main**: branch naming, PR requirement, and merge policy so CI (T13) and reviews stay predictable.

## Deliverables

- Short doc in **README** section or **`docs/branching.md`** (link from README).

## Steps

1. Choose model: **trunk-based** with short-lived `feature/*` or `fix/*` branches.
2. Require **PR to `main`**; optional: squash merge default.
3. Naming convention: `feature/T10-unit-tests`, `fix/authz-check-query`, etc.
4. Link **T13** when Actions are enabled.

## Files / paths

- **Create:** `docs/branching.md` (optional)
- **Edit:** [README.md](../../README.md)

## Acceptance criteria

- New contributor knows branch naming and that direct pushes to `main` are discouraged once protection exists (T6).

## Out of scope

- GitFlow release branches until needed; CODEOWNERS (optional in T6).

## Dependencies

- None within phase; **T6** enables enforcement.

## Curriculum link

**Theme 4 (CI/CD)** — PR workflow to main.
