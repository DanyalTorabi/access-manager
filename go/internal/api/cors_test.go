package api

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// nopHandler responds 200 OK and records that it was called via an atomic bool,
// safe for use in both synchronous (httptest.ResponseRecorder) and server-side goroutines.
func nopHandler(called *atomic.Bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if called != nil {
			called.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORSMiddleware_noOriginHeader(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	h := CORSMiddleware([]string{"*"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)

	if !called.Load() {
		t.Fatal("next handler must be called when Origin header is absent")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header, got %q", got)
	}
	// Vary:Origin is set unconditionally by the middleware so proxies know the response
	// can vary by Origin even when the request had none (Fetch spec recommendation).
	if got := rec.Header().Get("Vary"); got == "" {
		t.Fatal("Vary:Origin must be set unconditionally by the middleware")
	}
}

func TestCORSMiddleware_emptyOriginsList(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	h := CORSMiddleware([]string{})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://example.com")
	h.ServeHTTP(rec, req)

	if !called.Load() {
		t.Fatal("next handler must be called even when origins list is empty")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header, got %q", got)
	}
}

func TestCORSMiddleware_wildcardReflectsOriginNoCredentials(t *testing.T) {
	t.Parallel()
	const origin = "http://localhost:3000"
	var called atomic.Bool
	h := CORSMiddleware([]string{"*"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", origin)
	h.ServeHTTP(rec, req)

	if !called.Load() {
		t.Fatal("next handler must be called for non-preflight")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("Access-Control-Allow-Origin: got %q, want %q", got, origin)
	}
	// Wildcard-admitted requests must NOT set Allow-Credentials — this avoids the
	// OWASP wildcard+credentials anti-pattern (arbitrary origin + credential forwarding).
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials must be absent for wildcard, got %q", got)
	}
	if got := rec.Header().Get("Vary"); got == "" {
		t.Fatal("Vary header must be set")
	}
	if rec.Header().Get("Access-Control-Allow-Origin") == "*" {
		t.Fatal("Access-Control-Allow-Origin must not be literal \"*\"")
	}
}

func TestCORSMiddleware_explicitListAllowedSetsCredentials(t *testing.T) {
	t.Parallel()
	const origin = "https://app.example.com"
	var called atomic.Bool
	h := CORSMiddleware([]string{"https://app.example.com", "https://other.example.com"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	req.Header.Set("Origin", origin)
	h.ServeHTTP(rec, req)

	if !called.Load() {
		t.Fatal("next handler must be called for allowed origin")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("Access-Control-Allow-Origin: got %q, want %q", got, origin)
	}
	// Explicit origins get Allow-Credentials: true (enabling credentialed requests from known apps).
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials: got %q, want %q", got, "true")
	}
}

func TestCORSMiddleware_caseInsensitiveOriginRejected(t *testing.T) {
	t.Parallel()
	// Origins are case-sensitive per RFC 6454 §6.1. Mixed-case origin must not match.
	var called atomic.Bool
	h := CORSMiddleware([]string{"https://app.example.com"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	req.Header.Set("Origin", "HTTPS://APP.EXAMPLE.COM")
	h.ServeHTTP(rec, req)

	if !called.Load() {
		t.Fatal("next handler must still be called for disallowed origin")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header for case-variant origin, got %q", got)
	}
}

func TestCORSMiddleware_explicitListDisallowed(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	h := CORSMiddleware([]string{"https://allowed.example.com"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	h.ServeHTTP(rec, req)

	if !called.Load() {
		t.Fatal("next handler must still be called for disallowed origin")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header for disallowed origin, got %q", got)
	}
}

func TestCORSMiddleware_varySetUnconditionally(t *testing.T) {
	t.Parallel()
	// Vary:Origin must be set even when origin is disallowed, to prevent cache poisoning.
	var called atomic.Bool
	h := CORSMiddleware([]string{"https://allowed.example.com"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Vary"); got == "" {
		t.Fatal("Vary header must be set even when origin is not allowed")
	}
}

func TestCORSMiddleware_preflightWildcard(t *testing.T) {
	t.Parallel()
	const origin = "http://localhost:5173"
	var called atomic.Bool
	h := CORSMiddleware([]string{"*"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/domains", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	h.ServeHTTP(rec, req)

	if called.Load() {
		t.Fatal("inner handler must NOT be called for OPTIONS preflight")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("Access-Control-Allow-Origin: got %q, want %q", got, origin)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("Access-Control-Allow-Methods must be set on preflight")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("Access-Control-Allow-Headers must be set on preflight")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got == "" {
		t.Fatal("Access-Control-Max-Age must be set on preflight")
	}
	// Wildcard preflight: no credentials header.
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials must be absent for wildcard preflight, got %q", got)
	}
}

func TestCORSMiddleware_optionsWithoutRequestMethodNotPreflight(t *testing.T) {
	t.Parallel()
	// OPTIONS + Origin but no Access-Control-Request-Method is NOT a preflight (Fetch §3.2.3).
	// The inner handler must be called and the response must not be a 204 short-circuit.
	var called atomic.Bool
	h := CORSMiddleware([]string{"*"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/domains", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	// Deliberately NOT setting Access-Control-Request-Method.
	h.ServeHTTP(rec, req)

	if !called.Load() {
		t.Fatal("inner handler must be called for bare OPTIONS (not a preflight)")
	}
	if rec.Code == http.StatusNoContent {
		t.Fatal("bare OPTIONS must not be short-circuited as a preflight")
	}
}

func TestCORSMiddleware_preflightDisallowedOrigin(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	h := CORSMiddleware([]string{"https://allowed.example.com"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/domains", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	h.ServeHTTP(rec, req)

	if !called.Load() {
		t.Fatal("next handler must be called when preflight origin is not allowed")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header for disallowed preflight origin, got %q", got)
	}
}

func TestCORSMiddleware_serverIntegration(t *testing.T) {
	t.Parallel()
	// Verify CORS headers arrive through the full Server.Router() stack.
	st, cleanup := newTestStore(t)
	defer cleanup()
	srv := &Server{Store: st, CORSAllowedOrigins: []string{"*"}}
	ts := httptest.NewServer(srv.Router(nil, nil))
	t.Cleanup(ts.Close)

	const origin = "http://localhost:3000"
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", origin)
	// Use ts.Client() — purpose-built for this test server, avoids shared DefaultClient state.
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("health status: %d", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("Access-Control-Allow-Origin: got %q, want %q", got, origin)
	}
}

