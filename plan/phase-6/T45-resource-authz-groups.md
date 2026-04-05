# T45 — Resource authz groups listing

**Issue:** [#60](https://github.com/DanyalTorabi/access-manager/issues/60)  
**Parent plan:** [T41 — Authz listing umbrella](T41-authz-listing-and-principals-apis.md) (**#56**)

## Phase

**Phase 6** — P3 product options

## Goal

Implement **`GET /api/v1/domains/{domainID}/resources/{resourceID}/authz/groups`**: paginated list of **groups** with **direct** `group_permissions` on any permission targeting this resource, with **`mask`** = OR of contributing `access_mask` values per group.

## API (propose in PR)

- **Query:** `limit`, `offset` (T34).
- **Response:** items `{ "group_id", "mask" }` + list meta.

## Store

- Join `group_permissions` → `permissions` filtered by `resource_id`; `GROUP BY group_id` with OR of masks (or equivalent).

## Tests (required)

- Integration + E2E.

## Acceptance

- **Fixes #60** in PR.

## Out of scope

- T42, T43, T44.
