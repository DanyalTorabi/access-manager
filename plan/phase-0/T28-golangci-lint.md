# T28 — golangci-lint

## Ticket

**T28** — golangci-lint

## Phase

**Phase 0** — AI context and local ergonomics

## Goal

Add a **lint configuration** and a standard local command so code style and common bugs are caught before CI (T13).

## Deliverables

- **`.golangci.yml`** under **`go/`** (module root) with a **minimal** linter set appropriate for Go 1.22+.
- Document install: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` (or brew; pin version in README if team requires).

## Steps

1. Run `golangci-lint run ./...` on current codebase; fix or explicitly exclude only with comment in config.
2. Enable sensible defaults: e.g. `govet`, `staticcheck`, `errcheck`, `unused` (trim if noisy).
3. Exclude `vendor/` if present; keep timeouts reasonable for CI.
4. Note in README: run `make lint` after T9.

## Files / paths

- **Create:** `go/.golangci.yml`
- **Edit:** `README.md` (lint section), `Makefile` (T9 adds `lint` target)

## Acceptance criteria

- `golangci-lint run ./...` exits **0** on clean main.
- Config version field matches installed golangci-lint major version.

## Out of scope

- GitHub Actions wiring (T13); fixing every possible linter in the ecosystem—start minimal.

## Dependencies

- **T8** README should mention lint for contributors.

## Curriculum link

**Theme 4 (CI/CD)** — local parity with `go vet` / lint gate; full CI in T13.
