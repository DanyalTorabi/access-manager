# Tickets / backlog

Umbrella tickets only: break into sub-tasks when you start work. **Priority** reflects current **development phase** (P1 = do soon while building; P3 = defer).

Engineering practices follow the org **Backend Engineering** curriculum where they apply to this service: config, graceful exit, testing, CI, Docker, observability, concurrency safety. **Out of scope for this repo (not tracked):** event buses (Kafka/Redpanda), Protobuf/gRPC as a requirement, and Cassandra/Scylla as primary storage—they do not match the access-manager SQL model unless the product direction changes.

---

## Backend Engineering curriculum → access-manager

| # | Curriculum theme | How we align |
|---|------------------|--------------|
| 1 | **Go** | **`go/`** module: `cmd/` layout, health (`/health`), tests, **`go test -race`** (**T9**, **T10**, **T27**). Prefer **stdlib** where practical; small router dep is acceptable. Port from env/config (**T26**), not a fixed lab port. |
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
| T8 | README | done | Quickstart, layout, env + **config file** (**T26**), how to run tests **with `-race`**, link to curriculum alignment section |
| T9 | Makefile | done | **`go/Makefile`** + root **`Makefile`** delegating with **`make -C go`**; `build`, `test` (include **`-race`**), `cover`, `run` / `serve`, `tidy`; **`lint` → golangci-lint** (pairs **T28**); root **`docker-build` / `docker-up` / `docker-logs` / `docker-down`** (**T19**) |
| T10 | Unit tests (expand) | done | Handlers, store edge cases; table-driven |
| T11 | Integration tests | done | HTTP + real DB; **compose-backed** in **T13** |
| T12 | Test coverage | done | `go test -cover` / profile; optional CI gate |
| T13 | CI/CD (curriculum-aligned) | done | [.github/workflows/ci.yml](.github/workflows/ci.yml): **`go/`** test/vet/lint; Docker build + compose **health** smoke; push **`main`** → **GHCR** `latest` + `sha-<full>`; branch protection: require job checks **Go (test, vet, lint)** + **Docker build & compose smoke** ([CONTRIBUTING.md](CONTRIBUTING.md)) |
| T14 | Branching strategy | done | [docs/branching.md](docs/branching.md); [README.md](README.md) section (**T14**) |
| T26 | Config: file + env | done | File for ports, DSN, URLs, toggles; **env overrides**; no secrets in repo |
| T27 | Graceful shutdown & concurrency safety | done | **SIGINT/SIGTERM** → `Server.Shutdown`, drain in-flight; **context** on store/API; **`-race`** in Makefile/CI |
| T28 | golangci-lint | done | Add **`go/.golangci.yml`** (or org template), wire **T9** + **T13** |
| T7 | API authentication middleware | done | Optional **`API_BEARER_TOKEN`** (Bearer on **`/api/v1/*`**); JWT deferred; startup warning if non-loopback without token |
| T29 | Monorepo: Go under `go/` | done | Go module + service under **`go/`**; root **`Makefile`** → `go/`; room for **`spec/`**, other language dirs at repo root; plan: [plan/phase-3/T29-monorepo-go-directory.md](plan/phase-3/T29-monorepo-go-directory.md) |

---

## P2 — Next (after core flows stabilize)

| id | title | status | notes |
|----|-------|--------|-------|
| T15 | CHANGELOG | open | Keep a Changelog, semver, tags/releases |
| T16 | E2E / smoke tests | open | Full API journeys; optional Newman in CI |
| T17 | API docs & contract testing | open | OpenAPI/Swagger + Postman collection |
| T18 | Developer AI / editor tooling | done | `.cursor/rules`, `AGENTS.md`, tech stack doc |
| T19 | Docker | done | [Dockerfile](Dockerfile), [docker-compose.yml](docker-compose.yml), [.dockerignore](.dockerignore); distroless non-root + SQLite tmpfs; **T13** can reuse compose |
| T1 | Per-dialect migrations (postgres/mysql) | open | **`go/migrations/`** + **`go/internal/database`** |
| T6 | GitHub remote + repo hygiene | done | [CONTRIBUTING.md](CONTRIBUTING.md) maintainer checklist; [.github/](.github/) PR + issue templates; complete **branch protection** + **Actions/GHCR** in GitHub UI per CONTRIBUTING |

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
| T30 | Go coverage above 90% | open | Raise **`go/`** statement coverage past **90%** (see **`make cover`**); add tests for thin **`cmd/`** wiring and edge paths; optional CI gate |

---

## Quick reference (by theme)

| Theme | Ticket ids |
|-------|------------|
| Curriculum map | Table at top of this file |
| Docs & process | T8, T14, T15 |
| Build & local dev | T9, T19, T26, T28, T29 |
| Testing & quality | T10, T11, T12, T16, T17, T27, T5, T30 |
| Platform & delivery | T6, T13, T19, T21, T22, T29 |
| Security & access | T7, T20 |
| Product / data model | T1–T4 |
| Tooling / AI | T18 |
| Ops / runtime | T23 |
