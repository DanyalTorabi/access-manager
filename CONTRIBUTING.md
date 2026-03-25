# Contributing

Thanks for contributing to **access-manager**. The Go service lives under [`go/`](go/).

## Before you start

- [**AGENTS.md**](AGENTS.md) — security, layout, library boundaries, post-change checks.
- [**docs/branching.md**](docs/branching.md) — branch names and **PRs to `main`** (**T14**).

## Local development

From the repository root:

```bash
make test
make lint
```

See [**go/README.md**](go/README.md) for config, environment variables, and the HTTP API.

### Docker (**T19**)

From the repository root:

```bash
make docker-build
make docker-up          # detached; then: make docker-logs
make docker-down
```

See the root [**README** — Docker (T19)](README.md#docker-t19) for image layout and port binding.

## Pull requests

- Open PRs **into `main`**; follow [docs/branching.md](docs/branching.md).
- Reference a ticket from [TICKETS.md](TICKETS.md) when it applies (e.g. `T19`).
- Keep **`make test`** and **`make lint`** green for Go changes.

## GitHub CLI

Prefer **`gh`** for issues, PRs, Actions, and API calls ([GitHub CLI](https://cli.github.com/)); run `gh auth status` if commands fail with auth errors.

---

## Maintainer checklist: GitHub (**T6**)

**Canonical remote:** [https://github.com/DanyalTorabi/access-manager](https://github.com/DanyalTorabi/access-manager)

### Clone and remotes

New contributors:

```bash
git clone https://github.com/DanyalTorabi/access-manager.git
cd access-manager
```

Maintainers: ensure **`origin`** points at the repo above (`git remote -v`). To add or fix:

```bash
git remote add origin https://github.com/DanyalTorabi/access-manager.git
# or: git@github.com:DanyalTorabi/access-manager.git
```

### Branch protection on `main`

In GitHub: **Settings → Branches → Branch protection rules** for `main`:

- Require a **pull request** before merging (disable direct pushes to `main`).
- Optionally require **approvals**.
- Require the **CI** workflow (`.github/workflows/ci.yml`) as a **required status check** once it passes on `main`.

### Actions and GHCR (**T13**)

- **Settings → Actions → General:** enable Actions as appropriate for the org.
- CI workflow: **[`.github/workflows/ci.yml`](.github/workflows/ci.yml)** — the **Publish image to GHCR** job sets **`packages: write`** on `GITHUB_TOKEN` and pushes **`ghcr.io/<lowercase-owner>/access-manager`** on every push to **`main`**.

### Go module path

[`go/go.mod`](go/go.mod) currently declares `module github.com/dtorabi/access-manager`. That can stay (import path independent of clone URL) or move to `github.com/DanyalTorabi/access-manager` in a dedicated change if you want `go get` to match the GitHub org.
