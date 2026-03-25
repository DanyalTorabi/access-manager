# T13 — CI/CD (GitHub Actions)

## Ticket

**T13** — CI/CD (curriculum-aligned) (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 3** — GitHub, Docker, CI

## Goal

On **pull requests** to **main**: run **unit tests**, **integration tests** (compose if applicable), **`go vet`**, **golangci-lint**, and **build Docker image**. On **merge to main**: **publish image to ghcr.io** when all jobs pass.

## Deliverables

- `.github/workflows/ci.yml` (or split PR vs release).
- Ubuntu runners; Go version aligned with [`go/go.mod`](../../go/go.mod) (run jobs with **`defaults.run.working-directory: go`** or equivalent).
- Cache `~/go/pkg/mod`.

## Steps

1. **PR job:** checkout, setup-go, `go test -race ./...`, `go vet ./...`, `golangci-lint run`, `docker build`.
2. **Integration:** `docker compose up -d` (wait for health), run `go test -tags=integration ./...` or dedicated script; `compose down`.
3. **Main branch job:** build and push to `ghcr.io/<org>/<repo>:tag` (semver or `sha-<short>`).
4. Document required secrets (usually none for GHCR with `GITHUB_TOKEN`).
5. Optional: [nektos/act](https://github.com/nektos/act) note in README.

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
