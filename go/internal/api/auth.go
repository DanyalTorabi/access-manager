package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// BearerAuth returns chi middleware that requires `Authorization: Bearer <token>`
// matching expected (after trim). Tokens are compared via SHA-256 digests and
// subtle.ConstantTimeCompare so the equality check does not short-circuit on length.
// If expected is empty after trim, all requests are rejected (safe if middleware is
// wired without a non-empty guard). Health and routes outside this middleware are unaffected.
func BearerAuth(expected string) func(http.Handler) http.Handler {
	exp := strings.TrimSpace(expected)
	if exp == "" {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				bearerAuthUnauthorized(w)
			})
		}
	}
	expHash := sha256.Sum256([]byte(exp))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := parseBearerToken(r.Header.Get("Authorization"))
			gotHash := sha256.Sum256([]byte(got))
			if subtle.ConstantTimeCompare(gotHash[:], expHash[:]) != 1 {
				bearerAuthUnauthorized(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerAuthUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}

func parseBearerToken(h string) string {
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
