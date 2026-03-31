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
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		ReqTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by method, route pattern, and status code.",
		}, []string{"method", "route", "code"}),

		ReqDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds by method and route pattern.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),

		AuthzTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "authz_checks_total",
			Help: "Total authorization check calls by domain.",
		}, []string{"domain_id"}),
	}
	for _, c := range []prometheus.Collector{m.ReqTotal, m.ReqDuration, m.AuthzTotal} {
		if err := reg.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if errors.As(err, &are) {
				continue
			}
			panic(err)
		}
	}
	return m
}

// Middleware returns chi-compatible middleware that records request count and
// latency using the route pattern (e.g. /api/v1/domains/{domainID}/users)
// to keep label cardinality bounded.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
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
	if !sw.wroteHeader {
		sw.status = code
		sw.wroteHeader = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wroteHeader {
		sw.wroteHeader = true
	}
	return sw.ResponseWriter.Write(b)
}
