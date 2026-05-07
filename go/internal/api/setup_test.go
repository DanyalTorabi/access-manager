package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
	"github.com/dtorabi/access-manager/internal/testutil"
	"github.com/prometheus/client_golang/prometheus"
)

// newTestStore returns a migrated SQLite store and a cleanup function.
func newTestStore(t *testing.T) (store.Store, func()) {
	t.Helper()
	db, err := sqlstore.Open("file:" + filepath.Join(t.TempDir(), "api.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlstore.MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	return sqlstore.New(db), func() { _ = db.Close() }
}

// newTestAPI returns an HTTP test server backed by a real SQLite store and migrations.
func newTestAPI(t *testing.T) (*httptest.Server, store.Store) {
	t.Helper()
	st, cleanup := newTestStore(t)
	srv := &Server{Store: st}
	ts := httptest.NewServer(srv.Router(nil, nil))
	t.Cleanup(func() {
		ts.Close()
		cleanup()
	})
	return ts, st
}

// mustPostJSON201 is a convenience wrapper for mustPostJSON with http.StatusCreated.
func mustPostJSON201(t *testing.T, url, body string) []byte {
	t.Helper()
	return mustPostJSON(t, url, body, http.StatusCreated)
}

// auditLogEntries returns each newline-delimited JSON object from buf that has audit=true.
func auditLogEntries(t *testing.T, buf string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, rawLine := range strings.Split(buf, "\n") {
		rawLine = strings.TrimSpace(rawLine)
		if rawLine == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(rawLine), &m); err != nil {
			t.Fatalf("log line JSON: %v — line %q — full buf: %q", err, rawLine, buf)
		}
		if v, ok := m["audit"]; ok && v == true {
			out = append(out, m)
		}
	}
	return out
}

func auditLogEntriesWithAction(t *testing.T, buf, action string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, e := range auditLogEntries(t, buf) {
		if e["action"] == action {
			out = append(out, e)
		}
	}
	return out
}

// NOTE: Tests that call logger.Init mutate the package-level logger pointer.
// This is safe only because no test in this file uses t.Parallel().
// Do NOT add t.Parallel() without first switching to a logger-injectable
// Server field or an atomic pointer. Tracked on #47 (T36 follow-ups).

// --- writeStoreErr unit tests ---

func dummyRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/test", nil)
}

// newBrokenTestAPIWithRegistry builds a Server backed by a closed DB so any
// store call returns an error. If reg is non-nil it is wired through Router
// so callers can assert metrics; otherwise instrumentation is disabled.
func newBrokenTestAPIWithRegistry(t *testing.T, reg *prometheus.Registry) *httptest.Server {
	t.Helper()
	db, err := sqlstore.Open("file:" + filepath.Join(t.TempDir(), "broken.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlstore.MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	st := sqlstore.New(db)
	_ = db.Close()
	srv := &Server{Store: st}
	var router http.Handler
	if reg != nil {
		router = srv.Router(reg, reg)
	} else {
		router = srv.Router(nil, nil)
	}
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	return ts
}

func newBrokenTestAPI(t *testing.T) *httptest.Server {
	t.Helper()
	return newBrokenTestAPIWithRegistry(t, nil)
}

// mustCreateDomain is a test helper that creates a domain and returns its ID.
func mustCreateDomain(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	b := mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"test-domain"}`)
	var out struct{ ID string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

func mustCreateResource(t *testing.T, ts *httptest.Server, domainID, title string) string {
	t.Helper()
	b := mustPostJSON201(t, ts.URL+"/api/v1/domains/"+domainID+"/resources", fmt.Sprintf(`{"title":%q}`, title))
	var out struct{ ID string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

// --- duplicate-create 409 tests ---

// listResponse is a generic envelope for paginated list responses in tests.
type listResponse[T any] struct {
	Data []T `json:"data"`
	Meta struct {
		Total  int64  `json:"total"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
		Sort   string `json:"sort"`
		Order  string `json:"order"`
	} `json:"meta"`
}
