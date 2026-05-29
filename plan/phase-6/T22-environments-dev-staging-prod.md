# T22 — Environments: config promotion, tier matrix, and CORS disable sentinel

## Ticket

**T22** — Environments: dev / PR / staging / prod (GitHub [#33](https://github.com/DanyalTorabi/access-manager/issues/33))

## Phase

**Phase 6** — P3 scale, prod, product options

## Goal

Define **promotion** and **configuration** per environment: secrets injection, DB endpoints, feature flags, and how **PR previews** differ from **staging** and **prod**. Implement the CORS disable sentinel so operators fronting the service with a reverse proxy can opt out of in-process CORS headers cleanly.

## Deliverables

- Matrix doc `docs/environments.md`: tier → DSN source → auth mode → image tag strategy → rollback.
- `compose.prod.yml`: production compose override example (named volume, explicit env pointers).
- CORS `"none"` sentinel in `config.go`: `CORS_ALLOWED_ORIGINS=none` (or `cors_allowed_origins: "none"` in YAML) disables all CORS response headers, enabling operators behind a CORS-managing reverse proxy to opt out cleanly.
- README and `config.example.yaml` updated to document the sentinel.

## Steps

1. Refine plan file title and deliverables (this file).
2. List envs and owners (who deploys what); map T26 config keys to K8s Secrets / GitHub Environments.
3. Document rollback: previous image tag + migration backward policy.
4. Implement `CORS_ALLOWED_ORIGINS=none` sentinel in `config.go` (both env var and YAML paths): sentinel resolves to an empty `CORSAllowedOrigins` slice, which the existing middleware already interprets as "CORS disabled". See T67 (#114) for the CORS middleware itself.
5. Add tests for the sentinel; update `config.example.yaml` and `go/README.md`.
6. Create `docs/environments.md` and `compose.prod.yml`.

## Files / paths

- **Create:** `docs/environments.md`
- **Create:** `compose.prod.yml`
- **Modify:** `go/internal/config/config.go`, `go/internal/config/config_test.go`
- **Modify:** `go/config.example.yaml`, `go/README.md`, `README.md`

## Acceptance criteria

- On-call engineer can find DSN and auth config source for each tier in `docs/environments.md` in one lookup.
- `CORS_ALLOWED_ORIGINS=none` starts the server with an empty origin list (no CORS headers emitted).
- `cors_allowed_origins: "none"` in a YAML config file has the same effect.
- `go test -race ./internal/config/...` passes with sentinel tests included.

## Out of scope

- Multi-region active-active.
- Actual deployment to staging/prod environments.

## Dependencies

- **T13** CI/CD; **T19** Docker; **T21** K8s optional; **T67** CORS middleware.

## Curriculum link

**Theme 7** — operational maturity with K8s.

**Suggested P3 order:** after **T20** baseline or with **T21**.
