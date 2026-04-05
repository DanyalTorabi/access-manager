# T6 — GitHub remote + repo hygiene

## Ticket

**T6** — GitHub remote + repo hygiene (GitHub [#17](https://github.com/DanyalTorabi/access-manager/issues/17))

## Phase

**Phase 3** — GitHub, Docker, CI

## Goal

Connect the local repo to **GitHub**, protect **main**, and enable **GHCR** publishing for T13.

## Deliverables

- [x] `origin` / canonical URL documented ([CONTRIBUTING.md](../../CONTRIBUTING.md)); maintainer adds or verifies remote locally.
- [ ] Branch protection on `main` — **GitHub Settings** (steps in CONTRIBUTING).
- [ ] **GHCR** `packages: write` — in **T13** workflow when images are published.
- [x] Issue/PR templates ([`.github/`](../../.github/)), **CONTRIBUTING.md**.

## Steps

1. [x] Create GitHub repo; `git remote add origin …`; push `main`.
2. [ ] Settings → Branches → protect **main** (contributor completes in GitHub).
3. [ ] Enable **Actions**; GHCR permissions when **T13** workflow exists.
4. [x] Issue/PR templates, **CONTRIBUTING.md**.

## Files / paths

- **Edit:** none required in repo for remote itself; optional `.github/ISSUE_TEMPLATE/*`

## Acceptance criteria

- `git push origin main` works; T13 workflow can run on PR and push.

## Out of scope

- Org-wide SSO policies; secrets for third-party deploy.

## Dependencies

- **T14** doc ready for contributors.
- **T29** monorepo **`go/`** layout in place (recommended before first push so the default tree matches CI and docs).

## Curriculum link

**Theme 4** — remote + automation host.
