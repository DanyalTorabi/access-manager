# Security Review — access-manager

Short threat model and security posture for the access-manager service.

## Actors

| Actor | Trust level | Access |
|-------|-------------|--------|
| API consumer (Bearer token holder) | Medium | Full CRUD + authz queries on `/api/v1/*` |
| Prometheus scraper | Low | `/metrics` (unauthenticated) |
| Anonymous client | None | `/health` only |
| DB admin (filesystem) | High | Direct SQLite file access |

## Assets

- **Permission grants** — who can access what resources and at what bit level.
- **Access masks** — uint64 bitmasks controlling authorization decisions.
- **Domain configuration** — users, groups, resources, access types, group hierarchy.
- **Audit trail** — structured JSON log lines for all mutations.

## Trust boundaries

```
                    ┌──────────────────────────────────────┐
   Internet/LAN     │          HTTP edge                   │
  ──────────────────┤  /health, /metrics  (no auth)        │
                    │  /api/v1/*          (Bearer token)    │
                    └──────────┬───────────────────────────┘
                               │
                    ┌──────────▼───────────────────────────┐
                    │       Application (Go process)       │
                    │  • parameterized SQL only             │
                    │  • constant-time token comparison     │
                    │  • audit logging on mutations         │
                    └──────────┬───────────────────────────┘
                               │
                    ┌──────────▼───────────────────────────┐
                    │       SQLite file (local FS)         │
                    │  • tmpfs in Docker (ephemeral)        │
                    │  • foreign keys enforced              │
                    └──────────────────────────────────────┘
```

## Mitigations in place

| Risk | Mitigation |
|------|------------|
| SQL injection | All queries use `?` parameterized placeholders; no string concatenation. |
| Timing attacks on auth | Bearer token compared via SHA-256 digest + `subtle.ConstantTimeCompare`. |
| Accidental public exposure | **Process default** (e.g. local `go run`, env unset): listen on loopback `127.0.0.1:8080`. **Container image / compose** often set `HTTP_ADDR=0.0.0.0:8080` inside the container so the port can be published; restrict **host** binding (e.g. `127.0.0.1:8080:8080`) and network policy. Startup warns if the process binds a non-loopback address without a Bearer token. |
| Vulnerable dependencies | `govulncheck` in CI and `make vuln`; gosec linter enabled. |
| Unaudited privilege changes | Structured audit log (`audit=true`) emitted for all mutation endpoints (creates, updates, deletes, membership/grant changes). |
| Container privilege | Distroless non-root image; SQLite on tmpfs. |

## Known gaps / future work

| Gap | Ticket / notes |
|-----|---------------|
| JWT / JWKS validation | Static Bearer token only; no per-user identity in audit logs. |
| Per-user RBAC on admin API | Any valid token holder has full access; no role differentiation. |
| `/metrics` unauthenticated | Exposes Go runtime info; mitigated by loopback binding and documented in code. |
| Rate limiting | No request rate limiting; DoS possible on exposed instances. |
| Audit log integrity | Logs go to stderr (or container stdout); no tamper-proof storage. |
| TLS termination | Not handled by the Go process; expected to sit behind a reverse proxy or load balancer. |
