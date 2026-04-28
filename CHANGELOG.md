# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **T44 / #59:** `GET /api/v1/domains/{domainID}/resources/{resourceID}/authz/users` — paginated list of users in the resource's domain with non-zero effective access on the resource. Each item carries `effective_mask` (OR of direct user grants and grants inherited via group membership).
- **T43 / #58:** `GET /api/v1/domains/{domainID}/groups/{groupID}/authz/resources` — paginated list of resources where the group has direct `group_permissions`, with `mask` = bitwise OR of all grants for that `(domain, group, resource)`.
- **`make gosec`** target in `go/Makefile` and root `Makefile` for running `gosec` security scanner.
- **T35 / #46:** Title search (`?search=`) on all six list endpoints (`LIKE` with escaped wildcards). Optional `?search_type=` parameter: `contains` (default), `starts_with`, `ends_with`. Entity-specific filters: `?parent_group_id=` on groups, `?resource_id=` on permissions. Filters apply to both paged results and `meta.total`.
- **T34 / #45:** Offset/limit pagination on all six list endpoints (`domains`, `users`, `groups`, `resources`, `access-types`, `permissions`). Query params `offset` (default 0) and `limit` (default 20, max 100). Responses wrapped in `{"data": [...], "meta": {"total", "offset", "limit"}}` envelope with total record count.
- **T37 / #48:** `GET` / `PATCH` / `DELETE` for domains; `PATCH` / `DELETE` for users, groups, resources, access types, and permissions; `GET` for a single access type. Partial JSON bodies for `PATCH` (see OpenAPI).
- **T33:** SQLite migration `000002_restrict_foreign_keys` — foreign keys use `ON DELETE RESTRICT` so deletes fail while dependents exist (maps to **400** with `store.ErrFKViolation`), instead of cascading.
- `govulncheck` in CI and `make vuln` target for dependency vulnerability scanning
- `gosec` linter enabled in `golangci-lint` config
- `internal/logger` module wrapping `log/slog` with JSON handler and `Audit()` helper for structured audit events
- Audit logging on mutation handlers — each emits a structured JSON line with `audit=true`, action name, and relevant IDs
- Threat model: [`docs/security-review.md`](docs/security-review.md) documenting actors, assets, trust boundaries, mitigations, and known gaps
- Prometheus metrics middleware: `http_requests_total`, `http_request_duration_seconds`, `authz_checks_total`; `/metrics` endpoint
- Grafana + Prometheus in `docker-compose.yml`; provisioned datasource and **Access Manager** dashboard under `observability/`
- E2E smoke: **`go test -race -count=1 -tags=e2e ./e2e/...`**; optional bash twin under **`test/e2e/bash/`**; Docker CI runs Go e2e
- OpenAPI 3 spec and Postman collection under **`api/`** with README for **`baseUrl`** and Bearer token variables
- **T36 / #47:** Postman collection Create requests auto-save entity IDs into collection variables; items reordered for dependency-safe top-to-bottom run; OpenAPI `ErrorBody` schema documents stable error semantics

### Fixed

- OpenAPI: optional Bearer security, `authz/check` **400** documents both `text/plain` and JSON error body, clarify request vs response JSON field naming (review feedback)
- OpenAPI: distinguish CRUD vs authz response shapes; document mask/bit SQLite-safe range; Postman **Health** uses **noauth**; **api/README** wording (Copilot follow-up)

### Security

- **T36 / #47:** API error responses no longer expose internal SQL/driver details. All error bodies use stable, database-agnostic messages (`resource not found`, `referenced entity does not exist or is still referenced`, `internal server error`, etc.). Full errors are logged server-side for operator diagnostics.

- Temporary limitation: access masks are limited to the lower 63 bits (SQLite `INTEGER` is signed 64-bit and current storage casts to `int64`). This avoids overflow when clients use the most-significant unsigned bit. Enforce first-63-bits until v2 migration; tracked in [#67](https://github.com/DanyalTorabi/access-manager/issues/67).

### Changed

- **T40 / #55:** `access-types` list default sort changed from `bit` to `title` for consistency with other entities. Clients relying on bit-order should sort client-side.
- **T34 / #45 (breaking):** List endpoint responses changed from bare JSON arrays to paginated `{"data": [...], "meta": {...}}` envelope. Clients must update to read `data` for items and `meta` for pagination info.
- Migrated `cmd/server` from `log.Printf` / `log.Fatal` to structured `internal/logger` calls
- Pinned toolchain to **go1.25.8** via `toolchain` in `go/go.mod` (language `go 1.25.0`) for stdlib security patches
- Contributor docs: defer valid PR review follow-ups to a named ticket with a tracking note in **`plan/`** (AGENTS, CONTRIBUTING, Cursor rules)

## [0.1.0] - 2026-03-27

### Added

- Go HTTP service under `go/` with domain-scoped access control (users, groups, resources, access-type bits, permissions, authorization checks) and SQLite persistence
- Configuration via optional `CONFIG_PATH` YAML and environment variable overrides
- Optional static Bearer token for `/api/v1/*`; `/health` remains public
- Dockerfile, compose file, and Make targets for local container runs
- GitHub Actions workflow: Go test, vet, lint; Docker build and compose health smoke; container image publish to GHCR on pushes to `main`

### Security

- Logged startup warning when the listen address is not loopback and no API bearer token is configured

[Unreleased]: https://github.com/DanyalTorabi/access-manager/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/DanyalTorabi/access-manager/releases/tag/v0.1.0
