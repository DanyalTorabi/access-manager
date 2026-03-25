# AGENTS.md — access-manager

Guidance for human and AI contributors. Phased work: [plan/README.md](plan/README.md). Product context: [PLAN.md](PLAN.md), backlog: [TICKETS.md](TICKETS.md).

## Module

- **Path:** `github.com/dtorabi/access-manager`
- **Go:** see `go.mod` for version / toolchain.

## Layout

| Area | Role |
|------|------|
| `cmd/server` | Process entry: config (env today), DB open, migrate, HTTP listen |
| `internal/api` | HTTP routes and handlers (chi)—**no** core business rules here |
| `internal/access` | Pure access-mask helpers and domain-friendly logic (library-oriented) |
| `internal/store` | Persistence **interfaces** and shared types |
| `internal/store/sqlite` | SQLite implementation of `store.Store` |
| `internal/database` | Driver selection and migration entry for `cmd` |
| `migrations/sqlite` | SQL migrations |

## Security

- Never commit API keys, passwords, or tokens. Use env / secret managers; see `.env.example` (no real values).
- Default to **127.0.0.1** for dev HTTP. Do not expose internal admin APIs to `0.0.0.0` without auth and a documented requirement.

## Library vs service

Treat the **access model** (`internal/access`, `internal/store` contracts) as a **future standalone library**: keep it free of chi/HTTP imports. Wire HTTP only in `internal/api` and `cmd/server`. When we need other repos to import it, we will add **`pkg/...`** and move or expose APIs there (`internal/` cannot be imported by external modules).

## Future work and TODO comments

- Anything we **defer** (“do later”, “future work”) must have a **ticket** in [TICKETS.md](TICKETS.md) (or your tracker). **Do not** leave bare TODOs with no tracking.
- In code or docs, reference the ticket explicitly, e.g. `// TODO(T14): ...` or `See TICKETS.md T23`.
- **Prefer updating an existing open ticket** when the follow-up fits that scope; avoid spinning a new ticket for every micro-item unless it is genuinely separate work.

## After you finish a task (lightweight)

Match effort to change size.

1. **Docs** — If behavior or setup changed: update **README**. If [CHANGELOG.md](CHANGELOG.md) exists and the change is user-visible, add an **Unreleased** entry; otherwise skip.
2. **Tests** — `go test -race ./...` on affected packages, or `make test` when the Makefile exists (T9).
3. **Lint** — `golangci-lint run ./...` or `make lint` when configured (T28/T9).
4. **Coverage** — Optional for tiny edits; use `make cover` for larger or risky changes (T12).

## Commands (until Makefile lands)

```bash
go test -race ./...
go vet ./...
```

After Phase 0 T9/T28: prefer `make test`, `make lint`, `make cover`, `make run`.
