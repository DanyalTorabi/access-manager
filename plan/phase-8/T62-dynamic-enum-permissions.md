# T62 — Dynamic enum permissions: API returns titles (v2), keep mask in DB

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
- **Auto-assignment of `bit`**: in v2, `POST /v2/access-types` may omit `bit`; server allocates the lowest unused bit. v1 continues to require explicit `bit` in the request body for backward compatibility.

## Open design points (decide during implementation)

- **Path structure**: prefer flat `/v2/domains/{id}/...` mirroring v1, or a nested `/api/v2/...` prefix. Default plan: route prefix `/v2` only on the affected handlers; non-affected handlers (e.g. `/healthz`, `/metrics`) stay un-prefixed.
- **Shared handler core**: extract a per-handler core that takes a typed input/output and add thin v1/v2 adapters. Avoids handler duplication.
- **Caching access-types per domain**: a small per-domain LRU keyed by `domain_id` to avoid hitting the store on every conversion. Decide based on the `T5`/T62b benchmarks; OK to defer.

## Steps

1. **Helpers (`internal/access`)**: add `MaskToTitles` / `TitlesToMask`. Cover with unit tests using a fake `store.Store` that returns a fixed `access_types` set.
2. **Auto-bit-allocator**: add `store.AllocateNextAccessTypeBit(ctx, domainID) (uint64, error)` returning lowest unused bit or `store.ErrConflict` when 63 bits are taken. Unit-test against the SQLite store.
3. **v2 route group**: in `internal/api/server.go` (or a new `internal/api/v2.go`) wire `/v2/...` handlers that share the existing service code through the new helpers.
4. **Endpoints converted in v2** (initial list — confirm at implementation time):
   - `POST /v2/permissions` → request body `{ "title": "...", "resource_id": "...", "permissions": ["read","write"] }`.
   - `GET /v2/users/{userID}/resources/{resourceID}/permissions` → `["read","write"]` (sorted).
   - `GET /v2/users/{userID}/resources` (T42) → each entry's `permissions` field becomes `[]string`.
   - `GET /v2/groups/{groupID}/resources` (T43), `/v2/resources/{resourceID}/users` (T44), `/v2/resources/{resourceID}/groups` (T45) — same.
   - `POST /v2/access-types` accepting `{ "title": "..." }` without `bit`.
5. **OpenAPI**: add v2 schemas (`PermissionTitleArray`, `AccessTypeCreateV2`, …); document the version difference in `api/README.md`.
6. **Postman**: add a v2 folder mirroring v1 examples with title arrays.
7. **Tests**:
   - Unit: helpers, auto-bit allocator, JSON encoders.
   - Integration: v1 unaffected (regression); v2 round-trips title arrays through the store.
   - E2E: `go/e2e/v2_journey_test.go` (build tag `e2e`).
8. **Docs**: update `go/README.md`, root `README.md`, `api/README.md` with a "V1 vs V2 permission representation" section.

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
