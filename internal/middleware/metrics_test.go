package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetMetrics zeroes both the counter and the histogram. Both are
// package-level vars shared across tests; resetting only one leaves the other
// accumulating samples across runs and order-dependent.
func resetMetrics() {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()
}

// newMetricsTestRouter mirrors the wiring in cmd/o3k/main.go: /metrics is
// registered before MetricsMiddleware so the metrics endpoint itself is not
// counted (gin global middleware only attaches to routes registered after).
func newMetricsTestRouter(service string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterMetricsRoute(r)
	r.Use(MetricsMiddleware(service))
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	r.GET("/boom", func(c *gin.Context) {
		c.String(http.StatusInternalServerError, "boom")
	})
	return r
}

// findCounter returns the first counter sample whose labels exactly match
// want, or nil if the family is empty or no sample matches.
func findCounter(t *testing.T, name string, want map[string]string) *dto.Metric {
	t.Helper()
	mf, err := registry.Gather()
	require.NoError(t, err)

	var found *dto.MetricFamily
	for _, f := range mf {
		if f.GetName() == name {
			found = f
			break
		}
	}
	if found == nil {
		// Family absent (no observations recorded yet) is a valid state for
		// tests that assert "this label combo was NOT recorded."
		return nil
	}

	for _, m := range found.GetMetric() {
		labels := make(map[string]string, len(m.GetLabel()))
		for _, lp := range m.GetLabel() {
			labels[lp.GetName()] = lp.GetValue()
		}
		if labelsMatch(labels, want) {
			return m
		}
	}
	return nil
}

// findHistogram returns the first histogram sample whose labels exactly match
// want, or nil if the family is empty or no sample matches.
func findHistogram(t *testing.T, name string, want map[string]string) *dto.Metric {
	t.Helper()
	mf, err := registry.Gather()
	require.NoError(t, err)

	var found *dto.MetricFamily
	for _, f := range mf {
		if f.GetName() == name {
			found = f
			break
		}
	}
	if found == nil {
		return nil
	}

	for _, m := range found.GetMetric() {
		labels := make(map[string]string, len(m.GetLabel()))
		for _, lp := range m.GetLabel() {
			labels[lp.GetName()] = lp.GetValue()
		}
		if labelsMatch(labels, want) {
			return m
		}
	}
	return nil
}

func labelsMatch(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

func TestMetricsMiddleware_IncrementsCounter(t *testing.T) {
	resetMetrics()
	r := newMetricsTestRouter("test")

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	m := findCounter(t, "o3k_http_requests_total", map[string]string{
		"service":     "test",
		"method":      http.MethodGet,
		"path":        "/ping",
		"status_code": "200",
	})
	require.NotNil(t, m, "no counter sample matched expected labels")
	assert.Equal(t, float64(1), m.GetCounter().GetValue())
}

func TestMetricsMiddleware_RecordsHistogram(t *testing.T) {
	resetMetrics()
	r := newMetricsTestRouter("test")

	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/ping", nil))

	// The histogram does NOT carry status_code — keep that asymmetry asserted
	// so a future "let's just add status_code" change is caught.
	m := findHistogram(t, "o3k_http_request_duration_seconds", map[string]string{
		"service": "test",
		"method":  http.MethodGet,
		"path":    "/ping",
	})
	require.NotNil(t, m, "no histogram sample matched expected labels")

	h := m.GetHistogram()
	assert.Equal(t, uint64(1), h.GetSampleCount(), "exactly one observation expected")
	assert.GreaterOrEqual(t, h.GetSampleSum(), float64(0), "sample sum must be non-negative")
}

func TestMetricsMiddleware_5xxRecordedWithCorrectStatus(t *testing.T) {
	resetMetrics()
	r := newMetricsTestRouter("test")

	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/boom", nil))

	m := findCounter(t, "o3k_http_requests_total", map[string]string{
		"service":     "test",
		"method":      http.MethodGet,
		"path":        "/boom",
		"status_code": "500",
	})
	require.NotNil(t, m, "5xx status must be recorded with status_code=500 — alerting depends on this")
	assert.Equal(t, float64(1), m.GetCounter().GetValue())
}

func TestMetricsMiddleware_UnmatchedRouteCollapsesToSentinel(t *testing.T) {
	resetMetrics()
	r := newMetricsTestRouter("test")

	// Hit two distinct unmatched paths — both must collapse to path="unmatched".
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/does-not-exist/aaaa", nil))
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/does-not-exist/bbbb", nil))

	m := findCounter(t, "o3k_http_requests_total", map[string]string{
		"service":     "test",
		"method":      http.MethodGet,
		"path":        "unmatched",
		"status_code": "404",
	})
	require.NotNil(t, m, "unmatched routes must use path=\"unmatched\" — cardinality safety")
	assert.Equal(t, float64(2), m.GetCounter().GetValue(),
		"both unmatched requests must collapse into the same series")
}

func TestMetricsMiddleware_ServiceLabelIsolation(t *testing.T) {
	resetMetrics()
	rA := newMetricsTestRouter("svc-a")
	rB := newMetricsTestRouter("svc-b")

	rA.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))
	rB.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))

	for _, svc := range []string{"svc-a", "svc-b"} {
		m := findCounter(t, "o3k_http_requests_total", map[string]string{
			"service":     svc,
			"method":      http.MethodGet,
			"path":        "/ping",
			"status_code": "200",
		})
		require.NotNilf(t, m, "service=%s must produce its own series", svc)
		assert.Equalf(t, float64(1), m.GetCounter().GetValue(),
			"service=%s counter cross-contamination", svc)
	}
}

func TestRegisterMetricsRoute_Returns200WithCorrectContentType(t *testing.T) {
	resetMetrics()
	r := newMetricsTestRouter("test")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	ct := w.Header().Get("Content-Type")
	assert.Truef(t, strings.HasPrefix(ct, "text/plain"),
		"Content-Type should start with text/plain, got %q", ct)
}

func TestMetricsEndpoint_ContainsHelpLines(t *testing.T) {
	resetMetrics()
	r := newMetricsTestRouter("test")

	// Fire one request so both counter and histogram are populated.
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/ping", nil))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "# HELP o3k_http_requests_total",
		"/metrics output must include HELP for o3k_http_requests_total")
	assert.Contains(t, body, "# HELP o3k_http_request_duration_seconds",
		"/metrics output must include HELP for o3k_http_request_duration_seconds")
}

func TestMetricsEndpoint_NotCountedAgainstItself(t *testing.T) {
	resetMetrics()
	r := newMetricsTestRouter("test")

	// Hit /metrics directly — because RegisterMetricsRoute is called BEFORE
	// MetricsMiddleware in the test router (matching production), the metrics
	// endpoint itself should not be recorded as a request.
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/metrics", nil))

	m := findCounter(t, "o3k_http_requests_total", map[string]string{
		"service":     "test",
		"method":      http.MethodGet,
		"path":        "/metrics",
		"status_code": "200",
	})
	assert.Nil(t, m, "/metrics must not count itself — it sits before the middleware in the chain")
}
