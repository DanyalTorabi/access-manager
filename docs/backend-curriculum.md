# Backend Engineering curriculum → access-manager

This document maps org **Backend Engineering** themes to how this repository implements them. It is **not** a backlog: **status, priorities, and new work** live in [GitHub Issues](https://github.com/DanyalTorabi/access-manager/issues).

**Umbrella ids (`Tnn`)** in older text, branch names, and [`plan/`](../plan/) filenames are **stable labels** for those specs; they do not require a separate ticket file in the repo.

| # | Curriculum theme | How we align |
|---|------------------|--------------|
| 1 | **Go** | **`go/`** module: `cmd/` layout, health (`/health`), tests, **`go test -race`** (**T9**, **T10**, **T27**). Prefer **stdlib** where practical; small router dep is acceptable. Port from env/config (**T26**), not a fixed lab port. |
| 2 | **Docker** | **Dockerfile**, **scratch/minimal** final stage (**T8**, **T19**); config from file + env (**T26**). |
| 3 | **Databases** | **Pluggable SQL** (**T1**), **integration tests** (**T11**), **docker-compose** for app + DB in CI/local (**T13**, **T19**), **auth middleware** (**T7**). Relational schema only. |
| 4 | **CI/CD** | PRs to **main** on **ubuntu**: unit + **integration (compose)**, **`go vet`**, **[golangci-lint](https://github.com/golangci/golangci-lint)**, build image, **publish to ghcr.io** on merge (**T13**). Local **[act](https://github.com/nektos/act)** optional. |
| 5 | **Observability** | **Prometheus** metrics, **Grafana** in compose, dashboards, tests + **CI** (**T23**). |
| 6 | **Concurrency** | **Graceful shutdown** (`http.Server.Shutdown`, signals), **context** on store/API, **`-race`** in Makefile/CI (**T27**). |
| 7 | **Kubernetes** | **T21** + **T22**; config via manifests/Helm (**T26**). ArgoCD / Terraform when org defines. |

**Out of scope for this repo** (unless product direction changes): event buses (Kafka/Redpanda), Protobuf/gRPC as a requirement, and Cassandra/Scylla as primary storage—they do not match the access-manager SQL model.
