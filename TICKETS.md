# Tickets / backlog

Umbrella tickets only: break into sub-tasks when you start work. **Priority** reflects current **development phase** (P1 = do soon while building; P3 = defer).

---

## P1 — During active development

| id | title | status | notes |
|----|-------|--------|-------|
| T8 | README | open | Overview, quickstart, run server/tests, env vars (`DATABASE_*`, `HTTP_ADDR`, `MIGRATIONS_DIR`), module layout |
| T9 | Makefile | open | Targets: `build`, `test`, `cover`, `run` / `serve`, `lint` (optional), `tidy`; document in README |
| T10 | Unit tests (expand) | open | Beyond `internal/access` + sqlite store: handlers, edge cases, store errors; table-driven |
| T11 | Integration tests | open | HTTP + real DB (temp file), router + store wiring; not full staging stack |
| T12 | Test coverage | open | `go test -cover` / `-coverprofile`; optional CI threshold later; prioritize store + API |
| T13 | CI/CD (baseline) | open | GitHub Actions: `go test`, `go vet`, build `cmd/server`; module cache; extends **T6** when remote exists |
| T14 | Branching strategy | open | Trunk vs short-lived branches, PR policy, naming; link from README after GitHub (**T6**) |
| T7 | API authentication middleware | open | Beyond loopback bind: API keys/JWT/mTLS when external consumers exist |

---

## P2 — Next (after core flows stabilize)

| id | title | status | notes |
|----|-------|--------|-------|
| T15 | CHANGELOG | open | Keep a Changelog, semver, tags/releases |
| T16 | E2E / smoke tests | open | Full API journeys (shell, httpexpect, or Newman); separate scope from **T11** |
| T17 | API docs & contract testing | open | OpenAPI/Swagger + Postman collection; optional Newman step in **T13** |
| T18 | Developer AI / editor tooling | open | `.cursor/rules`, `AGENTS.md`, tech stack doc, Copilot instructions, skills — team standards |
| T19 | Docker | open | Dockerfile (multi-stage), optional Compose for app + future Postgres; no secrets in image |
| T1 | Per-dialect migrations (postgres/mysql) | open | `migrations/` placeholders; store + `internal/database` when adding drivers |
| T6 | GitHub remote + repo hygiene | open | Origin, branch protection, issue/PR templates; pairs with **T13**–**T14** |

---

## P3 — Later (scale, prod, hardening)

| id | title | status | notes |
|----|-------|--------|-------|
| T20 | Security review | open | govulncheck, SAST, threat model, secrets, authz audit logging |
| T21 | Kubernetes | open | Manifests or Helm, config/secrets, probes — after **T19** |
| T22 | Environments: dev / PR / staging / prod | open | Promotion, config, DB per env, feature flags; depends on **T13**, **T19**, **T21** |
| T23 | Observability | open | Structured logs, metrics, tracing for multi-instance |
| T2 | Optional group ancestor permission inheritance | open | V1: direct group membership only |
| T3 | Resource hierarchy | open | Flat resources in v1 |
| T4 | Materialized `user_resource_mask` for hot path | open | If profiling shows need at scale |
| T5 | Authz benchmarks / load tests | open | k6 or similar; pairs with **T4** if needed |

---

## Quick reference (by theme)

| Theme | Ticket ids |
|-------|------------|
| Docs & process | T8, T14, T15 |
| Build & local dev | T9, T19 |
| Testing & quality | T10, T11, T12, T16, T17, T5 |
| Platform & delivery | T6, T13, T19, T21, T22 |
| Security & access | T7, T20 |
| Product / data model | T1–T4 |
| Tooling / AI | T18 |
| Ops / runtime | T23 |
