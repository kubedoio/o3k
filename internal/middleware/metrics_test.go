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

func newMetricsTestRouter(service string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(MetricsMiddleware(service))
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	RegisterMetricsRoute(r)
	return r
}

func TestMetricsMiddleware_IncrementsCounter(t *testing.T) {
	// Reset the counter so earlier test runs don't bleed in.
	httpRequestsTotal.Reset()

	r := newMetricsTestRouter("test")

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Gather the counter family from the registry and verify the label set.
	mf, err := registry.Gather()
	require.NoError(t, err)

	var found *dto.MetricFamily
	for _, f := range mf {
		if f.GetName() == "o3k_http_requests_total" {
			found = f
			break
		}
	}
	require.NotNil(t, found, "o3k_http_requests_total not found in gathered metrics")

	// Find the sample with our exact label combination.
	var matched bool
	for _, m := range found.GetMetric() {
		labels := make(map[string]string)
		for _, lp := range m.GetLabel() {
			labels[lp.GetName()] = lp.GetValue()
		}
		if labels["service"] == "test" &&
			labels["method"] == http.MethodGet &&
			labels["path"] == "/ping" &&
			labels["status_code"] == "200" {
			assert.Equal(t, float64(1), m.GetCounter().GetValue(),
				"counter should be 1 after one request")
			matched = true
		}
	}
	assert.True(t, matched, "no metric sample matched expected labels {service=test method=GET path=/ping status_code=200}")
}

func TestRegisterMetricsRoute_Returns200WithCorrectContentType(t *testing.T) {
	r := newMetricsTestRouter("test")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	ct := w.Header().Get("Content-Type")
	assert.True(t, strings.HasPrefix(ct, "text/plain"),
		"Content-Type should start with text/plain, got %q", ct)
}

func TestMetricsEndpoint_ContainsHelpLine(t *testing.T) {
	// Fire one request so the counter metric is present in the output.
	httpRequestsTotal.Reset()
	r := newMetricsTestRouter("test")

	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/ping", nil))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "# HELP o3k_http_requests_total",
		"/metrics output must include a HELP comment for o3k_http_requests_total")
}
