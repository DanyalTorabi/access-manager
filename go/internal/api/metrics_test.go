package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
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
		t.Fatalf("failed to read /metrics body: %v", err)
	}
	if !strings.Contains(string(body), "http_requests_total") {
		t.Fatal("metrics body missing http_requests_total")
	}
	if !strings.Contains(string(body), "http_request_duration_seconds") {
		t.Fatal("metrics body missing http_request_duration_seconds")
	}
}

func TestMetrics_authzCheckIncrements(t *testing.T) {
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

	counter := findCounterWithLabel(t, reg, "authz_checks_total", "domain_id", dom.ID)
	if counter < 1 {
		t.Fatalf("authz_checks_total should be >= 1, got %v", counter)
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

func findCounterWithLabel(t *testing.T, reg *prometheus.Registry, name, labelName, labelValue string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() == name {
			for _, m := range m.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == labelName && lp.GetValue() == labelValue {
						return m.GetCounter().GetValue()
					}
				}
			}
		}
	}
	t.Fatalf("metric %q with label %s=%s not found", name, labelName, labelValue)
	return 0
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
			for _, m := range m.GetMetric() {
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
