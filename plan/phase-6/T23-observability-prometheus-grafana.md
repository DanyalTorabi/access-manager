# T23 — Observability (Prometheus & Grafana)

## Ticket

**T23** — Observability (GitHub [#34](https://github.com/DanyalTorabi/access-manager/issues/34))

## Phase

**Phase 6** — P3 scale, prod, product options

## Goal

Expose **Prometheus** metrics from the Go process and provide **Grafana** dashboards (via compose) for request volume, authz checks, errors, and latency buckets.

## Deliverables

- `/metrics` endpoint (prometheus client).
- Counters/histograms: `http_requests_total`, `authz_checks_total`, `http_request_duration_seconds`, etc.
- `docker-compose` snippet or overlay for Prometheus + Grafana; example dashboard JSON under `deploy/grafana/` or `observability/`.
- Tests asserting metrics register and increment on sample requests.

## Steps

1. Add `prometheus/client_golang` dependency; register default process collectors if desired.
2. Instrument chi with middleware wrapping or wrap `http.Handler`.
3. Add compose services: prometheus scrapes app, grafana datasources provisioned.
4. Document in README; add CI step that runs metric-related tests only (fast).

## Files / paths

- **Create:** `internal/api/metrics.go`, `observability/*` compose fragments, dashboard JSON
- **Edit:** [internal/api/server.go](../../go/internal/api/server.go), [go.mod](../../go/go.mod)

## Acceptance criteria

- Scraping `/metrics` shows application-specific series after traffic.
- Grafana dashboard renders at least one panel from those metrics.

## Out of scope

- Distributed tracing (optional future); log aggregation SaaS.

## Dependencies

- **T19** compose patterns; **T13** for CI.

## Curriculum link

**Theme 5** — Prometheus & Grafana.

**Suggested P3 order:** start with **T23** for runtime signal.
