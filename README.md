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

## Branching and pull requests (T14)

All changes should go to **`main`** via **pull requests**. Branch naming, merge policy, and how this ties to CI (**T13**) and branch protection (**T6**) are documented in **[docs/branching.md](docs/branching.md)**.

## Docs and planning

- [PLAN.md](PLAN.md) — product goals and milestones  
- [TICKETS.md](TICKETS.md) — backlog and curriculum alignment table  
- [plan/README.md](plan/README.md) — phased implementation plans per ticket  
- [docs/branching.md](docs/branching.md) — branches and PRs to `main` (**T14**)  
- [AGENTS.md](AGENTS.md) — contributor rules for humans and AI  

## License

Add a `LICENSE` file when you publish the repository.
