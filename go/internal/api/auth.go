package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// BearerAuth returns chi middleware that requires `Authorization: Bearer <token>`
// matching expected (after trim). Uses constant-time comparison on the secret.
// Health and other routes outside this middleware are unaffected.
func BearerAuth(expected string) func(http.Handler) http.Handler {
	exp := strings.TrimSpace(expected)
	expB := []byte(exp)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := parseBearerToken(r.Header.Get("Authorization"))
			gotB := []byte(got)
			ok := len(gotB) == len(expB) && subtle.ConstantTimeCompare(gotB, expB) == 1
			if !ok {
				w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseBearerToken(h string) string {
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
