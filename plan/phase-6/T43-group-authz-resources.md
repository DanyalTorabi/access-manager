# T43 — Group authz resources listing

**Issue:** [#58](https://github.com/DanyalTorabi/access-manager/issues/58)  
**Parent plan:** [T41 — Authz listing umbrella](T41-authz-listing-and-principals-apis.md) (**#56**)

## Phase

**Phase 6** — P3 product options

## Goal

Implement **`GET /api/v1/domains/{domainID}/groups/{groupID}/authz/resources`**: paginated list of resources where the group has **direct** `group_permissions`, with **`mask`** = bitwise **OR** of `permissions.access_mask` for that `(domain, group, resource)`.

## API (propose in PR)

- **Query:** `limit`, `offset` (T34).
- **Response:** list envelope + items `{ "resource_id", "mask" }` (field name documented; align with T41 umbrella if you standardize on `effective_mask` everywhere).

## Store

- Aggregate `permissions.resource_id` + OR of `access_mask` for grants via `group_permissions` for the given group.

## Tests (required)

- Integration + E2E for this route.

## Files / paths

- Same layers as T42 ([store](../../go/internal/store/), [api](../../go/internal/api/), [e2e](../../go/e2e/))

## Acceptance

- **Fixes #58** in PR.

## Out of scope

- T42, T44, T45.
