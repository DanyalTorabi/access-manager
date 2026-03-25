# T18 — Developer AI / editor tooling

## Ticket

**T18** — Developer AI / editor tooling (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 0** — AI context and local ergonomics

## Goal

Give humans and AI assistants consistent, repo-specific rules: layout, module path, security defaults, how to run checks, and **how we treat the core as a reusable library**—without heavy process for small changes.

## Deliverables

- Cursor rules under `.cursor/rules/` (scoped `.mdc`).
- Root **`AGENTS.md`**: module map, security, **lightweight best practices**, **post-task checklist**, **library vs app boundaries**.
- Short **tech stack** blurb: Go module, chi, SQLite/modernc, no secrets in repo.

## Best practices (lightweight)

- Prefer **small, focused changes**; match existing style; no drive-by refactors.
- **Security:** no credentials in source; loopback bind for dev; validate/sanitize inputs (see workspace rules).
- **Library mindset:** business rules and persistence contracts live in layers that **do not** import `chi` or HTTP types; handlers stay thin. See **Library boundary** below.
- **Proportionality:** trivial fixes (typo, one-liner) need only quick `go test` on affected packages—not a full release checklist.
- **Deferred work:** Anything left for later must have a **ticket** in TICKETS.md. Put the ticket id in the comment (e.g. `TODO(T17): ...`). **Update an existing open ticket** when the follow-up belongs there; avoid a new ticket per tiny item unless it is separate work.

## After each task (when applicable)

Complete what exists today; do not block on missing tooling.

| Step | When |
|------|------|
| **Docs** | If behavior, env vars, or developer workflow changed: update **README** (T8). If the change is user-visible and [CHANGELOG.md](../../CHANGELOG.md) exists (T15), add an **Unreleased** bullet; otherwise a one-line README note is enough. |
| **Tests** | Run **`go test ./...`** or **`go test -race ./...`** on touched packages; use **`make test`** once T9 exists. |
| **Lint** | Run **`golangci-lint run ./...`** or **`make lint`** once T28/T9 exist. |
| **Coverage** | Optional for tiny changes; use **`make cover`** (T9/T12) before larger merges or when touching critical paths. |

## Library boundary (standalone core)

**Intent:** The **access control model** (types, masks, evaluation helpers, store **interfaces**) should be usable as a **Go library** in another binary or repo later—without dragging the HTTP server.

- **Today:** Core logic lives under [`internal/access`](../../go/internal/access) and contracts under [`internal/store`](../../go/internal/store). **`internal/`** cannot be imported by **other modules**; only this module’s packages can import it.
- **Direction (incremental):** When the API stabilizes, introduce **`pkg/...`** (e.g. `pkg/accesscore` for pure domain + bitmask; `pkg/accessmanager` for `Store` interfaces and service façade) and move or re-export there. **Do not** do a big-bang move in a single “simple task”—track a dedicated follow-up if needed.
- **Until `pkg/` exists:** Add new domain/store logic in `internal/access` and `internal/store` as if it will move: **no** chi/router imports in those packages; keep HTTP-only code in [`internal/api`](../../go/internal/api) and wiring in [`cmd/server`](../../go/cmd/server).
- **SQLite / drivers:** Concrete DB code may stay `internal/store/sqlite` or move to `pkg/.../sqlite` later; [`internal/database`](../../go/internal/database) stays wiring for `cmd`.

## Steps

1. Add `.cursor/rules/access-manager.mdc` (alwaysApply or `**/*.go` globs): security, library boundary, post-task checklist summary.
2. Add root `AGENTS.md`: module path `github.com/dtorabi/access-manager`, directory map, links to [PLAN.md](../../PLAN.md), [TICKETS.md](../../TICKETS.md), [plan/README.md](../README.md).
3. Document commands: until T9/T28, `go test -race ./...`; after T9, prefer `make test`, `make lint`, `make cover`.
4. Call out **library-first** layering and future `pkg/` migration in both files.

## Files / paths

- **Create:** `.cursor/rules/access-manager.mdc`, `AGENTS.md`

## Acceptance criteria

- New contributor (or AI) can open `AGENTS.md` and understand layout, security, **after-task steps**, and **library vs HTTP** split in under a few minutes.
- Cursor rules reinforce the same without duplicating entire TICKETS.

## Out of scope

- Actually moving packages to `pkg/` (separate focused change unless trivial).
- T26 config loader, T27 shutdown, full README/Makefile (T8/T9)—only reference them.

## Dependencies

- None (first in phase 0).

## Curriculum link

**Theme 1 (Go)** — consistent structure and discipline for tooling-assisted development.

**Implementation order in phase 0:** T18 → T8 → T28 → T9.
