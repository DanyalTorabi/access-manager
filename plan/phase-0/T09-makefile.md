# T9 — Makefile

## Ticket

**T9** — Makefile (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 0** — AI context and local ergonomics

## Goal

Provide **one command surface** for build, test (with race), coverage, run, mod tidy, and lint—so humans and AI use the same invocations.

## Deliverables

- **`Makefile`** at repo root (may delegate to **`go/Makefile`** after **T29**); documented targets.

## Steps

1. **`build`:** `go build -o bin/server ./cmd/server` (ensure `bin/` in `.gitignore` if not already).
2. **`test`:** `go test -race ./...`
3. **`cover`:** `go test -race -coverprofile=coverage.out ./...` (optional `go tool cover -html=coverage.out`).
4. **`run` / `serve`:** `go run ./cmd/server`
5. **`tidy`:** `go mod tidy`
6. **`lint`:** `golangci-lint run ./...` (**requires T28** `.golangci.yml`).
7. Add `.PHONY` for all targets; document in README.

## Files / paths

- **Create:** `Makefile` (and **`go/Makefile`** when the module lives under **`go/`**)
- **Edit:** [README.md](../../README.md), [.gitignore](../../.gitignore) if `bin/` needed

## Acceptance criteria

- `make test` and `make lint` succeed on a clean tree after T28 is done.
- `make build` produces a runnable `bin/server`.

## Out of scope

- Docker targets (T19); `act` / CI (T13).

## Dependencies

- **T28** must be complete so `make lint` is meaningful.

## Curriculum link

**Themes 1, 4, 6** — standard commands, race detector, CI parity.

**Implementation order in phase 0:** T18 → T8 → T28 → T9.
