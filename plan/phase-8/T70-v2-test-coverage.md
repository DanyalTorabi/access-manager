# T70 — Comprehensive V2 API test coverage

## Ticket

**T70** — V2 API test coverage completion (GitHub [#127](https://github.com/DanyalTorabi/access-manager/issues/127))

## Phase

**Phase 8** — API evolution and dynamic enums

## Goal

Achieve feature parity in test coverage between V1 and V2 APIs. Currently, V2 has ~58% of V1's test coverage, with major gaps in list/search/sort operations, PATCH scenarios, edge cases, and integration tests.

## Background

T62 (#98) delivered the V2 API with new title-based permission endpoints. The initial implementation includes focused happy-path tests, but lacks comprehensive coverage of:
- Query parameters (search, filter, sort, pagination)
- Error cases and edge scenarios
- PATCH operations with partial updates
- Authorization listing with all variants
- Cross-version compatibility validation

V1 has 52 comprehensive tests across access types, permissions, and authz. V2 currently has only 22, leaving critical functionality untested.

## Current Coverage

| Module | V1 Tests | V2 Tests | Gap |
|--------|----------|----------|-----|
| Access Types | 12 | 7 | 5 missing |
| Permissions | 10 | 8 | 2 missing |
| Authz | 30 | 7 | 23 missing |
| **Total** | **52** | **22** | **30 missing (58%)** |

## Deliverables

### 1. Access Types V2 Tests (5 new tests)

- **`TestAPI_v2_accessTypeList_empty`** — GET returns empty array when no types exist
- **`TestAPI_v2_accessTypeList_multiple`** — GET with pagination, search, sort parameters
  - Search by title substring
  - Sort ascending/descending by title
  - Pagination (limit, offset)
- **`TestAPI_v2_accessTypeList_invalidSort`** — Invalid sort field rejected with 400
- **`TestAPI_v2_accessTypeList_invalidOrder`** — Invalid order (not asc/desc) rejected with 400
- **`TestAPI_v2_accessTypePatch_titleOnly`** — PATCH with title only (auto-allocate new bit if needed)

### 2. Permissions V2 Tests (2 new tests)

- **`TestAPI_v2_permissionList_search`** — List with search by title/resource
- **`TestAPI_v2_permissionList_sort`** — List with sort by title/resource
- **`TestAPI_v2_permissionPatch_titleOnly`** — PATCH with title only
- **`TestAPI_v2_permissionPatch_resourceOnly`** — PATCH with resource only

### 3. Authz V2 Tests (23 new tests)

#### Validation & Parameter Tests (8 tests)

- **`TestAPI_v2_userAuthzResources_unsupportedQueryParams`** — POST with unsupported params rejected
- **`TestAPI_v2_groupAuthzResources_unsupportedQueryParams`** — POST with unsupported params rejected
- **`TestAPI_v2_resourceAuthzUsers_unsupportedQueryParams`** — POST with unsupported params rejected
- **`TestAPI_v2_resourceAuthzGroups_unsupportedQueryParams`** — POST with unsupported params rejected
- **`TestAPI_v2_authzCheck_validation`** — userID/resourceID/bit required; missing → 400
- **`TestAPI_v2_authzCheck_invalidAccessBit`** — bit > 63 or 0 rejected with 400
- **`TestAPI_v2_authzMasks_validation`** — userID/resourceID required; missing → 400
- **`TestAPI_v2_authzCheck_accessBitOutOfRange`** — bit in [64, ∞) rejected with 400

#### NotFound Tests (4 tests)

- **`TestAPI_v2_userAuthzResources_notFound`** — GET with unknown user → 404
- **`TestAPI_v2_groupAuthzResources_notFound`** — GET with unknown group → 404
- **`TestAPI_v2_resourceAuthzUsers_notFound`** — GET with unknown resource → 404
- **`TestAPI_v2_resourceAuthzGroups_notFound`** — GET with unknown resource → 404

#### Integration Tests (6 tests)

- **`TestAPI_v2_authzCheck_deniedWithoutGrants`** — No grants → false
- **`TestAPI_v2_authzCheck_grantedViaUserPermission`** — User grant matches title → true
- **`TestAPI_v2_authzCheck_grantedViaGroupMembership`** — User inherits via group → true
- **`TestAPI_v2_authzMasks_emptyWithoutGrants`** — No grants → empty title array
- **`TestAPI_v2_authzMasks_userAndGroup`** — Union of user + group grants
- **`TestAPI_v2_userResourcePermissions_multiple`** — EffectiveMask with user + group grants

#### Duplicate/Conflict Tests (2 tests)

- **`TestAPI_v2_grantUserPermission_duplicate`** — Grant same user+permission twice → 409
- **`TestAPI_v2_grantGroupPermission_duplicate`** — Grant same group+permission twice → 409

#### Edge Cases (3 tests)

- **`TestAPI_v2_authzCheck_emptyDomain`** — Check against domain with no access types
- **`TestAPI_v2_userAuthzResources_allPermissions`** — User granted all access types; titles sorted correctly
- **`TestAPI_v2_resourceAuthzUsers_noDomainLeakage`** — Results exclude users from other domains

### 4. Pagination & Filtering Tests (implicit in list tests above)

- Verify limit/offset work on V2 permission/authz list endpoints
- Verify search filters correctly on title/resource fields
- Verify sort order is stable and consistent

### 5. Regression Tests (1 new test)

- **`TestAPI_v2_authz_v1CompatibilityRegression`** — V1 and V2 return same effective permissions

## Non-goals

- Changing V2 handler implementations (tests validate existing behavior)
- Adding new V2 endpoints (T62 is complete)
- Breaking V1 tests

## Criteria for done

- [ ] All 30 new test functions added and passing
- [ ] Test coverage report shows V1 ≈ V2 (within 5%)
- [ ] `go test -race ./...` passes all tests
- [ ] No regressions in V1 or V2 existing tests
- [ ] PR includes updated comments-PR#127.md documenting test additions

## Related issues

- [T62 #98](https://github.com/DanyalTorabi/access-manager/issues/98) — V2 API implementation
- [PR #126](https://github.com/DanyalTorabi/access-manager/pull/126) — V2 initial implementation and first-pass tests
