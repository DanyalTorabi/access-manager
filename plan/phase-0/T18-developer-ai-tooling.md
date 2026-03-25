# T18 — Developer AI / editor tooling

## Ticket

**T18** — Developer AI / editor tooling (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 0** — AI context and local ergonomics

## Goal

Give humans and AI assistants consistent, repo-specific rules: layout, module path, security defaults, and how to run checks—so generated changes match project conventions.

## Deliverables

- Cursor rules under `.cursor/rules/` (or equivalent scoped rule files).
- Root **`AGENTS.md`** (or `docs/AGENTS.md` if you prefer less root clutter—pick one and document in README).
- Short **tech stack** blurb: Go module, chi, SQLite/modernc, no secrets in repo.

## Steps

1. Add `.cursor/rules` entry (or `.mdc`) describing: `internal/` is app code; extend existing patterns; never commit credentials; default bind loopback for dev.
2. Add `AGENTS.md` with: module path `github.com/dtorabi/access-manager`, directory map (`cmd/server`, `internal/api`, `internal/store`, `migrations/`), link to [PLAN.md](../../PLAN.md) and [TICKETS.md](../../TICKETS.md).
3. Note commands: after T9/T28, prefer `make test`, `make lint`; until then `go test -race ./...`.
4. Optional: link to [plan/README.md](../README.md) for phased implementation.

## Files / paths

- **Create:** `.cursor/rules/*.md` or `.mdc`, `AGENTS.md` (or `docs/AGENTS.md` + README pointer)

## Acceptance criteria

- New contributor (or AI) can open `AGENTS.md` and understand layout and security rules in under two minutes.
- Rules explicitly say: no API keys/passwords in source; use env / secret stores.

## Out of scope

- Implementing T26 config loader, T27 shutdown, or any production auth—only document boundaries.

## Dependencies

- None (first in phase 0).

## Curriculum link

**Theme 1 (Go)** — consistent project structure and discipline for tooling-assisted development.
