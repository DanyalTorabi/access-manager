# T62 — /api/v2: title-based permission API (masks stay in DB)

## Ticket

**T62** — Dynamic enum permissions API (GitHub [#98](https://github.com/DanyalTorabi/access-manager/issues/98))

## Phase

**Phase 8** — API evolution and dynamic enums

## Goal

Let API clients work with permission **titles** (strings) instead of raw 63-bit access masks while keeping the bitmask representation **unchanged at the storage layer**. Each permission `title` is internally assigned to one of the 63 available bits in its `domain_id` namespace.

The change is delivered behind a **versioned API surface**: the existing `/v1/...` (or current default) keeps returning numeric masks for backward compatibility; a new `/v2/...` route group returns and accepts arrays of access-type titles instead.

## Background

- Storage uses a `BIGINT` `access_mask` per `permission` row plus an `access_types` table of `(domain_id, title, bit)`.
- The system currently exposes raw `access_mask` integers in HTTP responses and request bodies.
- Limit is **63 bits** per domain (issue [#67](https://github.com/DanyalTorabi/access-manager/issues/67) / T46).
- Goal: hide the `bit` mechanic from API consumers; let them say `["read", "write"]` instead of `3`.

## Deliverables

- New `/v2/...` route group in `internal/api` that:
  - **Reads:** decodes incoming `permissions: ["title1", "title2"]` arrays per `domain_id` and converts to a mask via the `access_types` table.
  - **Writes:** translates stored masks back to a sorted array of titles in responses.
  - **Errors:** returns 400 `{ "error": "unknown permission \"foo\" in domain \"<id>\"" }` if any title is not registered.
- `internal/access` (or a new `internal/access/enum`) helper:
  - `MaskToTitles(ctx, store, domainID, mask) ([]string, error)` — sorted, stable.
  - `TitlesToMask(ctx, store, domainID, titles []string) (uint64, error)` — atomic; returns `store.NewInvalidInput(...)` for unknown titles.
- Auto-bit-assignment helper used when `POST /v2/access-types` receives a new title without an explicit `bit`: pick the lowest unused bit ∈ [0, 62] in that domain; return 409 if all 63 bits are exhausted.
- OpenAPI v2 schema entries (under `api/openapi.yaml` `paths: /v2/...`).
- Postman collection v2 examples in `api/postman/access-manager.postman_collection.json`.
- Unit + integration tests for both directions and for the auto-bit-assignment.
- E2E test (`go/e2e/`, `//go:build e2e`) walking the v2 happy path: create access types by title only → grant by title → list user permissions and assert title array.

## Non-goals (this ticket)

- Removing v1 routes. v1 remains unchanged and tested.
- Breaking the wire format of v1.
- Storage migrations (no DB changes required; the access-types table already has `(domain_id, title, bit)`).

## Decided design points

- **Default = versioned**: v1 = mask (current behaviour); v2 = titles. The transport router decides; no per-request `?format=` query parameter.
- **One bit per title per domain.** Titles are case-sensitive and unique within a `domain_id`.
- **Sorted titles in responses**: lexical sort for deterministic output (helps test stability and human inspection).
- **Auto-assignment of `bit`**: in v2, `POST /api/v2/access-types` may omit `bit`; server allocates the lowest unused bit. v1 continues to require explicit `bit` in the request body for backward compatibility.
- **Route prefix**: `/api/v2` — mirrors the existing `/api/v1` prefix exactly.
- **Auto-bit allocation strategy**: pure helper `access.AllocateNextBit(types []store.AccessType) (uint64, error)` in `internal/access/enum.go`. Reads all access types for the domain via `AccessTypeList`, ORs all `Bit` values, returns the lowest power-of-2 not yet in use. **No change to `store.Store` interface** — MySQL/Postgres stubs are unaffected.
- **Title uniqueness**: enforced at the DB layer via migration `000004` adding `UNIQUE INDEX idx_access_types_domain_title ON access_types (domain_id, title)`. The existing `wrapConstraintError` in the SQLite store already maps UNIQUE violations to `ErrConflict` → HTTP 409.
- **Sentinel for unregistered bits**: `MaskToTitles` returns `"_bit:V"` (where `V` is the stored bit value, e.g. `"_bit:4"`) for set bits with no matching `access_types` row. This prevents v2 reads from failing on v1-era data whose access types were later deleted.
- **V2 authz scope**: all four listing endpoints (`userAuthzResources`, `groupAuthzResources`, `resourceAuthzUsers`, `resourceAuthzGroups`) plus a new effective-permissions endpoint are included in T62 scope.

## Open design points (deferred)

- **Caching access-types per domain**: a small per-domain LRU keyed by `domain_id` to avoid hitting the store on every conversion. Decide based on the `T5`/T62b benchmarks; OK to defer. // TODO(T62): revisit after benchmarks

## Steps

1. **DB migration `000004`**: add `UNIQUE INDEX idx_access_types_domain_title ON access_types (domain_id, title)` for SQLite, MySQL, and Postgres. Verify `wrapConstraintError` surfaces title-uniqueness violations as `ErrConflict` → 409.
2. **Helpers (`internal/access/enum.go`)**: add `MaskToTitles` / `TitlesToMask` / `AllocateNextBit`. Cover with unit tests using a fixed `[]store.AccessType` slice (no fake store needed — pure functions).
3. **v2 access-types handler** (`internal/api/server_access_types_v2.go`): `accessTypeCreateV2` accepts optional `bit`; calls `AllocateNextBit` when omitted. All other verbs reuse v1 handlers in the router.
4. **v2 permission handlers** (`internal/api/server_permissions_v2.go`): new request/response types with `permissions []string` replacing `access_mask string`. Five handlers: create, get, list, patch (all convert mask↔titles); delete reuses v1 handler.
5. **v2 authz handlers** (`internal/api/server_authz_v2.go`): four listing handlers with `permissions []string` responses plus a new `userResourcePermissionsV2` (GET effective permissions for a user+resource pair).
6. **Route wiring** (`internal/api/server.go`): add `r.Route("/api/v2", ...)` block with same `BearerAuth` middleware as v1. Reuse v1 handlers where behavior is identical.
7. **E2E journey test** (`go/e2e/v2_journey_test.go`, build tag `e2e`): end-to-end path from domain creation through title-based permission grant and authz listing, with a regression check that v1 still returns numeric masks.
8. **OpenAPI** (`api/openapi.yaml`): add v2 path entries and schema components (`PermissionBodyV2`, `PermissionResponseV2`, `AccessTypeBodyV2`).
9. **Postman** (`api/postman/access-manager.postman_collection.json`): add "V2 — Title-based Permissions" folder. Update `go/README.md`, `README.md`, `api/README.md` with a "V1 vs V2" section.

## Acceptance criteria

- All current v1 tests pass without modification.
- New v2 tests cover both directions of mask↔titles, the auto-bit-allocation path, the 63-bits-exhausted 409 path, and unknown-title 400 path.
- `make test` and `make lint` pass; `make e2e` passes when server is reachable.
- No new exported public API in `internal/store` is added beyond what v2 needs (no speculative helpers).
- OpenAPI doc lints (existing CI step) cleanly.

## Files / paths

- **Edit:** [go/internal/api/server.go](../../go/internal/api/server.go) (route registration), [api/openapi.yaml](../../api/openapi.yaml), [api/postman/access-manager.postman_collection.json](../../api/postman/access-manager.postman_collection.json)
- **Create:** `go/internal/api/v2_*.go` (or extend `server.go`), `go/internal/access/enum.go`, tests, `go/e2e/v2_journey_test.go`
- **Edit:** `go/README.md`, `README.md`, `api/README.md`

## Dependencies

- **T46 / #67** — 63-bit limit (already enforced).
- **T17 / #28** — OpenAPI/contract testing infra (apply same to v2).
- **T38 / #53** — E2E journey tests (mirror for v2).

## Out of scope / follow-ups

- Removing v1 routes — separate future ticket once consumers migrate.
- Per-domain title aliases / localisation.
- Deny semantics or hierarchical titles (e.g. `read.metadata`).
