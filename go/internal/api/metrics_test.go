package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
	"github.com/dtorabi/access-manager/internal/testutil"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
)

func newTestAPIWithMetrics(t *testing.T) (*httptest.Server, store.Store, *prometheus.Registry) {
	t.Helper()
	st, cleanup := newTestStore(t)
	reg := prometheus.NewRegistry()
	srv := &Server{Store: st}
	ts := httptest.NewServer(srv.Router(reg, reg))
	t.Cleanup(func() {
		ts.Close()
		cleanup()
	})
	return ts, st, reg
}

func TestMetrics_apiRouteIncrementsCounter(t *testing.T) {
	ts, _, reg := newTestAPIWithMetrics(t)

	res, err := http.Get(ts.URL + "/api/v1/domains")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}

	counter := findCounter(t, reg, "http_requests_total")
	if counter < 1 {
		t.Fatalf("http_requests_total should be >= 1, got %v", counter)
	}
}

func TestMetrics_healthExcludedFromCounter(t *testing.T) {
	ts, _, reg := newTestAPIWithMetrics(t)

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() == "http_requests_total" {
			t.Fatal("http_requests_total should not exist after only /health requests")
		}
	}
}

func TestMetrics_requestDurationRecorded(t *testing.T) {
	ts, _, reg := newTestAPIWithMetrics(t)

	res, err := http.Get(ts.URL + "/api/v1/domains")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}

	count := findHistogramSampleCount(t, reg, "http_request_duration_seconds")
	if count < 1 {
		t.Fatalf("http_request_duration_seconds should have >= 1 observation, got %d", count)
	}
}

func TestMetrics_endpoint(t *testing.T) {
	ts, _, _ := newTestAPIWithMetrics(t)

	res, err := http.Get(ts.URL + "/api/v1/domains")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("/api/v1/domains want 200, got %d", res.StatusCode)
	}

	res, err = http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read /metrics body: %v", err)
	}
	if !strings.Contains(string(body), "http_requests_total") {
		t.Fatal("metrics body missing http_requests_total")
	}
	if !strings.Contains(string(body), "http_request_duration_seconds") {
		t.Fatal("metrics body missing http_request_duration_seconds")
	}
}

func TestMetrics_authzCheckIncrementsExactlyOnce(t *testing.T) {
	ts, _, reg := newTestAPIWithMetrics(t)

	var dom store.Domain
	mustUnmarshal(t, mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom)

	var user store.User
	mustUnmarshal(t, mustPostJSON201(t, ts.URL+"/api/v1/domains/"+dom.ID+"/users", `{"title":"u"}`), &user)

	var resource store.Resource
	mustUnmarshal(t, mustPostJSON201(t, ts.URL+"/api/v1/domains/"+dom.ID+"/resources", `{"title":"r"}`), &resource)

	url := ts.URL + "/api/v1/domains/" + dom.ID + "/authz/check?user_id=" + user.ID + "&resource_id=" + resource.ID + "&access_bit=1"
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}

	if got := findCounterWithLabels(t, reg, "authz_checks_total", map[string]string{"domain_id": dom.ID, "result": "ok"}); got != 1 {
		t.Fatalf("authz_checks_total{domain_id=%q,result=ok} want 1, got %v", dom.ID, got)
	}
	if got := findCounterWithLabelsOrZero(t, reg, "authz_checks_total", map[string]string{"domain_id": dom.ID, "result": "err"}); got != 0 {
		t.Fatalf("authz_checks_total{result=err} want 0, got %v", got)
	}
}

func TestMetrics_authzMasksIncrementsExactlyOnce(t *testing.T) {
	ts, _, reg := newTestAPIWithMetrics(t)

	var dom store.Domain
	mustUnmarshal(t, mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom)

	var user store.User
	mustUnmarshal(t, mustPostJSON201(t, ts.URL+"/api/v1/domains/"+dom.ID+"/users", `{"title":"u"}`), &user)

	var resource store.Resource
	mustUnmarshal(t, mustPostJSON201(t, ts.URL+"/api/v1/domains/"+dom.ID+"/resources", `{"title":"r"}`), &resource)

	url := ts.URL + "/api/v1/domains/" + dom.ID + "/authz/masks?user_id=" + user.ID + "&resource_id=" + resource.ID
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}

	if got := findCounterWithLabels(t, reg, "authz_checks_total", map[string]string{"domain_id": dom.ID, "result": "ok"}); got != 1 {
		t.Fatalf("authz_checks_total{result=ok} want 1, got %v", got)
	}
}

// TestMetrics_authzCheckValidationErrorIncrementsErr verifies that a
// missing-parameter request increments authz_checks_total exactly once with
// result="err" (regression: previously the error path was uncounted).
func TestMetrics_authzCheckValidationErrorIncrementsErr(t *testing.T) {
	ts, _, reg := newTestAPIWithMetrics(t)

	var dom store.Domain
	mustUnmarshal(t, mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom)

	// Missing user_id, resource_id, access_bit — handler should 400 early.
	res, err := http.Get(ts.URL + "/api/v1/domains/" + dom.ID + "/authz/check")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}

	if got := findCounterWithLabels(t, reg, "authz_checks_total", map[string]string{"domain_id": dom.ID, "result": "err"}); got != 1 {
		t.Fatalf("authz_checks_total{result=err} want 1, got %v", got)
	}
	if got := findCounterWithLabelsOrZero(t, reg, "authz_checks_total", map[string]string{"domain_id": dom.ID, "result": "ok"}); got != 0 {
		t.Fatalf("authz_checks_total{result=ok} want 0, got %v", got)
	}
}

// TestMetrics_authzMasksValidationErrorIncrementsErr mirrors the authzCheck
// error-path test for authzMasks.
func TestMetrics_authzMasksValidationErrorIncrementsErr(t *testing.T) {
	ts, _, reg := newTestAPIWithMetrics(t)

	var dom store.Domain
	mustUnmarshal(t, mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom)

	res, err := http.Get(ts.URL + "/api/v1/domains/" + dom.ID + "/authz/masks")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}

	if got := findCounterWithLabels(t, reg, "authz_checks_total", map[string]string{"domain_id": dom.ID, "result": "err"}); got != 1 {
		t.Fatalf("authz_checks_total{result=err} want 1, got %v", got)
	}
}

// newBrokenTestAPIWithMetrics returns a server backed by a closed DB so any
// store call returns an error. A real Prometheus registry is wired so tests
// can assert authz error-path counters. See T50.
func newBrokenTestAPIWithMetrics(t *testing.T) (*httptest.Server, *prometheus.Registry) {
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
	reg := prometheus.NewRegistry()
	ts := httptest.NewServer(srv.Router(reg, reg))
	t.Cleanup(ts.Close)
	return ts, reg
}

// TestMetrics_authzCheckStoreErrorIncrementsErr asserts that a failing
// EffectiveMask store call still bumps authz_checks_total exactly once with
// result="err" — regression for the previous double-increment bug where
// the post-store Inc was skipped on early return.
func TestMetrics_authzCheckStoreErrorIncrementsErr(t *testing.T) {
	ts, reg := newBrokenTestAPIWithMetrics(t)

	domID := uuid.NewString()
	url := ts.URL + "/api/v1/domains/" + domID + "/authz/check?user_id=" + uuid.NewString() + "&resource_id=" + uuid.NewString() + "&access_bit=1"
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", res.StatusCode)
	}

	if got := findCounterWithLabels(t, reg, "authz_checks_total", map[string]string{"domain_id": domID, "result": "err"}); got != 1 {
		t.Fatalf("authz_checks_total{result=err} want 1, got %v", got)
	}
	if got := findCounterWithLabelsOrZero(t, reg, "authz_checks_total", map[string]string{"domain_id": domID, "result": "ok"}); got != 0 {
		t.Fatalf("authz_checks_total{result=ok} want 0, got %v", got)
	}
}

// TestMetrics_authzMasksStoreErrorIncrementsErr is the authzMasks
// counterpart to TestMetrics_authzCheckStoreErrorIncrementsErr.
func TestMetrics_authzMasksStoreErrorIncrementsErr(t *testing.T) {
	ts, reg := newBrokenTestAPIWithMetrics(t)

	domID := uuid.NewString()
	url := ts.URL + "/api/v1/domains/" + domID + "/authz/masks?user_id=" + uuid.NewString() + "&resource_id=" + uuid.NewString()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", res.StatusCode)
	}

	if got := findCounterWithLabels(t, reg, "authz_checks_total", map[string]string{"domain_id": domID, "result": "err"}); got != 1 {
		t.Fatalf("authz_checks_total{result=err} want 1, got %v", got)
	}
}

func findCounter(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() == name {
			var total float64
			for _, m := range mf.GetMetric() {
				total += m.GetCounter().GetValue()
			}
			return total
		}
	}
	t.Fatalf("metric %q not found", name)
	return 0
}

// findCounterWithLabels returns the counter sample whose labels contain
// every name/value pair in want. It fails the test if no such sample
// exists. Use findCounterWithLabelsOrZero to assert a specific label
// combination has not been observed.
func findCounterWithLabels(t *testing.T, reg *prometheus.Registry, name string, want map[string]string) float64 {
	t.Helper()
	v, ok := lookupCounterWithLabels(t, reg, name, want)
	if !ok {
		t.Fatalf("metric %q with labels %v not found", name, want)
	}
	return v
}

func findCounterWithLabelsOrZero(t *testing.T, reg *prometheus.Registry, name string, want map[string]string) float64 {
	t.Helper()
	v, _ := lookupCounterWithLabels(t, reg, name, want)
	return v
}

func lookupCounterWithLabels(t *testing.T, reg *prometheus.Registry, name string, want map[string]string) (float64, bool) {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			got := map[string]string{}
			for _, lp := range m.GetLabel() {
				got[lp.GetName()] = lp.GetValue()
			}
			match := true
			for k, v := range want {
				if got[k] != v {
					match = false
					break
				}
			}
			if match {
				return m.GetCounter().GetValue(), true
			}
		}
	}
	return 0, false
}

func findHistogramSampleCount(t *testing.T, reg *prometheus.Registry, name string) uint64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() == name {
			var total uint64
			for _, m := range mf.GetMetric() {
				total += m.GetHistogram().GetSampleCount()
			}
			return total
		}
	}
	t.Fatalf("metric %q not found", name)
	return 0
}

func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

// --- registerOrReuse tests ---

func TestNewMetrics_idempotent(t *testing.T) {
	reg := prometheus.NewRegistry()
	m1 := NewMetrics(reg)
	m2 := NewMetrics(reg)
	if m1.ReqTotal != m2.ReqTotal {
		t.Fatal("second NewMetrics should reuse existing ReqTotal collector")
	}
	if m1.ReqDuration != m2.ReqDuration {
		t.Fatal("second NewMetrics should reuse existing ReqDuration collector")
	}
	if m1.AuthzTotal != m2.AuthzTotal {
		t.Fatal("second NewMetrics should reuse existing AuthzTotal collector")
	}
}

// --- statusWriter tests ---

func TestStatusWriter_doubleWriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	sw.WriteHeader(http.StatusCreated)
	sw.WriteHeader(http.StatusNotFound)
	if sw.status != http.StatusCreated {
		t.Fatalf("status should be first WriteHeader value 201, got %d", sw.status)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("underlying writer should have 201, got %d", w.Code)
	}
}

func TestStatusWriter_writeWithoutWriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	_, err := sw.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if sw.status != http.StatusOK {
		t.Fatalf("status should default to 200, got %d", sw.status)
	}
	if !sw.wroteHeader {
		t.Fatal("wroteHeader should be true after Write")
	}
}

func TestMetrics_errorResponseCounted(t *testing.T) {
	ts, _, reg := newTestAPIWithMetrics(t)
	res, err := http.Get(ts.URL + "/api/v1/domains/nonexistent/users/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	counter := findCounter(t, reg, "http_requests_total")
	if counter < 1 {
		t.Fatalf("error responses should be counted, got %v", counter)
	}
}
