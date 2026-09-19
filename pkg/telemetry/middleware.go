package telemetry

import (
	"net/http"

	"ifritah/web-service-gin/pkg/logging"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Middleware extracts a W3C traceparent, creates a server span, and restores
// the request context before downstream Gin middleware and handlers execute.
func (r *Runtime) Middleware() gin.HandlerFunc {
	if r == nil || !r.enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	tracer := r.tracer
	propagator := r.propagator
	return func(c *gin.Context) {
		if c.Request == nil {
			c.Next()
			return
		}

		parent := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := tracer.Start(parent, "HTTP "+boundedHTTPMethod(c.Request.Method),
			trace.WithSpanKind(trace.SpanKindServer))
		c.Request = c.Request.WithContext(ctx)
		defer span.End()

		c.Next()

		status := c.Writer.Status()
		if status == 0 {
			status = http.StatusOK
		}
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := boundedHTTPMethod(c.Request.Method)
		span.SetName(method + " " + route)
		span.SetAttributes(
			attribute.String("http.request.method", method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
		)
		if requestID := logging.RequestIDFromContext(c.Request.Context()); logging.ValidRequestID(requestID) {
			span.SetAttributes(attribute.String("request.id", requestID))
		}
		if serverContext, ok := logging.TrustedServerContextFromContext(c.Request.Context()); ok {
			if serverContext.Tenant != "" {
				span.SetAttributes(attribute.String("tenant.id", serverContext.Tenant))
			}
			if serverContext.CompanyID != "" {
				span.SetAttributes(attribute.String("company.id", serverContext.CompanyID))
			}
		}
		if status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, "http request failed")
		}
	}
}
