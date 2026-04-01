package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
	"github.com/dtorabi/access-manager/internal/testutil"
)

func TestBearerAuth_emptyExpectedDeniesAll(t *testing.T) {
	cases := []struct {
		name string
		exp  string
	}{
		{name: "empty", exp: ""},
		{name: "whitespace_only", exp: "   \t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := BearerAuth(tc.exp)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("next handler must not run")
			}))
			srv := httptest.NewServer(h)
			t.Cleanup(srv.Close)

			req, err := http.NewRequest(http.MethodGet, srv.URL+"/x", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer anything")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = res.Body.Close() }()
			if res.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status %d", res.StatusCode)
			}
		})
	}
}

func TestBearerAuth_missingToken(t *testing.T) {
	h := BearerAuth("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
	if res.Header.Get("WWW-Authenticate") == "" {
		t.Fatal("expected WWW-Authenticate")
	}
}

func TestBearerAuth_wrongToken(t *testing.T) {
	h := BearerAuth("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/x", nil)
	req.Header.Set("Authorization", "Bearer other")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestBearerAuth_ok(t *testing.T) {
	h := BearerAuth("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/x", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusTeapot {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestBearerAuth_caseInsensitiveScheme(t *testing.T) {
	h := BearerAuth("tok")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/x", nil)
	req.Header.Set("Authorization", "bearer tok")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestParseBearerToken(t *testing.T) {
	if g := parseBearerToken("Bearer abc def"); g != "abc def" {
		t.Fatalf("got %q", g)
	}
	if g := parseBearerToken("bearer x"); g != "x" {
		t.Fatalf("got %q", g)
	}
	if parseBearerToken("Basic x") != "" {
		t.Fatal("expected empty")
	}
	if parseBearerToken("") != "" {
		t.Fatal("expected empty")
	}
}

func TestAPI_bearerRequiredOnAPIRoutes(t *testing.T) {
	db, err := sqlstore.Open("file:" + filepath.Join(t.TempDir(), "api.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlstore.MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	st := sqlstore.New(db)
	srv := &Server{Store: st, APIBearerToken: "test-token"}
	ts := httptest.NewServer(srv.Router(nil, nil))
	t.Cleanup(ts.Close)

	res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(`{"title":"X"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/domains", strings.NewReader(`{"title":"Y"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	res2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res2.Body.Close() }()
	if res2.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(res2.Body)
		t.Fatalf("status %d: %s", res2.StatusCode, body)
	}
}
