# Branching strategy

Work lands on **`main`** through **pull requests**. Once GitHub Actions CI is configured, CI runs on PRs; with branch protection enabled, direct pushes to `main` are blocked.

## Model

- **Trunk-based:** `main` is the integration branch; use **short-lived** topic branches.
- Prefer **no direct pushes to `main`** once branch protection is on. Until then, still use PRs when you can for review and history.

## Branch naming

Use **author / prefix / short-kebab-description** (all lowercase):

| Segment | Rule |
|---------|------|
| `author/` | Short personal identifier (e.g. first name or username), so reviewers know who owns the branch |
| `prefix/` | One of the prefixes below |
| description | Lowercase kebab-case; optionally include a ticket id |

| Prefix | Use |
|--------|-----|
| `feature/` | New behavior or larger changes, e.g. `danyal/feature/t13-ci-workflow` |
| `fix/` | Bug fixes, e.g. `danyal/fix/authz-empty-domain` |
| `docs/` | Documentation only, e.g. `danyal/docs/t14-branching` |
| `chore/` | Tooling, CI config, deps without product behavior change, e.g. `danyal/chore/lint-config` |

## Pull requests

1. Branch from up-to-date **`main`**.
2. Open a PR **into `main`** with a clear title and the **[pull request template](../.github/pull_request_template.md)** (**Summary**, **Ticket**, **Checklist**). From the CLI: **`gh pr create --base main`** — see **[CONTRIBUTING.md](../CONTRIBUTING.md)**.
3. **Squash merge** is the suggested default on GitHub (repo setting) if you want a linear history; merge commit is fine if the team prefers.
4. Fix failing CI checks before merge.

## Releases and long-lived branches

Release branches and version tags are **out of scope** until a CHANGELOG / semver process is in place.

## See also

- [CONTRIBUTING.md](../CONTRIBUTING.md) — maintainer checklist: remote, branch protection, Actions/GHCR
- [GitHub Issues](https://github.com/DanyalTorabi/access-manager/issues) — backlog  
- [docs/backend-curriculum.md](backend-curriculum.md) — curriculum ↔ repo (not a ticket list)
