# T26 — Config: file + env

## Ticket

**T26** — Config: file + env

## Phase

**Phase 1** — Runtime configuration and safe shutdown

## Goal

Load **ports, DSN, migrations path, and toggles** from a **config file** with **environment variable overrides**, without putting secrets in the repo.

## Deliverables

- Config file format (YAML or TOML) checked into repo as **example only** (e.g. `config.example.yaml`) with placeholder values.
- Loader in `cmd/server` or small `internal/config` package: merge file → env overrides.
- Document all keys in README.

## Steps

1. Define struct(s) for `HTTPAddr`, `DatabaseDriver`, `DatabaseURL`, `MigrationsDir`, future flags.
2. Parse file path from env e.g. `CONFIG_PATH` (optional); if unset, env-only mode compatible with current behavior.
3. **Precedence:** env overrides file for each field (document clearly).
4. Validate required fields; fail fast with clear errors (no secrets in error strings).
5. Add `config.example.yaml` (or `.toml`) to `.gitignore` entry for real `config.local.*` if needed.

## Files / paths

- **Create:** `config.example.yaml` (or `.toml`), `internal/config/config.go` (optional)
- **Edit:** [cmd/server/main.go](../../go/cmd/server/main.go), [README.md](../../README.md), [.env.example](../../go/.env.example) alignment

## Acceptance criteria

- Server starts with **only env** (backward compatible) or with **file + optional env overrides**.
- No committed file contains real passwords or production DSNs.

## Out of scope

- Kubernetes Secrets injection (T21/T22); full twelve-factor audit (T20).

## Dependencies

- **Phase 0** (README/Makefile) for documenting how to pass config.

## Curriculum link

**Themes 2–3** — portable config for Docker and databases.
