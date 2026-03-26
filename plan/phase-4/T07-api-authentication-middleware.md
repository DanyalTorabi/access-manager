# T7 — API authentication middleware

## Ticket

**T7** — API authentication middleware (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 4** — API hardening

## Goal

Protect HTTP routes when the service is exposed beyond **loopback**: **Bearer token** or **JWT** validation via chi middleware; clear **401/403** semantics.

## Deliverables

- [x] Middleware applied to `/api/v1/*` (`/health` public).
- [x] Config: static `API_BEARER_TOKEN` / `api_bearer_token` (JWT/JWKS deferred).
- [x] Startup warning when binding beyond loopback with an empty token (no silent “open API” in that case).

## Steps

1. Choose strategy: JWT (RS256 + JWKS) vs opaque bearer vs API key header.
2. Implement `Middleware` wrapping chi: extract token, validate, attach principal to `context`.
3. Optionally map principal to internal `user_id` for authz checks (product decision).
4. Add tests: missing token → 401; bad signature → 401; optional scope tests.
5. Document in README; bind `0.0.0.0` only behind TLS/reverse proxy (document, don’t default open without auth).

## Files / paths

- **Create:** `internal/api/auth.go`, `internal/api/auth_test.go` (or `internal/auth/`)
- **Edit:** [internal/api/server.go](../../go/internal/api/server.go), [cmd/server/main.go](../../go/cmd/server/main.go), config

## Acceptance criteria

- With auth enabled, unauthenticated API calls receive **401**.
- Health endpoint behavior documented (often public).

## Out of scope

- Full OAuth2 login UI; fine-grained ABAC beyond existing permission model.

## Dependencies

- **T26** config; **T10/T11** for tests; stable routes from phase 2.

## Curriculum link

**Theme 3** — simple auth middleware as in curriculum DB module (adapted to this service).
