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
	AuthzTotal  *prometheus.CounterVec
}

// NewMetrics registers HTTP and application metrics on reg and returns them.
// If a collector is already registered (e.g. router rebuilt with the same
// registry), the existing collector is reused so /metrics keeps working.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		ReqTotal:    registerOrReuse(reg, prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests by method, route pattern, and status code."}, []string{"method", "route", "code"})).(*prometheus.CounterVec),
		ReqDuration: registerOrReuse(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request latency in seconds by method and route pattern.", Buckets: prometheus.DefBuckets}, []string{"method", "route"})).(*prometheus.HistogramVec),
		AuthzTotal:  registerOrReuse(reg, prometheus.NewCounterVec(prometheus.CounterOpts{Name: "authz_checks_total", Help: "Total authorization check calls by domain."}, []string{"domain_id"})).(*prometheus.CounterVec),
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
