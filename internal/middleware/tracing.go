package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// TracingMiddleware returns an OpenTelemetry tracing handler for Gin.
// It propagates trace context from incoming requests and creates a root span
// for each HTTP request, named after the matched route pattern.
func TracingMiddleware() gin.HandlerFunc {
	return otelgin.Middleware("o3k")
}
