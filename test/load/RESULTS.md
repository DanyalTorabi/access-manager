# Load Test Results

Run with:

```sh
# From repo root
k6 run test/load/authz.js

# Saturation sweep
k6 run --vus 200 --duration 60s test/load/authz.js

# Soak (30 min)
k6 run --vus 50 --duration 30m test/load/authz.js
```

Set `BASE_URL` and `API_BEARER_TOKEN` env vars as needed. See comments in `test/load/authz.js` for the full option reference.

---

## Baseline

| Date | OS / CPU | Server version | DB | VUs | Duration | p50 (ms) | p95 (ms) | p99 (ms) | RPS | Notes |
|------|----------|----------------|----|-----|----------|----------|----------|----------|-----|-------|
| TODO | — | — | SQLite | 50 | 160 s | — | — | — | — | Baseline pending post-merge run |

---

## Saturation sweep

Record the VU level at which p99 first exceeds 50 ms or the error rate first exceeds 1%.

| Date | OS / CPU | Server version | DB | VUs at inflection | p99 at inflection (ms) | Error rate | Notes |
|------|----------|----------------|----|-------------------|------------------------|------------|-------|
| TODO | — | — | SQLite | — | — | — | |

---

## Soak (30 min @ 50 VUs)

Watch for memory growth (`go_memstats_alloc_bytes`), goroutine leaks (`go_goroutines`), and SQLite WAL growth.

| Date | OS / CPU | Server version | DB | VUs | Duration | Max RSS (MB) | Max goroutines | Final p99 (ms) | Notes |
|------|----------|----------------|----|-----|----------|--------------|----------------|----------------|-------|
| TODO | — | — | SQLite | 50 | 30 min | — | — | — | |
