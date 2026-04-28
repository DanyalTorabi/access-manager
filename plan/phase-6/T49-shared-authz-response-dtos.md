# T49 — Centralise shared authz listing response DTOs

## Ticket

**T49** — Centralise shared authz listing response DTOs (GitHub [#72](https://github.com/DanyalTorabi/access-manager/issues/72))

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Reduce duplication in the small per-handler response structs used by the three authz listing endpoints (T42 user→resources, T43 group→resources, T44 resource→users; later T45 resource→groups) by extracting one or more shared DTO types and (optionally) a small mapping helper. Today each handler defines its own `*Response` struct with two fields and an inline mapping loop, which is fine in isolation but is starting to repeat.

## Deliverables

- One small DTO package (or a few exported types in `go/internal/api`) covering the authz listing response shapes:
  - `{principal_id (or resource_id), mask-as-decimal-string}` rows
  - shared mapping helper that converts `[]store.X` → `[]DTO`
- Handlers (`userAuthzResources`, `groupAuthzResources`, `resourceAuthzUsers`, plus T45 once landed) updated to use the shared DTO/helper.
- No JSON wire-format change — every existing field name and decimal-string formatting must be preserved exactly. Existing API/integration/e2e tests must pass without modification.
- Optional: short doc comment on the shared type referencing the OpenAPI schemas (`UserAuthzResource`, `GroupAuthzResource`, `ResourceAuthzUser`, `ResourceAuthzGroup`).

## Steps

1. Inventory the existing per-handler response structs in `go/internal/api/server.go`:
   `userAuthzResourceResponse`, `groupAuthzResourceResponse`, `resourceAuthzUserResponse`, plus T45's response when added.
2. Decide on the shared shape — likely two small types because the JSON field names differ (`resource_id` vs `user_id`/`group_id` and `effective_mask` vs `mask`).
3. Extract those types and a `toDTOSlice` helper (generic over the source row type) and route all four handlers through it.
4. Run `make test` (existing API + e2e tests) and `make lint`; confirm no JSON payload diffs.

## Files / paths

- `go/internal/api/server.go` (DTO extraction + handler updates)
- `go/internal/api/server_test.go` (no behavioural change expected; only update if struct names referenced)
- Possibly a new `go/internal/api/authz_dto.go` if grouping is preferred.

## Acceptance criteria

- The four authz listing handlers share their response DTOs / mapping helper instead of defining bespoke per-handler structs.
- Wire format (field names, decimal-string masks, list envelope) is unchanged — existing API and e2e tests pass without edits.
- `make test` and `make lint` clean.

## Out of scope

- Renaming any JSON field or changing mask serialization format.
- Refactoring non-authz list responses.
- Changing pagination meta semantics.

## Dependencies

- Best done after **T45** (#60) is merged so all four endpoints exist and can be migrated in one pass.

## Deferred from other PRs

- **From T44 (#59 / PR #71) review (CM13):** the suggestion to centralise shared authz DTOs to reduce duplication. Deferred to this ticket because the cleanup is most valuable once T45 lands and there is a fourth handler to migrate in the same change.
