# T4 — Materialized `user_resource_masks` hot path

## Ticket

**T4** — Materialized `user_resource_masks` for hot path (GitHub [#15](https://github.com/DanyalTorabi/access-manager/issues/15))

## Phase

**Phase 6** — P3 product options

## Goal

Maintain **`(domain_id, user_id, resource_id) → access_mask`** in a table kept in sync with every grant/revoke/membership change so that reads (`EffectiveMask`, `UserAuthzResourcesList`, `ResourceAuthzUsersList`) are O(1) indexed lookups instead of multi-JOIN query-time aggregations.

## Deliverables

- Migration 000004: `user_resource_masks` table with covering indexes and ON DELETE CASCADE FKs.
- Application write-through: all 6 mutation methods (`Grant/Revoke UserPermission`, `Grant/Revoke GroupPermission`, `Add/RemoveUserFromGroup`) wrapped in explicit transactions that upsert/delete from `user_resource_masks` after each write.
- `EffectiveMask`, `UserAuthzResourcesList`, and `ResourceAuthzUsersList` switched to read from `user_resource_masks`.
- `ReconcileUserResourceMasks` backfill called at server startup (full DELETE + rebuild from source tables inside a single TX).
- Property tests that assert materialized result == `CombineMasks(PermissionMasksForUserResource(...))` (ground truth) after every mutation.

## Approach decisions

- **No T5 benchmarks required** before implementation (deferred; implementation proceeded on design merit).
- **Application write-through** (not SQLite triggers): explicit, testable, debuggable.
- **Listing reads from materialized table**: `UserAuthzResourcesList` and `ResourceAuthzUsersList` read from `user_resource_masks`. `GroupAuthzResourcesList` and `ResourceAuthzGroupsList` are group-scoped and unchanged.
- **Schema designed for T02 extension**: `TODO(T02)` comment in migration documents where ancestor-inheritance invalidation paths would be added.

## Files changed

| File | Change |
|------|--------|
| `go/migrations/sqlite/000004_user_resource_masks.up.sql` | New table + indexes |
| `go/migrations/sqlite/000004_user_resource_masks.down.sql` | Rollback |
| `go/internal/store/sqlite/store_authz_masks.go` | `computeAndUpsertMask`, updated `EffectiveMask`, helper functions |
| `go/internal/store/sqlite/store_authz_membership.go` | TX + write-through for all 6 mutations |
| `go/internal/store/sqlite/store_authz_user_listing.go` | `UserAuthzResourcesList` → materialized reads |
| `go/internal/store/sqlite/store_authz_resource_listing.go` | `ResourceAuthzUsersList` → materialized reads |
| `go/internal/store/sqlite/store_authz_reconcile.go` | `ReconcileUserResourceMasks` implementation |
| `go/internal/store/store.go` | `ReconcileUserResourceMasks` added to `Store` interface |
| `go/internal/store/postgres/store.go` | No-op stub to satisfy interface |
| `go/internal/store/mysql/store.go` | No-op stub to satisfy interface |
| `go/cmd/server/main.go` | Call reconcile after migration at startup |
| `go/internal/store/sqlite/store_authz_masks_property_test.go` | Property tests |

## Acceptance criteria

- ✅ `EffectiveMask` returns O(1) indexed result from `user_resource_masks`.
- ✅ Effective mask matches non-materialized ground-truth path on property tests (`TestMaterialized_masksMatchGroundTruth`).
- ✅ `ReconcileUserResourceMasks` restores correct masks after table wipe (`TestMaterialized_reconcile`).
- ✅ All existing tests pass unchanged (`make test`).

## Out of scope

- Cross-region eventual consistency.
- T5 benchmarks (deferred per implementation decision).
- MySQL / Postgres materialized tables (no-op stubs only).
- `GroupAuthzResourcesList` / `ResourceAuthzGroupsList` (group-level; no user key).
- Admin HTTP endpoint for on-demand reconciliation (future ticket).

## Dependencies

- **T02** (group ancestor inheritance): schema is designed to accommodate via `TODO(T02)` — no structural change needed, only additional invalidation paths in write-through methods.
- **T5** benchmarks: skipped; implementation justified by O(1) vs O(n joins) design reasoning.

