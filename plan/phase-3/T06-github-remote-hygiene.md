# T6 — GitHub remote + repo hygiene

## Ticket

**T6** — GitHub remote + repo hygiene (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 3** — GitHub, Docker, CI

## Goal

Connect the local repo to **GitHub**, protect **main**, and enable **GHCR** publishing for T13.

## Deliverables

- `origin` remote pointing at GitHub repo.
- Branch protection on `main` (require PR, optional required checks once T13 exists).
- **GitHub Packages / GHCR** permissions for Actions to push images.

## Steps

1. Create empty GitHub repo; `git remote add origin …`; push `main`/`master`.
2. Settings → Branches → protect default branch.
3. Enable **Actions**; for GHCR: use `GITHUB_TOKEN` with `packages: write` in workflow (document in T13).
4. Optional: issue/PR templates, `CONTRIBUTING.md`.

## Files / paths

- **Edit:** none required in repo for remote itself; optional `.github/ISSUE_TEMPLATE/*`

## Acceptance criteria

- `git push origin main` works; T13 workflow can run on PR and push.

## Out of scope

- Org-wide SSO policies; secrets for third-party deploy.

## Dependencies

- **T14** doc ready for contributors.

## Curriculum link

**Theme 4** — remote + automation host.
