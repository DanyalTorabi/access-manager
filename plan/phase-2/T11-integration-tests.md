# T11 — Integration tests

## Ticket

**T11** — Integration tests (GitHub [#22](https://github.com/DanyalTorabi/access-manager/issues/22))

## Phase

**Phase 2** — Tests and coverage

## Goal

Verify **HTTP + SQLite + migrations** together: real `chi` router, real store, temp DB file or memory—mirroring production wiring.

## Deliverables

- Integration test package that starts app wiring (or uses `httptest` against full router + migrated DB).
- Reuse migration path resolution pattern from [internal/store/sqlite/store_test.go](../../go/internal/store/sqlite/store_test.go).

## Steps

1. Extract helper: `newIntegrationServer(t)` → `*api.Server` + cleanup.
2. Run golden-path: create domain → user → resource → permission → authz check.
3. Optional: run under build tag `integration` if runtime is slow; document `go test -tags=integration ./...`.
4. Document in README; align with **T19/T13** when compose runs DB for Postgres later.

## Files / paths

- **Create:** e.g. `internal/api/integration_test.go` or `test/integration/...`
- **Edit:** [Makefile](../../Makefile) optional target `test-integration`

## Acceptance criteria

- One integration test proves end-to-end HTTP authz flow against real SQLite migrations.

## Out of scope

- CI compose job (T13); Postgres (T1).

## Dependencies

- **T10** optional; **Phase 0** Makefile helps standardize commands.

## Curriculum link

**Themes 3–4** — DB-backed behavior; prerequisite for CI integration stage.
