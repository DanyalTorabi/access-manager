# T67 — CORS Middleware with Origin Whitelist and Credentials Support

**GitHub issue:** [#114](https://github.com/DanyalTorabi/access-manager/issues/114)
**Branch:** `danyal/feature/t67-cors-whitelist`

## Summary

Add a CORS middleware to the access-manager HTTP server so browser-based clients
can make authenticated cross-origin requests.

Behaviour:
- **Config-driven:** `CORS_ALLOWED_ORIGINS` env var (or `cors_allowed_origins` YAML key) accepts
  a comma-separated list of origins. Default: `*` (allow any origin).
- **Wildcard safety:** when `*` is configured, the middleware reflects the actual `Origin`
  request header instead of echoing `*`. This is required because browsers reject
  `Access-Control-Allow-Origin: *` when `Access-Control-Allow-Credentials: true`.
- **Credentials support:** `Access-Control-Allow-Credentials: true` and `Vary: Origin` are set
  on every allowed-origin response.
- **OPTIONS preflight:** short-circuited with `204 No Content` and all preflight headers
  (`Access-Control-Allow-Methods`, `Access-Control-Allow-Headers`, `Access-Control-Max-Age`).
  The middleware sits **before** `BearerAuth` so preflights are never rejected by auth.
- **Startup warning:** logged when `*` is configured on a non-loopback bind address,
  mirroring the existing `API_BEARER_TOKEN` warning.

## Deliverables

| File | Change |
|------|--------|
| `go/internal/config/config.go` | Add `CORSAllowedOrigins []string` field; parse comma-separated YAML/env; default `["*"]` |
| `go/internal/config/warn.go` | Add `CORSStartupWarning(c Config) string` |
| `go/internal/config/warn_test.go` | Add `TestCORSStartupWarning` table-driven test |
| `go/internal/api/cors.go` | New: `CORSMiddleware(origins []string)` middleware |
| `go/internal/api/cors_test.go` | New: 8 unit tests + server-integration test |
| `go/internal/api/server.go` | Add `CORSAllowedOrigins []string` field; apply `CORSMiddleware` first in `Router()` |
| `go/cmd/server/main.go` | Pass `cfg.CORSAllowedOrigins` to `Server`; call `maybeWarnCORS` at startup |
| `go/config.example.yaml` | Document `cors_allowed_origins` option |
| `go/README.md` | Document `CORS_ALLOWED_ORIGINS` in configuration table |
| `CHANGELOG.md` | Unreleased entry |

## Configuration reference

| Env var | YAML key | Default | Notes |
|---------|----------|---------|-------|
| `CORS_ALLOWED_ORIGINS` | `cors_allowed_origins` | `*` | Comma-separated origin list. `*` = allow any (reflects actual Origin header). Empty string falls back to default. |

## Technical decisions

- **Default `*`:** wildcard is the most convenient default for local dev;
  the startup warning informs operators when this is potentially unsafe.
- **Never echo literal `*`:** browsers reject `Access-Control-Allow-Origin: *` with
  `Access-Control-Allow-Credentials: true`; always reflect the request Origin.
- **Outermost middleware:** CORS precedes `BearerAuth` so `OPTIONS` preflights from
  browsers succeed without a Bearer token.
- **Explicit empty slice disables CORS** (in tests / programmatic use); empty env string
  retains the default `*`.

## Out of scope

- `Access-Control-Expose-Headers` (no custom headers exposed yet)
- Per-route CORS configuration
- Support for `Access-Control-Allow-Credentials: false` opt-out

## Verification

```bash
# All origins allowed (default):
curl -v -H "Origin: http://localhost:3000" http://127.0.0.1:8080/health
# → Access-Control-Allow-Origin: http://localhost:3000
# → Access-Control-Allow-Credentials: true

# Preflight:
curl -v -X OPTIONS \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: POST" \
  http://127.0.0.1:8080/api/v1/domains
# → HTTP/1.1 204 No Content
# → Access-Control-Allow-Methods: GET, POST, PATCH, DELETE, OPTIONS

# Explicit origin list (env override):
CORS_ALLOWED_ORIGINS="https://app.example.com" ./bin/server
# → request from http://evil.com gets no CORS headers
```
