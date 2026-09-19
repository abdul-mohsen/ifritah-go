package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"ifritah/web-service-gin/pkg/logging"
	"ifritah/web-service-gin/pkg/telemetry"

	"github.com/gin-gonic/gin"
)

type MetricsObserver interface {
	ObserveRequest(method, route string, status int, duration time.Duration)
}

type Config struct {
	Logger        *slog.Logger
	ServerContext logging.ServerContext
	Metrics       MetricsObserver
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		ensureRequestMetadata(c, logging.ServerContext{})
		c.Next()
	}
}

func RequestCompletion(config Config) gin.HandlerFunc {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		// Deployment context is attached by authenticated middleware only.
		// Applying Config.ServerContext here would leak process-global tenant
		// metadata into unauthenticated completion events.
		requestID := ensureRequestMetadata(c, logging.ServerContext{})
		startedAt := time.Now()

		defer func() {
			duration := time.Since(startedAt)
			status := c.Writer.Status()
			if status == 0 {
				status = http.StatusOK
			}
			route := c.FullPath()
			if route == "" {
				route = "unmatched"
			}
			method := boundedMethod(c.Request.Method)
			requestID = strings.ReplaceAll(strings.ReplaceAll(requestID, "\r", " "), "\n", " ")
			method = strings.ReplaceAll(strings.ReplaceAll(method, "\r", " "), "\n", " ")
			route = strings.ReplaceAll(strings.ReplaceAll(route, "\r", " "), "\n", " ")

			attrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("method", method),
				slog.String("route", route),
				slog.Int("status", status),
				slog.Int64("duration_ms", duration.Milliseconds()),
			}
			if userID, ok := logging.TrustedUserIDFromContext(c.Request.Context()); ok {
				attrs = append(attrs, slog.Int64("user_id", userID))
			}
			if serverContext, ok := logging.TrustedServerContextFromContext(c.Request.Context()); ok {
				if serverContext.Tenant != "" {
					tenant := strings.ReplaceAll(strings.ReplaceAll(serverContext.Tenant, "\r", " "), "\n", " ")
					attrs = append(attrs, slog.String("tenant", tenant))
					attrs = append(attrs, slog.String("tenant_id", tenant))
				}
				if serverContext.CompanyID != "" {
					companyID := strings.ReplaceAll(strings.ReplaceAll(serverContext.CompanyID, "\r", " "), "\n", " ")
					attrs = append(attrs, slog.String("company_id", companyID))
				}
			}

			switch {
			case status >= http.StatusInternalServerError:
				logger.LogAttrs(c.Request.Context(), slog.LevelError, "http.request.completed", attrs...)
			case status >= http.StatusBadRequest:
				logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "http.request.completed", attrs...)
			default:
				logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "http.request.completed", attrs...)
			}

			if config.Metrics != nil {
				config.Metrics.ObserveRequest(method, route, status, duration)
			}
		}()

		c.Next()
	}
}

func RequestLogging(config Config) gin.HandlerFunc {
	return RequestCompletion(config)
}

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		panicType := "unknown"
		if recoveredType := reflect.TypeOf(recovered); recoveredType != nil {
			panicType = recoveredType.String()
		}
		telemetry.RecordPanic(c.Request.Context(), panicType)
		logger.LogAttrs(c.Request.Context(), slog.LevelError, "http.panic_recovered",
			slog.String("type", panicType))
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

func RequestIDMiddleware() gin.HandlerFunc {
	return RequestID()
}

func RequestCompletionMiddleware(config Config) gin.HandlerFunc {
	return RequestCompletion(config)
}

func RequestLoggingMiddleware(config Config) gin.HandlerFunc {
	return RequestLogging(config)
}

func ensureRequestMetadata(c *gin.Context, serverContext logging.ServerContext) string {
	if c.Request == nil {
		return ""
	}
	requestID := logging.RequestIDFromContext(c.Request.Context())
	if !logging.ValidRequestID(requestID) {
		requestID = c.GetHeader(logging.RequestIDHeader)
	}
	if !logging.ValidRequestID(requestID) {
		requestID = logging.NewRequestID()
	}

	requestContext := logging.WithRequestID(c.Request.Context(), requestID)
	if serverContext.Tenant != "" || serverContext.CompanyID != "" {
		requestContext = logging.WithServerContext(requestContext, serverContext)
	}
	c.Request = c.Request.WithContext(requestContext)
	c.Header(logging.RequestIDHeader, requestID)
	return requestID
}

func boundedMethod(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return strings.ToUpper(method)
	default:
		return "OTHER"
	}
}
