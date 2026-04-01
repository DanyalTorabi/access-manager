# T13 — CI/CD (GitHub Actions)

## Ticket

**T13** — CI/CD (curriculum-aligned)

## Phase

**Phase 3** — GitHub, Docker, CI

## Goal

On **pull requests** to **main**: run **unit tests**, **integration tests** (compose if applicable), **`go vet`**, **golangci-lint**, and **build Docker image**. On **merge to main**: **publish image to ghcr.io** when all jobs pass.

## Deliverables

- [x] `.github/workflows/ci.yml` — PR + push **`main`**; publish job conditional.
- [x] Ubuntu runners; Go from [`go/go.mod`](../../go/go.mod); **`defaults.run.working-directory: go`** on Go job.
- [x] Module cache via **actions/setup-go** `cache-dependency-path: go/go.sum`.

## Steps

1. [x] **PR / push:** checkout, setup-go, `go test -race ./...`, `go vet ./...`, golangci-lint (`go run` pin v2.11.4), `docker compose` build + smoke **`/health`**.
2. [x] Compose smoke (no separate `-tags=integration` suite yet — covered by **Go** job tests + **health** check).
3. [x] **Push `main`:** build and push **`ghcr.io/<lowercase-owner>/<repo>:latest`** and **`:sha-<full>`**.
4. [x] Document: no extra secrets for GHCR (`GITHUB_TOKEN` in publish job).
5. [x] Optional **act** note in README.

## Files / paths

- **Create:** `.github/workflows/ci.yml` (and `release.yml` if split)

## Acceptance criteria

- Failing test or lint **blocks** merge (with branch protection T6).
- Green main produces a **published** GHCR image.

## Out of scope

- Deploy to Kubernetes (T21); staging/prod promotion (T22).

## Dependencies

- **T29** `go/` module root; **T6** GitHub remote; **T28** lint config; **T19** compose for integration; **T9** parity with local commands.

## Curriculum link

**Theme 4** — full pipeline on Ubuntu, GHCR publish.

**Order in phase 3:** **T29 → T14 → T6 → T19 → T13**.
