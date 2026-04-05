# T41 — Authz listing APIs (umbrella)

**Issue:** [#56](https://github.com/DanyalTorabi/access-manager/issues/56)

## Phase

**Phase 6** — P3 product options (read-side authz surface)

## Goal

Expose **read APIs** for administrators and UIs for authz matrices: resources per user/group, and users/groups per resource, with computed masks. Work is split into **one plan + issue per endpoint family** (**T42–T45**); this ticket tracks the **overall** delivery and shared conventions.

## Child plans (sub-issues)

| Plan | Issue | Scope |
|------|--------|--------|
| [T42 — User authz resources](T42-user-authz-resources.md) | [#57](https://github.com/DanyalTorabi/access-manager/issues/57) | `GET …/users/{userID}/authz/resources` |
| [T43 — Group authz resources](T43-group-authz-resources.md) | [#58](https://github.com/DanyalTorabi/access-manager/issues/58) | `GET …/groups/{groupID}/authz/resources` |
| [T44 — Resource authz users](T44-resource-authz-users.md) | [#59](https://github.com/DanyalTorabi/access-manager/issues/59) | `GET …/resources/{resourceID}/authz/users` |
| [T45 — Resource authz groups](T45-resource-authz-groups.md) | [#60](https://github.com/DanyalTorabi/access-manager/issues/60) | `GET …/resources/{resourceID}/authz/groups` |

**GitHub:** #57–#60 are **sub-issues** of **#56**.

## Shared conventions (all children)

- **Paths:** `/api/v1/domains/{domainID}/…` (same style as `authz/check`).
- **Pagination:** **T34** — `limit` / `offset`, `MaxLimit`, stable sort; response envelope consistent with other list endpoints.
- **User effective mask:** Same as **`EffectiveMask`** (direct `user_permissions` + direct `group_members` + `group_permissions`; **no** ancestor inheritance until **T2**).
- **Group mask on a resource:** Bitwise **OR** of `access_mask` for permission rows granted to that group for that `resource_id` (direct grants only).
- **Layering:** Store + sqlite implementation; **no** chi in `internal/store` or `internal/access`.
- **Tests:** Each child ships **integration** (`internal/api`) tests + **E2E** (`go/e2e/`) for its endpoint(s).

## Umbrella acceptance

Close **#56** when **#57–#60** are closed (all four endpoints + tests per child plan).

## Out of scope (umbrella)

- **T2** ancestor inheritance (when added, children’s queries must match `EffectiveMask` expansion).
- **T4** materialized hot path.

## Dependencies

- **T34** pagination; stable **T10/T11** authz semantics.

## Curriculum link

— (API + SQL + tests)
