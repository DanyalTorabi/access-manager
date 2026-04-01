# Contributing

Thanks for contributing to **access-manager**. The Go service lives under [`go/`](go/).

## Before you start

- [**AGENTS.md**](AGENTS.md) — security, layout, library boundaries, post-change checks.
- [**docs/branching.md**](docs/branching.md) — branch names and **PRs to `main`**.

## Local development

From the repository root:

```bash
make test
make lint
```

End-to-end smoke: with the server reachable (default `http://127.0.0.1:8080`), run **`make e2e`** (`go test -race -count=1 -tags=e2e ./e2e/...` from **`go/`**). Optional **`make e2e-bash`** uses **curl** + **jq**. Set **`API_BEARER_TOKEN`** when Bearer auth is enabled. See **[test/e2e/README.md](test/e2e/README.md)**.

See [**go/README.md**](go/README.md) for config, environment variables, and the HTTP API.

### Docker

From the repository root:

```bash
make docker-build
make docker-up          # detached; then: make docker-logs
make docker-down
```

See the root [**README** — Docker](README.md#docker) for image layout and port binding.

## Commits

- Include **tests** for behavioral changes when practical; see [AGENTS.md](AGENTS.md).

### AI assistants (Cursor, etc.)

Provide **proposed** commit message and PR description text only. Do **not** run `git commit`, `git push`, or `gh pr create` unless the contributor explicitly asks. The contributor runs git and GitHub CLI locally.

## Pull requests

### Before you open a PR (self-review)

Do a quick **reviewer pass** on your own diff (humans and AI assistants) before pushing or asking for review:

1. **Automated checks** — From repo root: **`make test`** and **`make lint`**. If you changed **`go/e2e/`** (build tags, package layout), also from **`go/`**: **`go test -tags=e2e -count=1 -run '^$' ./e2e/...`** so the tagged package **compiles** (no need to hit a live server for this step).
2. **Docs and CI match implementation** — If you changed E2E or CI, confirm the **same** command appears where it matters: root **README**, **go/README**, **test/e2e/README**, **`go/Makefile`**, and **`.github/workflows/ci.yml`** (flags such as **`-race`**, **`-count=1`**, **`-tags=e2e`**).
3. **Secrets** — No API tokens, passwords, or real `.env` values in commits; use env vars and **`.env.example`**-style placeholders only.
4. **Common review nits** — Skim for: **`//go:build`** mistakes (e2e package must not be “tests only” under `-tags=e2e`), **assert HTTP status before `json.Decode` / Unmarshal** in tests when setup requests can fail, and **strict checks** for IDs / booleans in smoke scripts (e.g. `jq -e`) where loose parsing hides bugs.

- Deferring a valid review comment to a **later ticket**: reply on the PR with the ticket id (**Txx**) and add a short tracking note to that ticket’s **`plan/...`** spec so it is not forgotten (see [AGENTS.md](AGENTS.md)).
- Open PRs **into `main`**; follow [docs/branching.md](docs/branching.md).
- Use the repo **[pull request template](.github/pull_request_template.md)** (GitHub fills it when you open a PR from the UI): **Summary**, **Ticket**, **Checklist**.
- Reference a ticket from [TICKETS.md](TICKETS.md) when it applies (e.g. `T19`).
- Keep **`make test`** and **`make lint`** green for Go changes.

### Create a PR with `gh`

After `git push -u origin <branch>`:

```bash
gh pr create --base main \
  --title "ci: GitHub Actions, Docker smoke, GHCR on main" \
  --body "## Summary
Adds \`.github/workflows/ci.yml\`: Go test/vet/lint in \`go/\`, Docker compose health smoke, and GHCR publish on pushes to \`main\`.

## Ticket
T13

## Checklist
- [x] \`make test\` / \`make lint\` locally
- [x] Docs updated (README, CONTRIBUTING, AGENTS, TICKETS)
"
```

Match the **same three sections** as the template; swap the title and body for your change.

## GitHub CLI

Prefer **`gh`** for issues, PRs, Actions, and API calls ([GitHub CLI](https://cli.github.com/)); run `gh auth status` if commands fail with auth errors.

---

## Maintainer checklist: GitHub

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
- After the first green CI run on **`main`**, add **required status checks** by **check / job name** (GitHub does not key off the workflow file path). Enable **Go (test, vet, lint)** and **Docker build & compose smoke** from workflow **CI** (see [`.github/workflows/ci.yml`](.github/workflows/ci.yml)).

### Actions and GHCR

- **Settings → Actions → General:** enable Actions as appropriate for the org.
- CI workflow: **[`.github/workflows/ci.yml`](.github/workflows/ci.yml)** — the **Publish image to GHCR** job sets **`packages: write`** on `GITHUB_TOKEN` and pushes **`ghcr.io/<lowercase-owner>/access-manager`** on every push to **`main`**.

### Go module path

[`go/go.mod`](go/go.mod) currently declares `module github.com/dtorabi/access-manager`. That can stay (import path independent of clone URL) or move to `github.com/DanyalTorabi/access-manager` in a dedicated change if you want `go get` to match the GitHub org.
