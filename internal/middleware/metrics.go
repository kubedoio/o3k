package middleware

import (
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// registry is the per-process Prometheus registry used by all o3k services.
// A custom registry avoids collisions when multiple service routers are
// initialised in the same test binary or embedded process.
var registry = prometheus.NewRegistry()

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "o3k_http_requests_total",
			Help: "Total number of HTTP requests handled, partitioned by service, method, path and status code.",
		},
		[]string{"service", "method", "path", "status_code"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "o3k_http_request_duration_seconds",
			Help: "HTTP request latency in seconds, partitioned by service, method and path.",
			// Bucket bounds extend beyond prometheus.DefBuckets (max 10s) because
			// OpenStack real-mode operations (nova boot, cinder create, glance
			// upload) regularly take 5–30 seconds; capping at 2.5s collapses the
			// long tail into +Inf and breaks P99 alerting under realistic load.
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
		},
		[]string{"service", "method", "path"},
	)
)

// init registers metrics on the custom registry. We intentionally do not use
// promauto here — that would auto-register on prometheus.DefaultRegisterer,
// defeating the test-isolation and embedding goals of the custom registry.
func init() {
	registry.MustRegister(httpRequestsTotal, httpRequestDuration)
}

// Registry returns the custom Prometheus registry so callers can register
// additional, service-specific metrics (e.g. nova instance gauges).
func Registry() *prometheus.Registry {
	return registry
}

// MetricsMiddleware records o3k_http_requests_total and
// o3k_http_request_duration_seconds for every request handled by the router.
// service identifies the owning OpenStack service (e.g. "keystone", "nova")
// and becomes the value of the "service" label on every sample.
//
// Place this middleware after RequestIDMiddleware but before business-logic
// handlers so it captures the full request lifecycle.
func MetricsMiddleware(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			// Unmatched routes — avoid high-cardinality label explosion.
			path = "unmatched"
		}

		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())
		elapsed := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(service, method, path, status).Inc()
		httpRequestDuration.WithLabelValues(service, method, path).Observe(elapsed)
	}
}

// RegisterMetricsRoute adds GET /metrics to r using the standard Prometheus
// text exposition format (includes # HELP and # TYPE comment lines).
//
// No authentication is applied to /metrics by design — Prometheus scrapers do
// not authenticate. See docs/production-readiness.md §5 (Observability) for
// guidance on restricting access via the reverse proxy or a private interface.
func RegisterMetricsRoute(r *gin.Engine) {
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		// Surface gather errors instead of silently returning partial data.
		ErrorLog:      log.Default(),
		ErrorHandling: promhttp.ContinueOnError,
	})
	r.GET("/metrics", func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	})
}
