package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestRateLimiterAllowsUnderLimit verifies that requests within the limit all pass.
func TestRateLimiterAllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	for i := range 10 {
		if !rl.Allow("192.0.2.1") {
			t.Fatalf("request %d should be allowed (limit=10)", i+1)
		}
	}
}

// TestRateLimiterBlocksOverLimit verifies that the 11th request is denied.
func TestRateLimiterBlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	for range 10 {
		rl.Allow("192.0.2.1")
	}
	if rl.Allow("192.0.2.1") {
		t.Fatal("11th request should be denied")
	}
}

// TestRateLimiterResetsAfterWindow verifies that requests are allowed again
// after the sliding window expires.
func TestRateLimiterResetsAfterWindow(t *testing.T) {
	window := 50 * time.Millisecond
	rl := NewRateLimiter(2, window)

	if !rl.Allow("10.0.0.1") {
		t.Fatal("request 1 should pass")
	}
	if !rl.Allow("10.0.0.1") {
		t.Fatal("request 2 should pass")
	}
	if rl.Allow("10.0.0.1") {
		t.Fatal("request 3 should be denied")
	}

	// Wait for the window to expire.
	time.Sleep(window + 10*time.Millisecond)

	if !rl.Allow("10.0.0.1") {
		t.Fatal("request after window reset should pass")
	}
}

// TestRateLimiterPerIP verifies that different keys have independent limits.
func TestRateLimiterPerIP(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	// Fill ip-A's quota.
	rl.Allow("10.0.0.1")
	rl.Allow("10.0.0.1")
	if rl.Allow("10.0.0.1") {
		t.Fatal("ip-A third request should be denied")
	}

	// ip-B should still have a clean slate.
	if !rl.Allow("10.0.0.2") {
		t.Fatal("ip-B first request should be allowed")
	}
	if !rl.Allow("10.0.0.2") {
		t.Fatal("ip-B second request should be allowed")
	}
	if rl.Allow("10.0.0.2") {
		t.Fatal("ip-B third request should be denied")
	}
}

// ---------------------------------------------------------------------------
// Middleware integration tests (via gin test context)
// ---------------------------------------------------------------------------

func newRateLimitRouter(limit int, window time.Duration, ip string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(limit, window)
	r := gin.New()
	r.Use(RateLimitMiddleware(rl))

	// Override ClientIP by setting X-Forwarded-For.
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func doRequest(r *gin.Engine, ip string) int {
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Forwarded-For", ip)
	req.RemoteAddr = ip + ":9999"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// TestRateLimitMiddlewareAllows verifies that requests under the limit return 200.
func TestRateLimitMiddlewareAllows(t *testing.T) {
	r := newRateLimitRouter(3, time.Minute, "")
	for i := range 3 {
		if code := doRequest(r, "203.0.113.1"); code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, code)
		}
	}
}

// TestRateLimitMiddlewareBlocks verifies that the (limit+1)th request returns 429.
func TestRateLimitMiddlewareBlocks(t *testing.T) {
	r := newRateLimitRouter(3, time.Minute, "")
	for range 3 {
		doRequest(r, "203.0.113.2")
	}
	if code := doRequest(r, "203.0.113.2"); code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", code)
	}
}

// TestRateLimitMiddlewareRetryAfterHeader verifies the Retry-After header is set.
func TestRateLimitMiddlewareRetryAfterHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(1, time.Minute)
	r := gin.New()
	r.Use(RateLimitMiddleware(rl))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "198.51.100.1:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req) // first — passes

	req2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req2.RemoteAddr = "198.51.100.1:1234"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2) // second — blocked

	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w2.Code)
	}
	if got := w2.Header().Get("Retry-After"); got == "" {
		t.Error("expected Retry-After header to be set on 429 response")
	}
}
