# T29 — Monorepo layout: Go under `go/`

## Ticket

**T29** — Monorepo layout: Go implementation lives under **`go/`** (GitHub [#40](https://github.com/DanyalTorabi/access-manager/issues/40))

## Phase

**Phase 3** — GitHub, Docker, CI (do **before** T6 remote / T13 CI so the default tree is stable)

## Goal

Prepare a **multi-language–friendly** repository: shared docs and plans at the root; the **Go service** isolated under **`go/`** with its own `go.mod`, `Makefile`, and tooling. Import path stays **`github.com/dtorabi/access-manager`** (module root = `go/`).

## Deliverables

- [x] `go/` contains: `cmd/`, `internal/`, `migrations/`, `go.mod`, `go.sum`, `Makefile`, `.golangci.yml`, `config.example.yaml`, `.env.example`
- [x] Root **`Makefile`** forwards targets to `$(MAKE) -C go …`
- [x] Root **README** describes monorepo layout; Go quickstart uses `cd go` or `make` from root
- [x] **AGENTS.md**, **PLAN.md**, plan links updated for `go/` paths
- [ ] Optional later: `spec/` (OpenAPI, fixtures), other language roots (`rust/`, …) at repo root

## Steps

1. `mkdir go` and `git mv` the Go module files and directories into `go/`.
2. Add root Makefile delegating to `go/Makefile`.
3. Update documentation and plan relative links (`go/internal/...`, `go/go.mod`).
4. Run `make test` and `make lint` from repo root.

## Acceptance criteria

- `make test`, `make run`, `make cover` from **repository root** succeed.
- `go test -race ./...` from **`go/`** succeeds (module root).

## Dependencies

- None (can precede **T14**, **T6**, **T13**).

## Out of scope

- Adding non-Go implementations; CI matrix (T13) beyond path updates.

## Curriculum link

**Theme 4** — clear repo shape before remote and automation.
