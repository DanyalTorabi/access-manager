# T44 — Resource authz users listing

**Issue:** [#59](https://github.com/DanyalTorabi/access-manager/issues/59)  
**Parent plan:** [T41 — Authz listing umbrella](T41-authz-listing-and-principals-apis.md) (**#56**)

## Phase

**Phase 6** — P3 product options

## Goal

Implement **`GET /api/v1/domains/{domainID}/resources/{resourceID}/authz/users`**: paginated list of **users** in the domain with **non-zero** effective access on that resource, each with **`effective_mask`** (**`EffectiveMask`** semantics).

## API (propose in PR)

- **Query:** `limit`, `offset` (T34).
- **Response:** items `{ "user_id", "effective_mask" }` + list meta.

## Store

- Efficient query or strategy to list users with non-zero mask for `(domainID, resourceID)`; paginated, stable sort (e.g. `user_id`).

## Tests (required)

- Integration + E2E.

## Acceptance

- **Fixes #59** in PR.

## Out of scope

- T42, T43, T45.
