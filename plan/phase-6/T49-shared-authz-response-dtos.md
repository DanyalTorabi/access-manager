# T49 — Centralise shared authz listing response DTOs

## Ticket

**T49** — Centralise shared authz listing response DTOs (GitHub [#72](https://github.com/DanyalTorabi/access-manager/issues/72))

## Phase

**Phase 6** — P3 scale, prod, hardening

## Goal

Reduce duplication in the small per-handler response structs used by the three authz listing endpoints (T42 user→resources, T43 group→resources, T44 resource→users; later T45 resource→groups) by extracting one or more shared DTO types and (optionally) a small mapping helper. Today each handler defines its own `*Response` struct with two fields and an inline mapping loop, which is fine in isolation but is starting to repeat.

## Deliverables

- A new `go/internal/api/authz_dto.go` file containing:
  - Four exported DTO types (replacing the unexported `*Response` structs in `server.go`):
    - `UserAuthzResourceDTO` — `resource_id` + `effective_mask` (OpenAPI: `UserAuthzResource`)
    - `GroupAuthzResourceDTO` — `resource_id` + `mask` (OpenAPI: `GroupAuthzResource`)
    - `ResourceAuthzUserDTO` — `user_id` + `effective_mask` (OpenAPI: `ResourceAuthzUser`)
    - `ResourceAuthzGroupDTO` — `group_id` + `mask` (OpenAPI: `ResourceAuthzGroup`)
  - Four types are required (not two) because all four shapes have distinct JSON field names for both the entity ID and the mask; merging any pair would require renaming a wire field.
  - A generic `toAuthzDTOs[S, D any](list []S, fn func(S) D) []D` mapping helper that replaces the four identical `for _, it := range list` loops in the handlers.
  - Doc comments on each type referencing the corresponding OpenAPI schema component.
- Handlers (`userAuthzResources`, `groupAuthzResources`, `resourceAuthzUsers`, `resourceAuthzGroups`) updated to use the new types and `toAuthzDTOs`.
- No JSON wire-format change — every existing field name and decimal-string formatting must be preserved exactly. Existing API/integration/e2e tests must pass without modification.

## Steps

1. Inventory the four existing per-handler response structs in `go/internal/api/server.go`:
   `userAuthzResourceResponse`, `groupAuthzResourceResponse`, `resourceAuthzUserResponse`, `resourceAuthzGroupResponse`.
2. Create `go/internal/api/authz_dto.go` with four exported DTO types (named `*DTO` to distinguish from store types) and a generic `toAuthzDTOs` mapping helper. Add doc comments referencing OpenAPI schema names.
3. Remove the four unexported `*Response` struct definitions from `server.go` and update the four handlers to use the new exported types and `toAuthzDTOs`.
4. Run `make test` (existing API + e2e tests) and `make lint`; confirm no JSON payload diffs.

## Files / paths

- `go/internal/api/authz_dto.go` (new file — exported DTO types + generic mapping helper)
- `go/internal/api/server.go` (remove 4 unexported `*Response` structs; update 4 handlers)

## Acceptance criteria

- The four authz listing handlers share their response DTO types and mapping helper (`toAuthzDTOs`) instead of defining bespoke per-handler structs and duplicated mapping loops.
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
