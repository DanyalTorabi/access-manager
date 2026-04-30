# T49 — Centralise shared authz listing response DTOs

## Ticket

**T49** — Centralise shared authz listing response DTOs (GitHub [#72](https://github.com/DanyalTorabi/access-manager/issues/72))

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Reduce duplication in the small per-handler response structs used by the three authz listing endpoints (T42 user→resources, T43 group→resources, T44 resource→users; later T45 resource→groups) by extracting one or more shared DTO types and (optionally) a small mapping helper. Today each handler defines its own `*Response` struct with two fields and an inline mapping loop, which is fine in isolation but is starting to repeat.

## Deliverables

- A new `go/internal/api/authz_dto.go` file containing:
  - Four **unexported** DTO types (replacing the unexported `*Response` structs in `server.go`):
    - `userAuthzResourceDTO` — `resource_id` + `effective_mask` (OpenAPI: `UserAuthzResource`)
    - `groupAuthzResourceDTO` — `resource_id` + `mask` (OpenAPI: `GroupAuthzResource`)
    - `resourceAuthzUserDTO` — `user_id` + `effective_mask` (OpenAPI: `ResourceAuthzUser`)
    - `resourceAuthzGroupDTO` — `group_id` + `mask` (OpenAPI: `ResourceAuthzGroup`)
  - Four types are required (not two) because all four shapes have distinct JSON field names for both the entity ID and the mask; merging any pair would require renaming a wire field.
  - Four typed converter functions (`userAuthzResourceDTOs`, etc.) each with a direct `make/for/append` loop — avoids a generic intermediate layer that added indirection without proportional gain.
  - Doc comments on each type referencing the corresponding OpenAPI schema component and full `/api/v1/...` endpoint path.
- `go/internal/api/authz_dto_test.go` with unit tests covering nil input, empty input, and field-level mapping for all four converters.
- Handlers (`userAuthzResources`, `groupAuthzResources`, `resourceAuthzUsers`, `resourceAuthzGroups`) updated to use the new converter functions.
- No JSON wire-format change — every existing field name and decimal-string formatting must be preserved exactly. Existing API/integration/e2e tests must pass without modification.

## Steps

1. Inventory the four existing per-handler response structs in `go/internal/api/server.go`:
   `userAuthzResourceResponse`, `groupAuthzResourceResponse`, `resourceAuthzUserResponse`, `resourceAuthzGroupResponse`.
2. Create `go/internal/api/authz_dto.go` with four unexported DTO types (named `*DTO` to distinguish from store types) and four typed converter functions with direct loops. Add doc comments with full `/api/v1/...` endpoint paths and OpenAPI schema names.
3. Remove the four unexported `*Response` struct definitions from `server.go` and update the four handlers to use the new converter functions.
4. Add `go/internal/api/authz_dto_test.go` with unit tests for nil/empty input and field-level mapping.
5. Run `make test` and `make lint`; confirm no JSON payload diffs.

## Files / paths

- `go/internal/api/authz_dto.go` (new file — unexported DTO types + typed converter functions)
- `go/internal/api/authz_dto_test.go` (new file — unit tests for converter functions)
- `go/internal/api/server.go` (remove 4 unexported `*Response` structs; update 4 handlers)

## Acceptance criteria

- The four authz listing handlers share their response DTO types and converter functions instead of defining bespoke per-handler structs and duplicated mapping loops.
- DTO types are unexported; converter functions are unexported. No exported identifiers without callers are added.
- Doc comments on DTO types include the full `/api/v1/...` endpoint path and OpenAPI schema name.
- Converter functions are directly tested (nil input, empty input, field mapping).
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
