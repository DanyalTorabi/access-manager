# T17 — API docs & contract testing

## Ticket

**T17** — API docs & contract testing

## Phase

**Phase 5** — P2 polish and multi-DB

## Goal

Publish **OpenAPI 3** (YAML/JSON) for REST routes and a **Postman collection** (or Bruno) for manual and optional CI contract checks.

## Deliverables

- [x] `api/openapi.yaml` (or `docs/openapi.yaml`) generated or hand-maintained.
- [x] Postman collection under `api/postman/` or linked export.
- [x] README section: how to import collection and set `baseUrl`.

## Steps

1. Inventory routes from [internal/api/server.go](../../go/internal/api/server.go).
2. Document request/response schemas (UUID strings, error body shape).
3. Optional: `oapi-codegen` or `swag` if you want generated types later—not required v1.
4. Optional Newman step in T13 for smoke (link in Dependencies).

## Files / paths

- **Create:** `api/openapi.yaml`, `api/postman/*.json` (or similar)

## Acceptance criteria

- OpenAPI describes all public routes; example requests validate against running server.

## Out of scope

- gRPC/Protobuf (out of product scope per [PLAN.md](../../PLAN.md) and [docs/backend-curriculum.md](../../docs/backend-curriculum.md)).

## Dependencies

- **T7** if auth headers must appear in spec.

## Curriculum link

**Contract clarity** — supports curriculum-style API discipline.
