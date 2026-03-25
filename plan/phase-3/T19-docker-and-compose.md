# T19 — Docker & Compose

## Ticket

**T19** — Docker (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 3** — GitHub, Docker, CI

## Goal

Ship a **multi-stage Dockerfile** ending in **scratch or distroless**, and **`docker-compose.yml`** to run the app with a SQL database for local integration and CI (T13).

## Deliverables

- **`Dockerfile`** (repo root or under **`go/`**): build context should include the **Go module** (`go/`); multi-stage build + minimal runtime image; non-root user if distroless allows.
- **`docker-compose.yml`**: service `app` + optional `postgres` (or sqlite volume) per T1 readiness—**v1** can be app-only + volume-mounted SQLite for simplicity.
- Document build/run in README.

## Steps

1. Build static or mostly static binary (`CGO_ENABLED=0` if using pure Go sqlite).
2. Copy binary + migrations into final image; set `MIGRATIONS_DIR` inside image.
3. Compose: expose HTTP only to localhost or internal network by default (align workspace security rules).
4. Use **T26** config/env for ports and DSN inside compose.

## Files / paths

- **Create:** `Dockerfile`, `docker-compose.yml`, optional `.dockerignore`
- **Edit:** [README.md](../../README.md)

## Acceptance criteria

- `docker compose build` and `docker compose up` start a healthy server (healthcheck optional).
- No secrets baked into image layers.

## Out of scope

- Kubernetes manifests (T21); full prod hardening (T20).

## Dependencies

- **T26** for clean config injection.

## Curriculum link

**Themes 2–3–4** — portable runtime and DB for integration tests.

**Order in phase 3:** Prefer **T19 before T13** so CI can run compose-based integration.
