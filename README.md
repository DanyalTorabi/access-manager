# access-manager

[![CI](https://github.com/DanyalTorabi/access-manager/actions/workflows/ci.yml/badge.svg)](https://github.com/DanyalTorabi/access-manager/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/DanyalTorabi/access-manager/graph/badge.svg)](https://codecov.io/gh/DanyalTorabi/access-manager)

Educational / product repo for **domain-scoped** access control (users, groups, resources, access-type bits, permissions). Implementations can live side by side: the **Go** HTTP service is under **[`go/`](go/)**; the HTTP contract lives under **[`api/`](api/)** (**T17** — OpenAPI + Postman).

## Repository layout

| Path | Purpose |
|------|---------|
| **[`go/`](go/)** | Go module `github.com/dtorabi/access-manager`: HTTP service, `internal/*`, SQLite migrations |
| [`api/`](api/) | OpenAPI 3 spec and Postman collection (**T17**); see [api/README.md](api/README.md) |
| [`plan/`](plan/) | Phased implementation plans per ticket |
| [`PLAN.md`](PLAN.md), [`TICKETS.md`](TICKETS.md) | Product goals and backlog |

**Import path** (Go): `github.com/dtorabi/access-manager/...` with **module root** = [`go/go.mod`](go/go.mod).

## Go service quickstart

From the **repository root**, `make` delegates to `go/`:

```bash
git clone https://github.com/DanyalTorabi/access-manager.git
cd access-manager
make test
make run
```

Or work **inside the module**:

```bash
cd go
go mod download
go run ./cmd/server
```

Details: **[go/README.md](go/README.md)** (config, env, API overview, `make` targets).

**API auth (T7):** optional static Bearer token via **`API_BEARER_TOKEN`** / **`api_bearer_token`** protects **`/api/v1/*`**; **`/health`** stays public. If you listen on a non-loopback address without a token, the process logs a startup warning—set a token before any real exposure.

## Docker (T19)

Multi-stage image (distroless, non-root, `CGO_ENABLED=0` / `modernc.org/sqlite`) built from repo root; SQLite data uses a **tmpfs** mount (ephemeral) in the default compose file.

From the **repository root**, use **`make`** (same idea as `make test` / `make run`):

| Target | What it runs |
|--------|----------------|
| `make docker-build` | `docker compose build` |
| `make docker-up` | `docker compose up -d` (detached) |
| `make docker-logs` | `docker compose logs -f` (follow app logs) |
| `make docker-down` | `docker compose down` |
| `make e2e` | **[T16]** — `go test -race -count=1 -tags=e2e ./e2e/...` against **`BASE_URL`** (server must be up; optional **`make e2e-bash`**) |

```bash
make docker-build
make docker-up
curl -s http://127.0.0.1:8080/health
BASE_URL=http://127.0.0.1:8080 make e2e
# optional: make e2e-bash
make docker-down
```

`docker-compose.yml` publishes **`127.0.0.1:8080:8080`** only (not all interfaces on the host). The process listens on **`0.0.0.0:8080`** inside the container so port mapping works.

- **[Dockerfile](Dockerfile)** — build context `.`, copies **`go/`**  
- **[docker-compose.yml](docker-compose.yml)** — `app` service  
- **[.dockerignore](.dockerignore)**

## CI / GitHub Actions (T13)

Workflow: **[`.github/workflows/ci.yml`](.github/workflows/ci.yml)** runs on **pull requests** and **pushes** to **`main`**:

| Job | What it does |
|-----|----------------|
| **Go** | From **`go/`**: `go test -race ./...`, `go vet ./...`, **golangci-lint** (same pin as `go/Makefile`) |
| **Docker** | `docker compose build`, `compose up`, **`curl /health`**, **Go E2E** (`go test -race -count=1 -tags=e2e ./e2e/...`), `compose down` |
| **Publish** | On **`push` to `main`** only: push image to **`ghcr.io/<owner>/access-manager`** as **`latest`** and **`sha-<commit>`** (`packages: write` on `GITHUB_TOKEN`) |

Enable **Actions** for the repo; for **branch protection**, add required checks matching the workflow **job** names after the first green run (e.g. **Go (test, vet, lint)** and **Docker build & compose smoke**) — see [CONTRIBUTING.md](CONTRIBUTING.md).

Optional: run workflows locally with **[nektos/act](https://github.com/nektos/act)** (limited parity vs GitHub runners).

## Branching and pull requests (T14)

All changes should go to **`main`** via **pull requests**. Branch naming, merge policy, and how this ties to CI (**T13**) and branch protection (**T6**) are documented in **[docs/branching.md](docs/branching.md)**.

## Contributing

See **[CONTRIBUTING.md](CONTRIBUTING.md)** for local setup, PR expectations, **`gh`** usage, and the **maintainer checklist** (remote, branch protection, Actions/GHCR) for **T6**.

## Docs and planning

- [PLAN.md](PLAN.md) — product goals and milestones  
- [TICKETS.md](TICKETS.md) — backlog and curriculum alignment table  
- [plan/README.md](plan/README.md) — phased implementation plans per ticket  
- [docs/branching.md](docs/branching.md) — branches and PRs to `main` (**T14**)  
- [CONTRIBUTING.md](CONTRIBUTING.md) — contributor guide and GitHub hygiene (**T6**)  
- [AGENTS.md](AGENTS.md) — contributor rules for humans and AI  
- [`.github/workflows/ci.yml`](.github/workflows/ci.yml) — CI + GHCR (**T13**)  
- [CHANGELOG.md](CHANGELOG.md) — release notes (**T15**); see below  
- [api/README.md](api/README.md) — OpenAPI + Postman import and variables (**T17**)  

### Changelog (**T15**)

User-facing or notable changes belong under **`## [Unreleased]`** in [CHANGELOG.md](CHANGELOG.md), using [Keep a Changelog](https://keepachangelog.com/) sections (**Added**, **Changed**, **Fixed**, **Removed**, **Security**). Merge each PR that needs a note before release; when you cut a release, move the bullets into a new `## [x.y.z] - YYYY-MM-DD` section and create a matching Git tag (`v0.1.0`, etc.).

## License

Add a `LICENSE` file when you publish the repository.
