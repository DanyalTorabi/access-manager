# End-to-end tests

## Go suite (primary)

The comprehensive E2E test suite lives in **`go/e2e/`** and covers:

- Full CRUD lifecycles for every entity type (domains, users, groups, resources, access types, permissions)
- Authz challenge scenarios (direct, group-inherited, no-permission, multiple masks, revoke-and-recheck)
- Pagination journeys
- Error path journeys (not-found, invalid JSON, duplicates, referential integrity, bad pagination)
- Bearer auth journeys (when `API_BEARER_TOKEN` is set)

Run from repo root:

```bash
make e2e
```

Or from `go/`:

```bash
go test -tags=e2e -race -count=1 ./e2e/...
```

Requires a running server on `BASE_URL` (default `http://127.0.0.1:8080`).
Set `API_BEARER_TOKEN` when the server enforces bearer auth.

## Bash script (deprecated)

`test/e2e/bash/run.sh` is a minimal curl+jq smoke test kept for reference.
The Go suite above is now the primary E2E mechanism; the bash script may be
removed in a future PR.
