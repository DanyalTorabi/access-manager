# T22 — Environments (dev / PR / staging / prod)

## Ticket

**T22** — Environments: dev / PR / staging / prod (GitHub [#33](https://github.com/DanyalTorabi/access-manager/issues/33))

## Phase

**Phase 6** — P3 scale, prod, product options

## Goal

Define **promotion** and **configuration** per environment: secrets injection, DB endpoints, feature flags, and how **PR previews** differ from **staging** and **prod**.

## Deliverables

- Matrix doc: env → DSN source → auth mode → image tag strategy.
- Optional: separate compose files `compose.prod.yml` overrides.

## Steps

1. List envs and owners (who deploys what).
2. Map T26 config keys to K8s Secrets / GitHub Environments.
3. Document rollback: previous image tag + migration backward policy (if any).
4. Add a mechanism to explicitly disable CORS via environment variable (e.g. sentinel value `"none"` → empty
   origin slice) so operators fronting the service with a CORS-managing reverse proxy can cleanly opt out
   without relying on whitespace/comma hacks. Document the sentinel in README and `config.example.yaml`.
   See T67 (#114) for the current CORS implementation; disabling CORS via env is deliberately deferred here.

## Files / paths

- **Create:** `docs/environments.md`

## Acceptance criteria

- On-call engineer can find DSN and auth config source for each tier in one doc.

## Out of scope

- Multi-region active-active.

## Dependencies

- **T13** CI/CD; **T19** Docker; **T21** K8s optional.

## Curriculum link

**Theme 7** — operational maturity with K8s.

**Suggested P3 order:** after **T20** baseline or with **T21**.
