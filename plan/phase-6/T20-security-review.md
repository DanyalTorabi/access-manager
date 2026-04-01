# T20 — Security review

## Ticket

**T20** — Security review

## Phase

**Phase 6** — P3 scale, prod, product options

## Goal

Systematic review: **dependencies** (govulncheck), **SAST**, **threat model** for authz API, **secrets** handling, and **audit** logging for sensitive mutations.

## Deliverables

- `govulncheck` in CI (or Makefile).
- Short **threat model** doc: actors, assets, trust boundaries.
- Audit log events: grant/revoke, domain changes (structured log, no PII overload).

## Steps

1. Add `govulncheck ./...` to T13 or Makefile.
2. Run golangci-lint security-related linters if enabled.
3. Review SQL: ensure parameterized queries only (already pattern).
4. Document authz bypass risks (direct DB access, JWT validation gaps).

## Files / paths

- **Create:** `docs/security-review.md` (optional)
- **Edit:** `.github/workflows/ci.yml`, logging in API/store

## Acceptance criteria

- `govulncheck` passes or documented exceptions with ticket.
- Audit events emitted for permission grants/revokes.

## Out of scope

- Full pen test engagement; SOC2 paperwork.

## Dependencies

- **T7** auth in place for realistic review.

## Curriculum link

**Hardening** — production readiness.

**Suggested P3 order:** after **T23** or in parallel with observability.
