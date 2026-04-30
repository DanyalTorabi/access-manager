# T61 — Docker Compose services + integration-test targets for Postgres and MySQL

## Ticket

**T61** — Docker Compose services + integration-test targets for Postgres and MySQL (GitHub [#96](https://github.com/DanyalTorabi/access-manager/issues/96))

## Parent

**T1** ([#12](https://github.com/DanyalTorabi/access-manager/issues/12)) — [plan/phase-5/T01-per-dialect-migrations.md](../phase-5/T01-per-dialect-migrations.md)

## Phase

**Phase 7** — Multi-DB implementation (sub-tasks of T1)

## Goal

Add docker-compose services for Postgres and MySQL, Makefile targets to run integration tests against each, and extend the CI workflow to run those integration tests on every PR.

## Deliverables

- `docker-compose.yml` — add `postgres` and `mysql` services with health checks
- `go/Makefile` — `test-integration-postgres` and `test-integration-mysql` targets
- Root `Makefile` — delegate `test-integration-postgres` / `test-integration-mysql` from root
- `.github/workflows/ci.yml` — new job (or step) running integration tests against postgres and mysql using compose services
- `go/.env.example` — DSN env vars used by the integration-test Makefile targets

## Steps

1. **docker-compose services**: Add to `docker-compose.yml`:
   ```yaml
   postgres:
     image: postgres:15-alpine
     environment:
       POSTGRES_DB: access_test
       POSTGRES_USER: access
       POSTGRES_PASSWORD: access
     ports:
       - "5432:5432"
     healthcheck:
       test: ["CMD-SHELL", "pg_isready -U access"]
       interval: 5s
       retries: 10

   mysql:
     image: mysql:8
     environment:
       MYSQL_DATABASE: access_test
       MYSQL_ROOT_PASSWORD: access
     ports:
       - "3306:3306"
     healthcheck:
       test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-paccess"]
       interval: 5s
       retries: 10
   ```
2. **Makefile targets** (in `go/Makefile`):
   ```makefile
   test-integration-postgres:
       DATABASE_DSN_POSTGRES="postgres://access:access@localhost:5432/access_test?sslmode=disable" \
       go test -race -tags=integration -count=1 ./internal/store/postgres/...

   test-integration-mysql:
       DATABASE_DSN_MYSQL="root:access@tcp(localhost:3306)/access_test?parseTime=true&multiStatements=true" \
       go test -race -tags=integration -count=1 ./internal/store/mysql/...
   ```
3. **CI workflow**: Add an integration-test job that:
   - Uses `docker-compose up -d postgres mysql`
   - Waits for healthchecks (`docker-compose ps` or `--wait` flag)
   - Runs `make test-integration-postgres` and `make test-integration-mysql`
   - Tears down with `docker-compose down` in a `post` step
4. **Polling over sleep**: In any test helpers that wait for DB readiness (e.g. retry connection), use a deadline loop (`time.Now().Add(30 * time.Second)` + retry) rather than `time.Sleep`.

## Acceptance criteria

- `docker-compose up postgres mysql` starts both services and they pass healthchecks.
- `make test-integration-postgres` passes with a running postgres container.
- `make test-integration-mysql` passes with a running mysql container.
- CI passes with the new integration-test job on every push to `main` and on PRs.
- `make test` (unit only) still passes without any running containers.

## Files / paths

- **Edit:** `docker-compose.yml`
- **Edit:** `go/Makefile`, root `Makefile`
- **Edit:** `.github/workflows/ci.yml`
- **Edit:** `go/.env.example` (document DSN vars used by integration targets)

## Out of scope

- E2E tests against Postgres/MySQL backend (that is future T62+ work). This ticket focuses on store integration tests only.

## Dependencies

- **T56, T57** — migration SQL files.
- **T58, T59** — store packages under test.
- **T60** — driver wiring (for a combined end-to-end smoke option in CI, optional here).
