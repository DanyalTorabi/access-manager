# Branching strategy (T14)

Work lands on **`main`** through **pull requests**. After **T13** (GitHub Actions), CI runs on PRs; after **T6**, branch protection should block direct pushes to `main`.

## Model

- **Trunk-based:** `main` is the integration branch; use **short-lived** topic branches.
- Prefer **no direct pushes to `main`** once branch protection is on (**T6**). Until then, still use PRs when you can for review and history.

## Branch naming

Use **prefix / short-kebab-description**:

| Prefix | Use |
|--------|-----|
| `feature/` | New behavior or larger changes, e.g. `feature/T13-ci-workflow`, `feature/openapi-spec` |
| `fix/` | Bug fixes, e.g. `fix/authz-empty-domain` |
| `docs/` | Documentation only, e.g. `docs/t14-branching` |
| `chore/` | Tooling, CI config, deps without product behavior change, e.g. `chore/lint-config` |

Optional: include a ticket id (`feature/T19-docker`).

## Pull requests

1. Branch from up-to-date **`main`**.
2. Open a PR **into `main`** with a clear title and the **[pull request template](../.github/pull_request_template.md)** (**Summary**, **Ticket**, **Checklist**). From the CLI: **`gh pr create --base main`** — see **[CONTRIBUTING.md](../CONTRIBUTING.md)**.
3. **Squash merge** is the suggested default on GitHub (repo setting) if you want a linear history; merge commit is fine if the team prefers.
4. After **T13**, fix failing checks before merge.

## Releases and long-lived branches

Release branches and version tags are **out of scope** until **T15** (CHANGELOG / semver) if you need them.

## See also

- [CONTRIBUTING.md](../CONTRIBUTING.md) — maintainer checklist: remote, branch protection, Actions/GHCR (**T6**)
- [TICKETS.md](../TICKETS.md) — **T6** (remote + protection), **T13** (CI)
- [plan/phase-3/T14-branching-strategy.md](../plan/phase-3/T14-branching-strategy.md)
