package api

import (
	"net/http"
)

const (
	corsAllowMethods = "GET, POST, PATCH, DELETE, OPTIONS"
	// corsAllowHeaders lists the non-safelisted request headers the API accepts.
	// Accept is included for convenience; Authorization triggers preflights for non-safelisted access.
	// TODO(T67): make corsAllowHeaders and corsMaxAge configurable via Server field if operator needs change.
	corsAllowHeaders = "Accept, Content-Type, Authorization"
	corsMaxAge       = "300"
)

// CORSMiddleware returns a chi-compatible middleware that adds CORS response headers
// based on the configured allowed origins.
//
// Behaviour:
//   - Requests without an Origin header pass through unchanged (non-browser traffic).
//   - When origins contains "*", any request Origin is reflected in Access-Control-Allow-Origin.
//     Access-Control-Allow-Credentials is NOT set for wildcard-admitted origins; browsers treat
//     it as standard Allow-Origin:* (no credential forwarding). This avoids the OWASP anti-pattern
//     of reflecting arbitrary origins with credentials enabled.
//   - When origins is an explicit list, only exact-match origins (case-sensitive, per RFC 6454 §6.1)
//     are allowed and Access-Control-Allow-Credentials: true is set, enabling credentialed requests.
//   - Vary: Origin is set unconditionally for all responses reaching this middleware, preventing
//     cache-poisoning on intermediate proxies even when the origin is absent or not allowed.
//   - CORS preflight (OPTIONS + Access-Control-Request-Method header per Fetch §3.2.3) from
//     allowed origins receives a 204 response with full preflight headers and bypasses the handler.
//   - Disallowed origins pass through to the handler without CORS headers.
//   - An empty origins slice disables all CORS header injection.
//
// Note: this middleware wraps all routes including /health and /metrics. Those endpoints have no
// auth, so Access-Control-Allow-Credentials: true applies only when explicit origins are listed.
// Wildcard mode does not set Allow-Credentials, limiting cross-origin exposure on public endpoints.
// Per-route CORS opt-out is deferred; see plan/phase-8/T67-cors-whitelist.md.
func CORSMiddleware(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Vary: Origin must be set unconditionally so caching proxies know the response
			// varies by Origin even when no CORS headers are added (Fetch spec recommendation).
			w.Header().Add("Vary", "Origin")

			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			allowed, matchedWildcard := checkOriginAllowed(origin, origins)
			if !allowed {
				next.ServeHTTP(w, r)
				return
			}

			h := w.Header()
			// Always reflect the actual Origin header (never the literal "*").
			// This is required for credentials support and avoids ambiguity in caches.
			h.Set("Access-Control-Allow-Origin", origin)
			// Only set Allow-Credentials for explicit (non-wildcard) origin matches.
			// Wildcard-admitted origins behave like standard Allow-Origin:* (no credentials),
			// avoiding the OWASP wildcard+credentials anti-pattern.
			if !matchedWildcard {
				h.Set("Access-Control-Allow-Credentials", "true")
			}

			// Guard preflight: OPTIONS + Access-Control-Request-Method header (Fetch §3.2.3).
			// A bare OPTIONS request without this header is a regular request — let it through.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", corsAllowMethods)
				h.Set("Access-Control-Allow-Headers", corsAllowHeaders)
				h.Set("Access-Control-Max-Age", corsMaxAge)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// checkOriginAllowed reports whether origin is permitted and whether it matched via a wildcard entry.
// Origin comparisons are case-sensitive (RFC 6454 §6.1); browsers always send lowercase scheme+host.
func checkOriginAllowed(origin string, allowed []string) (ok bool, matchedWildcard bool) {
	for _, a := range allowed {
		if a == "*" {
			return true, true
		}
		if a == origin {
			return true, false
		}
	}
	return false, false
}

