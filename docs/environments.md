# Environments — access-manager

This document is the single reference for how configuration, secrets, and deployments are managed per environment tier. An on-call engineer should be able to locate the DSN, auth token, and CORS setting for any tier from this file.

See [go/README.md](../go/README.md) for the full configuration reference, and [go/config.example.yaml](../go/config.example.yaml) for a commented template.

---

## Tier matrix

| Tier | Owner | Host port binding[^1] | Database | DSN source | Auth token source | CORS | Image tag |
|------|-------|-----------|----------|-----------|-------------------|------|-----------|
| **Local dev** | Engineer | `127.0.0.1:8080` | SQLite (file or tmpfs) | `.env` / `config.local.yaml` | unset (loopback only) | `*` (default) | local build |
| **CI / PR** | GitHub Actions | `127.0.0.1:8080` | SQLite tmpfs (smoke); Postgres + MySQL for integration job | `DATABASE_URL` via compose env | unset or `ACTIONS_…` secret | `*` (loopback-only CI runner) | `sha-<sha>` (built per run, not pushed except on `main`) |
| **Staging** | Team / CI `main` merge | none (reverse proxy) | Postgres | `DATABASE_URL` → GitHub Environment secret `STAGING_DATABASE_URL` | `API_BEARER_TOKEN` → GitHub Environment secret `STAGING_API_BEARER_TOKEN` | explicit origin list or `none` if proxy handles CORS | `sha-<sha>` (latest green `main` SHA) |
| **Production** | Ops / on-call | none (reverse proxy) | Postgres (or MySQL) | `DATABASE_URL` → K8s Secret `access-manager-db` key `url` | `API_BEARER_TOKEN` → K8s Secret `access-manager-auth` key `token` | `none` (reverse proxy enforces CORS policy) | pinned `sha-<sha>` (promoted manually after staging sign-off) |

[^1]: **Host port binding** is the `ports:` mapping in the compose file (what the host OS sees). Inside every container, `HTTP_ADDR` is always `0.0.0.0:8080` so the port mapping works — see the Config key table for per-tier values. For staging and production, no host port is exposed; the reverse proxy reaches the container via Docker/K8s internal network.

---

## Config key → secrets mapping

| Config key / env var | Local dev | CI / PR | Staging | Production |
|---------------------|-----------|---------|---------|------------|
| `DATABASE_URL` | `.env` or `config.local.yaml` | compose `environment:` block | GitHub Environment secret `STAGING_DATABASE_URL` | K8s Secret `access-manager-db`, key `url` |
| `DATABASE_DRIVER` | `.env` (default `sqlite`) | compose `environment:` | GitHub Environment var `STAGING_DATABASE_DRIVER` | K8s ConfigMap (non-sensitive) |
| `MIGRATIONS_DIR` | `.env` or default | compose `environment:` | GitHub Environment var | K8s ConfigMap |
| `API_BEARER_TOKEN` | unset | unset or test secret | GitHub Environment secret `STAGING_API_BEARER_TOKEN` | K8s Secret `access-manager-auth`, key `token` |
| `CORS_ALLOWED_ORIGINS` | `*` (default) | `*` (loopback runner, safe) | explicit list or `none` | `none` (proxy handles CORS) |
| `HTTP_ADDR` | `127.0.0.1:8080` | `0.0.0.0:8080` (container) | `0.0.0.0:8080` (container) | `0.0.0.0:8080` (container) |
| `SHUTDOWN_TIMEOUT_SECONDS` | `30` | `30` | `60` | `60` |

> **Note on CORS `none`:** Setting `CORS_ALLOWED_ORIGINS=none` (or `cors_allowed_origins: "none"` in YAML) disables all in-process CORS response headers. Use this in staging/production when a reverse proxy (nginx, Caddy, API gateway) manages CORS centrally. See [go/README.md § Environment variables](../go/README.md#environment-variables).

---

## Image tag strategy

| Event | Tags pushed to GHCR | Description |
|-------|---------------------|-------------|
| Push to `main` | `latest`, `sha-<full-sha>` | Auto-published by CI `publish` job. `latest` is always the most recent `main` build. |
| PR branches | not pushed | Image is built and smoke-tested in CI but not published. |
| Staging deploy | `sha-<sha>` | Pin the specific SHA from the `main` build that passed all tests. |
| Production deploy | `sha-<sha>` | Promote the same SHA that ran in staging; do not re-tag or rebuild. |

**Repository:** `ghcr.io/danyaltorabi/access-manager`

---

## Rollback procedure

### Application rollback (image)

1. Identify the last known-good image tag (e.g. `sha-abc1234`). Check the GHCR package history or the CI run that built it.
2. Update the deployment to point at the previous tag:
   - **Docker Compose (staging):** Set `image: ghcr.io/danyaltorabi/access-manager:sha-abc1234` in the override file and run `docker compose up -d app`.
   - **Kubernetes (production):** `kubectl set image deployment/access-manager server=ghcr.io/danyaltorabi/access-manager:sha-abc1234 -n <namespace>`.
3. Verify `/health` returns `{"status":"ok"}`.

### Database migration rollback

Migrations are **forward-only** (no down scripts). The backward policy is:

- **Additive changes** (new columns, new tables, new indexes): safe to roll back the app binary to a previous version while leaving the new schema in place — the older code ignores unknown columns.
- **Breaking changes** (column removal, type changes, constraint tightening): require a new forward migration to restore the previous schema shape, or a full restore from backup. No automated rollback is provided.
- **Recommendation:** before any breaking migration, take a point-in-time backup of the database.

---

## Local development

```bash
# Defaults only — SQLite, loopback, no auth token required
cd go && go run ./cmd/server

# With a local config file
CONFIG_PATH=config.local.yaml go run ./cmd/server

# SQLite with explicit env overrides
DATABASE_URL="file:dev.db?_pragma=foreign_keys(1)" HTTP_ADDR=127.0.0.1:8080 go run ./cmd/server

# Docker (SQLite on tmpfs — ephemeral)
make docker-up          # from repo root
curl http://127.0.0.1:8080/health
```

Copy [go/config.example.yaml](../go/config.example.yaml) to a gitignored path (e.g. `go/config.local.yaml`) for persistent local settings; do **not** commit real credentials.

---

## CI / PR environment

The `ci.yml` workflow runs three jobs:

| Job | Env | Notes |
|-----|-----|-------|
| `go` | Unit tests + lint | SQLite in-memory; no external services |
| `docker` | Docker build + compose smoke + e2e | SQLite tmpfs; all ports on loopback `127.0.0.1` |
| `integration` | Postgres 15 + MySQL 8 | DSN via compose `environment:` block |
| `publish` | GHCR push | Only on `main` push; requires `packages: write` |

Secrets used in CI:
- `CODECOV_TOKEN` — Codecov upload (repository secret)
- `GITHUB_TOKEN` — GHCR push (built-in Actions token)

No `API_BEARER_TOKEN` or database credentials are committed; the integration job uses short-lived compose credentials (`access` / `access`) that are safe for ephemeral test containers.

---

## GitHub Environments (staging / prod)

Configure [GitHub Environments](https://docs.github.com/en/actions/deployment/targeting-different-environments/using-environments-for-deployment) in the repository settings:

| Environment | Required reviewers | Secrets to configure |
|-------------|-------------------|----------------------|
| `staging` | None (auto-deploy on `main`) | `STAGING_DATABASE_URL`, `STAGING_API_BEARER_TOKEN` |
| `production` | At least 1 reviewer | `PROD_DATABASE_URL`, `PROD_API_BEARER_TOKEN` |

Each GitHub Environment secret maps 1-to-1 to the corresponding env var the service reads at startup (see the Config key table above).

---

## Kubernetes (production — T21, planned)

> **Planned — T21 ([#32](https://github.com/DanyalTorabi/access-manager/issues/32)) is not yet implemented.** The manifests and namespace below are a design sketch for review, not a description of a running cluster. Do not use them as an on-call reference until T21 ships.

The expected secrets layout for a future K8s deployment:

```yaml
# access-manager-db Secret (example)
apiVersion: v1
kind: Secret
metadata:
  name: access-manager-db
  namespace: access-manager
stringData:
  url: "postgres://user:password@postgres-svc:5432/access_manager?sslmode=require"

# access-manager-auth Secret (example)
apiVersion: v1
kind: Secret
metadata:
  name: access-manager-auth
  namespace: access-manager
stringData:
  token: "<strong-random-token>"
```

Mount these as environment variables in the Deployment:

```yaml
env:
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: access-manager-db
        key: url
  - name: API_BEARER_TOKEN
    valueFrom:
      secretKeyRef:
        name: access-manager-auth
        key: token
  - name: DATABASE_DRIVER
    value: "postgres"
  - name: MIGRATIONS_DIR
    value: "migrations/postgres"
  - name: HTTP_ADDR
    value: "0.0.0.0:8080"
  - name: CORS_ALLOWED_ORIGINS
    value: "none"
  - name: SHUTDOWN_TIMEOUT_SECONDS
    value: "60"
```

Never put K8s Secret values in manifests committed to the repository. Use a secret manager (e.g. External Secrets Operator, Sealed Secrets, or Vault) to inject them at deploy time.

---

## CORS disable sentinel

When the service runs behind a reverse proxy that manages CORS (nginx, Caddy, API gateway), set:

```
CORS_ALLOWED_ORIGINS=none
```

or in YAML:

```yaml
cors_allowed_origins: "none"
```

This resolves `CORSAllowedOrigins` to an empty slice, which the CORS middleware interprets as "skip all CORS headers". The sentinel is case-insensitive (`none`, `NONE`, `None` are all equivalent). See [go/README.md](../go/README.md) and [go/config.example.yaml](../go/config.example.yaml).

---

*Last updated: T22 (#33)*
