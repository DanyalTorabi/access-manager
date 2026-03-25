# Tickets / backlog

Umbrella tickets only: break into sub-tasks when you start work. **Priority** reflects current **development phase** (P1 = do soon while building; P3 = defer).

Engineering practices follow the org **Backend Engineering** curriculum where they apply to this service: config, graceful exit, testing, CI, Docker, observability, concurrency safety. **Out of scope for this repo (not tracked):** event buses (Kafka/Redpanda), Protobuf/gRPC as a requirement, and Cassandra/Scylla as primary storage—they do not match the access-manager SQL model unless the product direction changes.

---

## Backend Engineering curriculum → access-manager

| # | Curriculum theme | How we align |
|---|------------------|--------------|
| 1 | **Go** | `cmd/` layout, health (`/health`), tests, **`go test -race`** (**T9**, **T10**, **T27**). Prefer **stdlib** where practical; small router dep is acceptable. Port from env/config (**T26**), not a fixed lab port. |
| 2 | **Docker** | **Dockerfile**, **scratch/minimal** final stage (**T8**, **T19**); config from file + env (**T26**). |
| 3 | **Databases** | **Pluggable SQL** (**T1**), **integration tests** (**T11**), **docker-compose** for app + DB in CI/local (**T13**, **T19**), **auth middleware** (**T7**). Relational schema only. |
| 4 | **CI/CD** | PRs to **main** on **ubuntu**: unit + **integration (compose)**, **`go vet`**, **[golangci-lint](https://github.com/golangci/golangci-lint)**, build image, **publish to ghcr.io** on merge (**T13**). Local **[act](https://github.com/nektos/act)** optional. |
| 5 | **Observability** | **Prometheus** metrics, **Grafana** in compose, dashboards, tests + **CI** (**T23**). |
| 6 | **Concurrency** | **Graceful shutdown** (`http.Server.Shutdown`, signals), **context** on store/API, **`-race`** in Makefile/CI (**T27**). |
| 7 | **Kubernetes** | **T21** + **T22**; config via manifests/Helm (**T26**). ArgoCD / Terraform when org defines. |

---

## P1 — During active development

| id | title | status | notes |
|----|-------|--------|-------|
| T8 | README | open | Quickstart, layout, env + **config file** (**T26**), how to run tests **with `-race`**, link to curriculum alignment section |
| T9 | Makefile | open | `build`, `test` (include **`-race`**), `cover`, `run` / `serve`, `tidy`; **`lint` → golangci-lint** (pairs **T28**) |
| T10 | Unit tests (expand) | open | Handlers, store edge cases; table-driven |
| T11 | Integration tests | open | HTTP + real DB; **compose-backed** in **T13** |
| T12 | Test coverage | open | `go test -cover` / profile; optional CI gate |
| T13 | CI/CD (curriculum-aligned) | open | **Ubuntu**; on **PR**: unit + **integration (compose)**, **`go vet`**, **golangci-lint**; build Docker image; on **merge to main**: **publish `ghcr.io`**; module cache; requires **T6** |
| T14 | Branching strategy | open | PRs to **main**, naming, protection |
| T26 | Config: file + env | open | File for ports, DSN, URLs, toggles; **env overrides**; no secrets in repo |
| T27 | Graceful shutdown & concurrency safety | open | **SIGINT/SIGTERM** → `Server.Shutdown`, drain in-flight; **context** on store/API; **`-race`** in Makefile/CI |
| T28 | golangci-lint | open | Add `.golangci.yml` (or org template), wire **T9** + **T13** |
| T7 | API authentication middleware | open | Bearer / JWT when exposing beyond loopback |

---

## P2 — Next (after core flows stabilize)

| id | title | status | notes |
|----|-------|--------|-------|
| T15 | CHANGELOG | open | Keep a Changelog, semver, tags/releases |
| T16 | E2E / smoke tests | open | Full API journeys; optional Newman in CI |
| T17 | API docs & contract testing | open | OpenAPI/Swagger + Postman collection |
| T18 | Developer AI / editor tooling | open | `.cursor/rules`, `AGENTS.md`, tech stack doc |
| T19 | Docker | open | Multi-stage + **scratch/distroless** final image; **compose** for app + DB for **T11**/**T13**; config via **T26** |
| T1 | Per-dialect migrations (postgres/mysql) | open | `migrations/` + `internal/database` |
| T6 | GitHub remote + repo hygiene | open | Origin, branch protection, GHCR package permissions for **T13** |

---

## P3 — Later (scale, prod, hardening)

| id | title | status | notes |
|----|-------|--------|-------|
| T20 | Security review | open | govulncheck, SAST, threat model, secrets |
| T21 | Kubernetes | open | Deployments, probes, config from **T26**; ArgoCD/Terraform when org defines |
| T22 | Environments: dev / PR / staging / prod | open | Promotion, secrets, DB per env |
| T23 | Observability | open | **Prometheus** metrics (e.g. requests, authz checks, errors), **Grafana** dashboards, compose; tests + CI |
| T2 | Optional group ancestor permission inheritance | open | V1: direct membership only |
| T3 | Resource hierarchy | open | Flat resources in v1 |
| T4 | Materialized `user_resource_mask` for hot path | open | If profiling requires |
| T5 | Authz benchmarks / load tests | open | k6 etc.; pairs **T4** |

---

## Quick reference (by theme)

| Theme | Ticket ids |
|-------|------------|
| Curriculum map | Table at top of this file |
| Docs & process | T8, T14, T15 |
| Build & local dev | T9, T19, T26, T28 |
| Testing & quality | T10, T11, T12, T16, T17, T27, T5 |
| Platform & delivery | T6, T13, T19, T21, T22 |
| Security & access | T7, T20 |
| Product / data model | T1–T4 |
| Tooling / AI | T18 |
| Ops / runtime | T23 |
