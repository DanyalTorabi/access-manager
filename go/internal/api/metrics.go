package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds the Prometheus collectors used by the HTTP middleware and
// application-level instrumentation (e.g. authz counters).
type Metrics struct {
	ReqTotal    *prometheus.CounterVec
	ReqDuration *prometheus.HistogramVec
	// AuthzTotal counts authz handler calls (authzCheck, authzMasks). It is
	// incremented exactly once per request via labels {domain_id, result}
	// where result is "ok" on success and "err" on any failure path
	// (validation, parse, store error). See T50.
	AuthzTotal *prometheus.CounterVec
	// NegativeMaskTotal is bumped whenever the SQLite store reads a
	// negative int64 access mask (treated as 0 by maskFromSQL). Operators
	// can alert on a non-zero value to detect out-of-band rows or legacy
	// data inserted before T46's bit-63 guard. See T50 / T46.
	NegativeMaskTotal prometheus.Counter
}

// Authz result label values used by AuthzTotal.
const (
	authzResultOK  = "ok"
	authzResultErr = "err"
)

// NewMetrics registers HTTP and application metrics on reg and returns them.
// If a collector is already registered (e.g. router rebuilt with the same
// registry), the existing collector is reused so /metrics keeps working.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		ReqTotal:    registerOrReuse(reg, prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests by method, route pattern, and status code."}, []string{"method", "route", "code"})).(*prometheus.CounterVec),
		ReqDuration: registerOrReuse(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request latency in seconds by method and route pattern.", Buckets: prometheus.DefBuckets}, []string{"method", "route"})).(*prometheus.HistogramVec),
		AuthzTotal:  registerOrReuse(reg, prometheus.NewCounterVec(prometheus.CounterOpts{Name: "authz_checks_total", Help: "Total authorization handler calls by domain and outcome (ok/err). Incremented exactly once per request."}, []string{"domain_id", "result"})).(*prometheus.CounterVec),
		NegativeMaskTotal: registerOrReuse(reg, prometheus.NewCounter(prometheus.CounterOpts{Name: "store_negative_mask_observed_total", Help: "Number of negative access-mask values read from the store (treated as 0 by maskFromSQL). Non-zero indicates legacy or out-of-band data."})).(prometheus.Counter),
	}
	return m
}

func registerOrReuse(reg prometheus.Registerer, c prometheus.Collector) prometheus.Collector {
	if err := reg.Register(c); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			return are.ExistingCollector
		}
		panic(err)
	}
	return c
}

// Middleware returns chi-compatible middleware that records request count and
// latency using the route pattern (e.g. /api/v1/domains/{domainID}/users)
// to keep label cardinality bounded. Infrastructure routes (/health, /metrics)
// are excluded so Prometheus scrapes don't skew application metrics.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		if route == "/health" || route == "/metrics" {
			return
		}
		m.ReqTotal.WithLabelValues(r.Method, route, strconv.Itoa(sw.status)).Inc()
		m.ReqDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// statusWriter captures the HTTP status code written by the handler.
// NOTE: wrapping http.ResponseWriter hides optional interfaces (Flusher,
// Hijacker, etc.). If callers ever need those, add an Unwrap method that
// returns the underlying ResponseWriter so http.NewResponseController works.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if sw.wroteHeader {
		return
	}
	sw.status = code
	sw.wroteHeader = true
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wroteHeader {
		sw.wroteHeader = true
	}
	return sw.ResponseWriter.Write(b)
}
