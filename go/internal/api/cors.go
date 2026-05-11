package api

import (
	"net/http"
	"strings"
)

const (
	corsAllowMethods = "GET, POST, PATCH, DELETE, OPTIONS"
	corsAllowHeaders = "Content-Type, Authorization"
	corsMaxAge       = "300"
)

// CORSMiddleware returns a chi-compatible middleware that adds CORS response headers
// based on the configured allowed origins.
//
// Behaviour:
//   - Requests without an Origin header pass through unchanged (non-browser traffic).
//   - When origins contains "*", any origin is allowed and the actual request Origin is
//     reflected in Access-Control-Allow-Origin (never the literal "*") so that
//     Access-Control-Allow-Credentials: true is accepted by browsers.
//   - When origins is an explicit list, only exact-match origins are allowed.
//   - OPTIONS preflight requests from allowed origins receive a 204 response with full
//     preflight headers and do not reach the inner handler.
//   - Disallowed origins pass through to the handler without any CORS headers.
//   - An empty origins slice disables all CORS header injection.
func CORSMiddleware(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			if !isOriginAllowed(origin, origins) {
				next.ServeHTTP(w, r)
				return
			}

			h := w.Header()
			// Reflect the actual Origin instead of echoing "*"; required when
			// Access-Control-Allow-Credentials is true (browsers reject Origin: *).
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Add("Vary", "Origin")

			if r.Method == http.MethodOptions {
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

// isOriginAllowed reports whether the given origin is permitted by the configured list.
func isOriginAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" {
			return true
		}
		if strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}
