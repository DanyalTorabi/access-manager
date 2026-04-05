# T42 — User authz resources listing

**Issue:** [#57](https://github.com/DanyalTorabi/access-manager/issues/57)  
**Parent plan:** [T41 — Authz listing umbrella](T41-authz-listing-and-principals-apis.md) (**#56**)

## Phase

**Phase 6** — P3 product options

## Goal

Implement **`GET /api/v1/domains/{domainID}/users/{userID}/authz/resources`**: paginated list of resources where the user has **any** effective access, with **`effective_mask`** per row (same semantics as **`EffectiveMask`** — direct user grants + direct group membership; **no** **T2** ancestor inheritance in V1).

## API (propose in PR)

- **Query:** `limit`, `offset` (T34).
- **Response:** e.g. `{ "data": [ { "resource_id", "effective_mask" } ], "meta": { "total", "offset", "limit" } }` — align with existing list envelope.
- **Mask:** String decimal for `uint64` if that matches `authz/check` / `authz/masks`.
- **404:** Unknown domain or user → same pattern as other handlers.

## Store

- New method on **`store.Store`** + sqlite: e.g. list distinct `resource_id` with aggregated effective mask for `(domainID, userID)`, with `COUNT(*)` for pagination (implementation detail in PR).

## Tests (required)

- **Integration:** `internal/api/server_test.go` — seed data, multiple permissions/groups, pagination, OR of masks.
- **E2E:** `go/e2e/` — live `BASE_URL`; document in `test/e2e/README.md` if needed.

## Files / paths

- [internal/store/store.go](../../go/internal/store/store.go), [internal/store/sqlite/store.go](../../go/internal/store/sqlite/store.go), tests
- [internal/api/server.go](../../go/internal/api/server.go), [internal/api/server_test.go](../../go/internal/api/server_test.go)
- [go/e2e/](../../go/e2e/)

## Acceptance

- Endpoint + store + integration + E2E for this route; **Fixes #57** in PR.

## Out of scope

- Other T41 routes (T43–T45).
