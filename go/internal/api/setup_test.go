package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
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
// The Server uses a discard logger so tests never mutate the package-level logger
// and are safe to run in parallel. For tests that assert on log output use
// newTestAPIWithLog instead.
func newTestAPI(t *testing.T) (*httptest.Server, store.Store) {
	t.Helper()
	st, cleanup := newTestStore(t)
	srv := &Server{
		Store: st,
		Log:   slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	ts := httptest.NewServer(srv.Router(nil, nil))
	t.Cleanup(func() {
		ts.Close()
		cleanup()
	})
	return ts, st
}

// newTestAPIWithLog returns an HTTP test server, the underlying store, and a
// bytes.Buffer that captures all log output from the Server. The handler writes
// JSON log entries at DEBUG level so all log levels are visible to the test.
func newTestAPIWithLog(t *testing.T) (*httptest.Server, store.Store, *bytes.Buffer) {
	t.Helper()
	st, cleanup := newTestStore(t)
	var buf bytes.Buffer
	srv := &Server{
		Store: st,
		Log:   slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	ts := httptest.NewServer(srv.Router(nil, nil))
	t.Cleanup(func() {
		ts.Close()
		cleanup()
	})
	return ts, st, &buf
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

// NOTE: Each test gets its own logger through Server.Log (set in newTestAPI /
// newTestAPIWithLog / newBrokenTestAPIWithRegistry). The global logger is never
// mutated in this package. t.Parallel() is safe to add to any test that does
// not share state by other means (e.g. integration_test.go is sequential by
// design; see its own comment).

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
	srv := &Server{
		Store: st,
		Log:   slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
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
