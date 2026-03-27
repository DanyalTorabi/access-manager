# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- E2E smoke: **`go test -race -count=1 -tags=e2e ./e2e/...`** (**T16**); optional bash twin under **`test/e2e/bash/`**; Docker CI runs Go e2e
- OpenAPI 3 spec and Postman collection under **`api/`** with README for **`baseUrl`** and Bearer token variables (**T17**)

### Fixed

- OpenAPI: optional Bearer security, `authz/check` **400** documents both `text/plain` and JSON error body, clarify request vs response JSON field naming (review feedback)
- OpenAPI: distinguish CRUD vs authz response shapes; document mask/bit SQLite-safe range; Postman **Health** uses **noauth**; **api/README** wording (Copilot follow-up)

### Changed

- Contributor docs: defer valid PR review follow-ups to a named ticket (**Txx**) with a tracking note in **`plan/`** (AGENTS, CONTRIBUTING, Cursor rules)

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
