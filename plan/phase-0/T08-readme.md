# T8 — README

## Ticket

**T8** — README (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 0** — AI context and local ergonomics

## Goal

Add a root **README.md** so anyone can clone, configure env, run the server, and run tests—aligned with Makefile and future config file (T26).

## Deliverables

- **`README.md`** at repository root with prerequisites, quickstart, env vars, and one health-check example.

## Steps

1. Document **Go version** (match `go` directive in [go.mod](../../go.mod); note toolchain if used).
2. **Build/run:** `go run ./cmd/server` or `make run` after T9.
3. **Environment:** `DATABASE_DRIVER`, `DATABASE_URL`, `HTTP_ADDR`, `MIGRATIONS_DIR` (see [cmd/server/main.go](../../cmd/server/main.go)).
4. **Smoke test:** `curl` example for `GET /health`.
5. Link [TICKETS.md](../../TICKETS.md) curriculum table and [plan/README.md](../README.md).
6. State that **structured config file** is introduced in **T26** (until then, env only).

## Files / paths

- **Create:** `README.md`
- **Edit:** none required beyond README (optional one-line in PLAN pointing to README)

## Acceptance criteria

- Following README from a clean clone yields a running server and passing `go test ./...` (or `make test` after T9).
- No secrets or real DSNs with credentials in examples.

## Out of scope

- Full OpenAPI/API reference (T17); Docker instructions (T19); CI badges (T13).

## Dependencies

- **T18** recommended first so README can link `AGENTS.md` / Cursor rules.

## Curriculum link

**Themes 1–2** — runnable app and documentation for local/Docker prep later.
