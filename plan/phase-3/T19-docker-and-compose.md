# T19 — Docker & Compose

## Ticket

**T19** — Docker (see [TICKETS.md](../../TICKETS.md))

## Phase

**Phase 3** — GitHub, Docker, CI

## Goal

Ship a **multi-stage Dockerfile** ending in **scratch or distroless**, and **`docker-compose.yml`** to run the app with a SQL database for local integration and CI (T13).

## Deliverables

- [x] **`Dockerfile`** (repo root): context includes **`go/`**; multi-stage **distroless** `nonroot` image.
- [x] **`docker-compose.yml`**: **`app`** only + SQLite on **tmpfs** (v1); optional Postgres deferred to **T1**.
- [x] Document build/run in **README** (+ **go/README**, **CONTRIBUTING**).

## Steps

1. [x] Static binary (`CGO_ENABLED=0`, pure Go sqlite).
2. [x] Copy binary + migrations; `MIGRATIONS_DIR` + `DATABASE_URL` via env / image defaults.
3. [x] Compose: **`127.0.0.1:8080:8080`** on host; `HTTP_ADDR=0.0.0.0:8080` in container.
4. [x] **T26** env for ports and DSN in compose.

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
