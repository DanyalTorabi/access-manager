# AGENTS.md — access-manager

Guidance for human and AI contributors. Phased work: [plan/README.md](plan/README.md). Product context: [PLAN.md](PLAN.md), backlog: [TICKETS.md](TICKETS.md). Branching: [docs/branching.md](docs/branching.md) (**T14**). Contributing + GitHub: [CONTRIBUTING.md](CONTRIBUTING.md) (**T6**).

## Repository shape

- **Root:** plans, tickets, product docs; optional future **`spec/`** and non-Go implementations.
- **Go service and module:** everything under **[`go/`](go/)** (**T29**).

## Go module

- **Path:** `github.com/dtorabi/access-manager`
- **Module root:** [`go/go.mod`](go/go.mod) (run **`go`** / **`make`** for Go work from **`go/`**, or **`make`** from repo root via delegating Makefile).

## Layout (under `go/`)

| Area | Role |
|------|------|
| `go/cmd/server` | Process entry: config (env today), DB open, migrate, HTTP listen |
| `go/internal/api` | HTTP routes and handlers (chi)—**no** core business rules here |
| `go/internal/access` | Pure access-mask helpers and domain-friendly logic (library-oriented) |
| `go/internal/store` | Persistence **interfaces** and shared types |
| `go/internal/store/sqlite` | SQLite implementation of `store.Store` |
| `go/internal/database` | Driver selection and migration entry for `cmd` |
| `go/migrations/sqlite` | SQL migrations |

## Security

- Never commit API keys, passwords, or tokens. Use env / secret managers; see [`go/.env.example`](go/.env.example) (no real values).
- Default to **127.0.0.1** for dev HTTP. Do not expose internal admin APIs to `0.0.0.0` without auth and a documented requirement.

## Library vs service

Treat the **access model** (`internal/access`, `internal/store` contracts) as a **future standalone library**: keep it free of chi/HTTP imports. Wire HTTP only in `internal/api` and `cmd/server`. When we need other repos to import it, we will add **`pkg/...`** under **`go/`** and move or expose APIs there (`internal/` cannot be imported by external modules).

## No unused code

Do not add functions, methods, or interfaces that have no callers in the current codebase. If a reviewer (human or Copilot) suggests speculative code (e.g. "consider implementing X for future compatibility"), **defer** it to a ticket and reply on the review thread — do not merge dead code. The only exception is when building a module whose public API is defined by a plan; in that case, reference the plan in a code comment (e.g. `// See plan/phase-N/Txx.md`).

## Future work and TODO comments

- Anything we **defer** (“do later”, “future work”) must have a **ticket** in [TICKETS.md](TICKETS.md) (or your tracker). **Do not** leave bare TODOs with no tracking.
- In code or docs, reference the ticket explicitly, e.g. `// TODO(T14): ...` or `See TICKETS.md T23`.
- **Prefer updating an existing open ticket** when the follow-up fits that scope; avoid spinning a new ticket for every micro-item unless it is genuinely separate work.

### Pull request / Copilot follow-ups

If a review comment is correct but **out of scope for the current PR**, say on the thread that it will be handled in **Txx** (specific ticket id), and add a **note** to that ticket’s [plan/](plan/) file (e.g. under Steps or Deliverables) so the item is scheduled and traceable.

## Before you open a PR (self-review)

Run a short **reviewer-style pass** on your diff before opening the PR. Checklist: **[CONTRIBUTING.md — Before you open a PR (self-review)](CONTRIBUTING.md#before-you-open-a-pr-self-review)** (tests, e2e compile-check, docs/CI alignment, secrets, build tags and test rigor).

## After you finish a task (lightweight)

Match effort to change size.

1. **Docs** — If behavior or setup changed: update **[`go/README.md`](go/README.md)** and root **[README.md](README.md)** if layout/entrypoints changed. If [CHANGELOG.md](CHANGELOG.md) exists and the change is user-visible, add an **Unreleased** entry; otherwise skip.
2. **Tests** — Add or extend **unit/integration tests** for behavioral changes in the same change set (T10). Run `go test -race ./...` from **`go/`** on affected packages, or **`make test`** from repo root (T9).
3. **Lint** — `golangci-lint run ./...` from **`go/`** or **`make lint`** from root (T28/T9).
4. **Coverage** — Optional for tiny edits; use **`make cover`** for larger or risky changes (T12).

## Commits and pull requests (AI assistants)

By default, **do not** run `git commit`, `git push`, or `gh pr create`. Provide a **proposed commit message** and **PR body** (Summary, Ticket, Checklist per [`.github/pull_request_template.md`](.github/pull_request_template.md)) for the human to paste. Run those git/`gh` commands only when the human **explicitly** asks you to.

## GitHub

**Humans:** Prefer the **`gh`** CLI for issues, PRs, Actions, and API tasks when scripting. Requires [GitHub CLI](https://cli.github.com/) and `gh auth login`. Maintainer steps: [CONTRIBUTING.md](CONTRIBUTING.md) (**T6**). CI: [`.github/workflows/ci.yml`](.github/workflows/ci.yml) (**T13**); **`gh run list`** / **`gh pr checks`**.

## Commands

From **repository root:** `make test`, `make lint`, `make cover`, `make run` (delegate to **`go/`**); **`make docker-build`**, **`make docker-up`**, **`make docker-logs`**, **`make docker-down`** for **Docker** (**T19**, repo root only).

From **`go/`:** same Go targets via [go/Makefile](go/Makefile), or plain `go test -race ./...`, `go vet ./...`.

Install **golangci-lint** v2 for `make lint` (see [go/README.md](go/README.md)).
