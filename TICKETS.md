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
| T15 | CHANGELOG | done | [CHANGELOG.md](CHANGELOG.md) + README process; semver tags on release (**T6**) |
| T16 | E2E / smoke tests | done | **`go test -race -count=1 -tags=e2e ./e2e/...`** ([`go/e2e/`](go/e2e/)); optional [`test/e2e/bash/run.sh`](test/e2e/bash/run.sh); **`make e2e`** / **`make e2e-bash`**; CI (**T13**) |
| T17 | API docs & contract testing | done | [`api/openapi.yaml`](api/openapi.yaml), [`api/postman/`](api/postman/), [`api/README.md`](api/README.md) |
| T18 | Developer AI / editor tooling | done | `.cursor/rules`, `AGENTS.md`, tech stack doc |
| T19 | Docker | done | [Dockerfile](Dockerfile), [docker-compose.yml](docker-compose.yml), [.dockerignore](.dockerignore); distroless non-root + SQLite tmpfs; **T13** can reuse compose |
| T1 | Per-dialect migrations (postgres/mysql) | open | **`go/migrations/`** + **`go/internal/database`** |
| T6 | GitHub remote + repo hygiene | done | [CONTRIBUTING.md](CONTRIBUTING.md) maintainer checklist; [.github/](.github/) PR + issue templates; complete **branch protection** + **Actions/GHCR** in GitHub UI per CONTRIBUTING |
| T33 | Restrict cascading deletes | open | Change FK constraints from `CASCADE` to `RESTRICT` on referenced entities so deleting an in-use entity (e.g. a group with members, a permission that is granted) returns an error instead of silently removing related data. New migration + store-layer error handling. Pairs with **T37** (delete endpoints). |
| T34 | List pagination | open | Add `start` (offset) and `pagesize` query parameters to all list endpoints. Default page size, max cap, return total count or next-page indicator in response. |
| T35 | List filtering | open | Add filter query parameters on all fields for list endpoints. Support operators: `starts_with`, `contains`, `ends_with`. Design a consistent query parameter convention (e.g. `?title__contains=foo`). |
| T36 | Sanitize API error responses + fix Postman/OpenAPI samples | open | (1) Stop exposing internal/SQL error details to API consumers — return clean, database-agnostic messages (e.g. "referenced domain does not exist") while logging the internal error via `logger`. (2) Complete Postman collection variables and request ordering so samples work out of the box (create domain before user, etc.). (3) Align OpenAPI spec with any changes. |
| T37 | Delete and update (PATCH) endpoints | open | Add `DELETE` and `PATCH` endpoints for all entities (domains, users, groups, resources, access types, permissions). PATCH for partial updates. DELETE behavior depends on **T33** (restrict when in use). Update Postman + OpenAPI. |

---

## P3 — Later (scale, prod, hardening)

| id | title | status | notes |
|----|-------|--------|-------|
| T20 | Security review | done | govulncheck in CI + `make vuln`; gosec linter; `internal/logger` wrapping `log/slog` with structured audit logging on all 13 mutation handlers; threat model in [`docs/security-review.md`](docs/security-review.md); `toolchain go1.25.8` in `go.mod` for stdlib patches |
| T21 | Kubernetes | open | Deployments, probes, config from **T26**; ArgoCD/Terraform when org defines |
| T22 | Environments: dev / PR / staging / prod | open | Promotion, secrets, DB per env |
| T23 | Observability | done | **Prometheus** metrics (`/metrics`), **Grafana** dashboards, compose; middleware + authz counter; tests. Plan: [plan/phase-6/T23-observability-prometheus-grafana.md](plan/phase-6/T23-observability-prometheus-grafana.md) |
| T2 | Optional group ancestor permission inheritance | open | V1: direct membership only |
| T3 | Resource hierarchy | open | Flat resources in v1 |
| T4 | Materialized `user_resource_mask` for hot path | open | If profiling requires |
| T5 | Authz benchmarks / load tests | open | k6 etc.; pairs **T4** |
| T30 | Go coverage above 90% | done | Raise **`go/`** statement coverage past **90%** (see **`make cover`**); add tests for thin **`cmd/`** wiring and edge paths; optional CI gate |
| T31 | Handler error classification | done | Typed store errors (`store.ErrFKViolation`, `store.ErrInvalidInput`, `store.ErrNotFound`) → proper HTTP status codes (FK → 400, unknown → 500, not-found → 404). Plan: [plan/phase-6/T31-handler-error-classification.md](plan/phase-6/T31-handler-error-classification.md) |
| T32 | Duplicate constraint handling (ErrConflict) | done | Duplicate membership/grant inserts hit UNIQUE/PK constraint → currently 500; should be `409 Conflict`. Add `store.ErrConflict`, detect `SQLITE_CONSTRAINT_PRIMARYKEY`/`UNIQUE`, map in `writeStoreErr`. Plan: [plan/phase-6/T32-duplicate-constraint-handling.md](plan/phase-6/T32-duplicate-constraint-handling.md) |

---

## Quick reference (by theme)

| Theme | Ticket ids |
|-------|------------|
| Curriculum map | Table at top of this file |
| Docs & process | T8, T14, T15 |
| Build & local dev | T9, T19, T26, T28, T29 |
| Testing & quality | T10, T11, T12, T16, T17, T27, T5, T30 |
| Platform & delivery | T6, T13, T19, T21, T22, T29 |
| Security & access | T7, T20, T31, T33, T36 |
| Product / data model | T1–T4, T34, T35, T37 |
| Tooling / AI | T18 |
| Ops / runtime | T23 |
