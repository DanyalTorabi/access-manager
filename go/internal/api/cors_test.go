package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// nopHandler responds 200 OK and records that it was called.
func nopHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if called != nil {
			*called = true
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORSMiddleware_noOriginHeader(t *testing.T) {
	t.Parallel()
	called := false
	h := CORSMiddleware([]string{"*"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler must be called when Origin header is absent")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header, got %q", got)
	}
}

func TestCORSMiddleware_emptyOriginsList(t *testing.T) {
	t.Parallel()
	called := false
	h := CORSMiddleware([]string{})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://example.com")
	h.ServeHTTP(rec, req)

	// With an empty list CORSMiddleware is not wired at all in Router(),
	// but if someone calls it directly with an empty slice, next is still called.
	if !called {
		t.Fatal("next handler must be called even when origins list is empty")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header, got %q", got)
	}
}

func TestCORSMiddleware_wildcardReflectsOrigin(t *testing.T) {
	t.Parallel()
	const origin = "http://localhost:3000"
	called := false
	h := CORSMiddleware([]string{"*"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", origin)
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler must be called for non-preflight")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("Access-Control-Allow-Origin: got %q, want %q", got, origin)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials: got %q, want %q", got, "true")
	}
	if got := rec.Header().Get("Vary"); got == "" {
		t.Fatal("Vary header must be set")
	}
	// The literal "*" must never appear — browsers reject it with credentials.
	if rec.Header().Get("Access-Control-Allow-Origin") == "*" {
		t.Fatal("Access-Control-Allow-Origin must not be literal \"*\"")
	}
}

func TestCORSMiddleware_explicitListAllowed(t *testing.T) {
	t.Parallel()
	const origin = "https://app.example.com"
	called := false
	h := CORSMiddleware([]string{"https://app.example.com", "https://other.example.com"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	req.Header.Set("Origin", origin)
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler must be called for allowed origin")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("Access-Control-Allow-Origin: got %q, want %q", got, origin)
	}
}

func TestCORSMiddleware_explicitListDisallowed(t *testing.T) {
	t.Parallel()
	called := false
	h := CORSMiddleware([]string{"https://allowed.example.com"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler must still be called for disallowed origin")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header for disallowed origin, got %q", got)
	}
}

func TestCORSMiddleware_preflightWildcard(t *testing.T) {
	t.Parallel()
	const origin = "http://localhost:5173"
	called := false
	h := CORSMiddleware([]string{"*"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/domains", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	h.ServeHTTP(rec, req)

	if called {
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
}

func TestCORSMiddleware_preflightDisallowedOrigin(t *testing.T) {
	t.Parallel()
	called := false
	h := CORSMiddleware([]string{"https://allowed.example.com"})(nopHandler(&called))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/domains", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	h.ServeHTTP(rec, req)

	// Disallowed: must not short-circuit; next is called.
	if !called {
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
	res, err := http.DefaultClient.Do(req)
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
