# access-manager

Educational / product repo for **domain-scoped** access control (users, groups, resources, access-type bits, permissions). Implementations can live side by side: the **Go** HTTP service is under **[`go/`](go/)**; shared contracts and other language trees may be added at the root later (**T29**, **T17** / `spec/`).

## Repository layout

| Path | Purpose |
|------|---------|
| **[`go/`](go/)** | Go module `github.com/dtorabi/access-manager`: HTTP service, `internal/*`, SQLite migrations |
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

## Docker (T19)

Multi-stage image (distroless, non-root, `CGO_ENABLED=0` / `modernc.org/sqlite`) built from repo root; SQLite data uses a **tmpfs** mount (ephemeral) in the default compose file.

From the **repository root**, use **`make`** (same idea as `make test` / `make run`):

| Target | What it runs |
|--------|----------------|
| `make docker-build` | `docker compose build` |
| `make docker-up` | `docker compose up -d` (detached) |
| `make docker-logs` | `docker compose logs -f` (follow app logs) |
| `make docker-down` | `docker compose down` |

```bash
make docker-build
make docker-up
curl -s http://127.0.0.1:8080/health
make docker-down
```

`docker-compose.yml` publishes **`127.0.0.1:8080:8080`** only (not all interfaces on the host). The process listens on **`0.0.0.0:8080`** inside the container so port mapping works.

- **[Dockerfile](Dockerfile)** — build context `.`, copies **`go/`**  
- **[docker-compose.yml](docker-compose.yml)** — `app` service  
- **[.dockerignore](.dockerignore)**

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

## License

Add a `LICENSE` file when you publish the repository.
