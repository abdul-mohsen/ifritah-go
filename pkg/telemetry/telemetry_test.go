package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ifritah/web-service-gin/pkg/logging"

	"github.com/gin-gonic/gin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestLoadConfigSupportsOpenObserveEndpointHeadersAndBounds(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://openobserve:5080/api/default")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS", "Authorization=Basic placeholder,stream-name=default")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_INSECURE", "true")
	t.Setenv("OTEL_BSP_SCHEDULE_DELAY", "1000")
	t.Setenv("OTEL_BSP_MAX_QUEUE_SIZE", "8192")
	t.Setenv("OTEL_BSP_MAX_EXPORT_BATCH_SIZE", "1024")

	cfg, err := loadConfigFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Enabled || cfg.Endpoint != "http://openobserve:5080/api/default/v1/traces" || !cfg.Insecure {
		t.Fatalf("config = %#v", cfg)
	}
	if cfg.Headers["Authorization"] != "Basic placeholder" || cfg.Headers["stream-name"] != "default" {
		t.Fatalf("headers = %#v", cfg.Headers)
	}
	if cfg.QueueSize != maxQueueSize || cfg.BatchSize != maxBatchSize {
		t.Fatalf("bounds = queue %d/batch %d", cfg.QueueSize, cfg.BatchSize)
	}
}

func TestTracingDisabledLeavesRequestContextUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runtime := disabledRuntime(nil)
	router := gin.New()
	router.Use(runtime.Middleware())
	router.GET("/disabled", func(c *gin.Context) {
		if trace.SpanContextFromContext(c.Request.Context()).IsValid() {
			t.Error("disabled middleware created a span context")
		}
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/disabled", nil)
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestMiddlewareExtractsW3CContextAndExportsSanitizedException(t *testing.T) {
	gin.SetMode(gin.TestMode)
	exporter := tracetest.NewInMemoryExporter()
	counting := &countingExporter{delegate: exporter}
	runtime, err := newRuntimeWithExporter(context.Background(), Config{
		Enabled:       true,
		Endpoint:      "http://127.0.0.1:4318",
		Insecure:      true,
		ServiceName:   "test-backend",
		BatchTimeout:  time.Hour,
		ExportTimeout: time.Second,
		QueueSize:     16,
		BatchSize:     4,
	}, nil, counting)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Shutdown(context.Background())
	})

	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(logging.New(logging.Config{Level: slog.LevelInfo, Format: "json"}, &output))
	t.Cleanup(func() { slog.SetDefault(previous) })

	router := gin.New()
	router.Use(runtime.Middleware())
	recording := false
	router.GET("/items/:id", func(c *gin.Context) {
		requestContext := logging.WithRequestID(c.Request.Context(), "req-test")
		requestContext = logging.WithServerContext(requestContext, logging.ServerContext{
			Tenant:    "tenant-a",
			CompanyID: "7",
		})
		c.Request = c.Request.WithContext(requestContext)
		recording = trace.SpanFromContext(c.Request.Context()).IsRecording()
		logging.LogInfo(c.Request.Context(), "test.trace_event")
		RecordError(c.Request.Context(), errors.New("SELECT password FROM users WHERE token = 'secret'"), "db.lookup")
		c.Status(http.StatusInternalServerError)
	})

	request := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if !recording {
		t.Fatal("server span is not recording")
	}

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log: %v; output=%s", err, output.String())
	}
	if record["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" ||
		len(record["span_id"].(string)) != 16 {
		t.Fatalf("trace fields = %#v", record)
	}
	if strings.Contains(output.String(), "SELECT password") || strings.Contains(output.String(), "secret") {
		t.Fatalf("raw exception text leaked into logs: %s", output.String())
	}

	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported spans = %d, want 1 (export calls=%d, spans=%d)", len(spans), counting.calls, counting.spans)
	}
	span := spans[0]
	if span.Parent.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" ||
		span.SpanKind != trace.SpanKindServer ||
		span.Name != "GET /items/:id" {
		t.Fatalf("server span = %#v", span)
	}
	spanAttrs := make(map[string]string)
	for _, attr := range span.Attributes {
		spanAttrs[string(attr.Key)] = attr.Value.AsString()
	}
	if spanAttrs["request.id"] == "" {
		t.Fatalf("request.id span attribute missing: %#v", spanAttrs)
	}
	if spanAttrs["tenant.id"] != "tenant-a" || spanAttrs["company.id"] != "7" {
		t.Fatalf("tenant/company span attributes = %#v", spanAttrs)
	}
	if span.Status.Code.String() != "Error" {
		t.Fatalf("span status = %s, want Error", span.Status.Code)
	}
	if len(span.Events) != 1 || span.Events[0].Name != "exception" {
		t.Fatalf("span events = %#v", span.Events)
	}
	eventAttrs := make(map[string]string)
	for _, attr := range span.Events[0].Attributes {
		eventAttrs[string(attr.Key)] = attr.Value.AsString()
	}
	if eventAttrs["exception.message"] != logging.RedactedValue ||
		eventAttrs["exception.operation"] != "db.lookup" ||
		strings.Contains(eventAttrs["exception.type"], "SELECT") {
		t.Fatalf("exception attrs = %#v", eventAttrs)
	}
}

func TestResilientExporterDropsFailuresWithoutLeakingErrorText(t *testing.T) {
	var output bytes.Buffer
	logger := logging.New(logging.Config{Level: slog.LevelInfo, Format: "json"}, &output)
	resilient := &resilientExporter{
		exporter: failingExporter{err: errors.New("authorization secret should not be logged")},
		logger:   logger,
	}

	if err := resilient.ExportSpans(context.Background(), nil); err != nil {
		t.Fatalf("export error = %v, want fail-open nil", err)
	}
	if strings.Contains(output.String(), "authorization secret") {
		t.Fatalf("exporter error text leaked: %s", output.String())
	}
	if !strings.Contains(output.String(), "telemetry.export_failed") {
		t.Fatalf("missing bounded exporter warning: %s", output.String())
	}
}

type failingExporter struct {
	err error
}

type countingExporter struct {
	delegate sdktrace.SpanExporter
	calls    int
	spans    int
}

func (e *countingExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.calls++
	e.spans += len(spans)
	return e.delegate.ExportSpans(ctx, spans)
}

func (e *countingExporter) Shutdown(ctx context.Context) error {
	return nil
}

func (f failingExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	return f.err
}

func (failingExporter) Shutdown(context.Context) error {
	return nil
}
