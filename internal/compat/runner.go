package compat

import (
	"net/http"
	"sync"
)

// Recorder captures HTTP request/response pairs for compat analysis.
type Recorder struct {
	mu      sync.Mutex
	results []EndpointResult
}

// NewRecorder returns an empty Recorder.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Record appends an observed API call to the recorder.
func (r *Recorder) Record(method, path string, statusCode int) {
	compatible := isCompatibleStatus(method, statusCode)
	r.mu.Lock()
	r.results = append(r.results, EndpointResult{
		Method:     method,
		Path:       path,
		Called:     true,
		StatusCode: statusCode,
		Compatible: compatible,
	})
	r.mu.Unlock()
}

// Results returns a snapshot copy of all recorded endpoint results.
func (r *Recorder) Results() []EndpointResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]EndpointResult, len(r.results))
	copy(out, r.results)
	return out
}

// Middleware wraps an http.Handler to record each request's status code.
func (r *Recorder) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, code: 200}
		next.ServeHTTP(rw, req)
		r.Record(req.Method, req.URL.Path, rw.code)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

// isCompatibleStatus reports whether the given method/status pair is
// considered OpenStack-compatible.
func isCompatibleStatus(method string, code int) bool {
	if code >= 200 && code < 300 {
		return true
	}
	if method == "GET" && code == 404 {
		return true
	}
	if code == 409 {
		return true
	}
	return false
}
