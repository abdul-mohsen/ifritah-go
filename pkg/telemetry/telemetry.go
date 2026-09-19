package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"ifritah/web-service-gin/pkg/logging"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	instrumentationName = "ifritah/web-service-gin"
	defaultServiceName  = "ifritah-backend"

	defaultBatchTimeout  = 2 * time.Second
	defaultExportTimeout = 5 * time.Second
	defaultShutdownLimit = 5 * time.Second
	defaultQueueSize     = 2048
	defaultBatchSize     = 256
	maxExportRequestSize = 2 * 1024 * 1024

	maxBatchTimeout   = 30 * time.Second
	maxExportTimeout  = 10 * time.Second
	maxQueueSize      = 8192
	maxBatchSize      = 1024
	maxHeaderBytes    = 4096
	maxHeaderCount    = 16
	maxHeaderKeyLen   = 128
	maxHeaderValueLen = 1024
	maxValueLen       = 128
	maxOperationLen   = 64
)

// Config contains the bounded OTLP trace exporter settings used by the
// backend. Tracing is disabled unless Enabled is true and Endpoint is set.
type Config struct {
	Enabled        bool
	Endpoint       string
	Headers        map[string]string
	Insecure       bool
	ServiceName    string
	ServiceVersion string

	BatchTimeout  time.Duration
	ExportTimeout time.Duration
	QueueSize     int
	BatchSize     int
}

// Runtime owns the process-wide tracer provider and request middleware.
// Disabled runtimes intentionally leave request contexts and headers alone.
type Runtime struct {
	enabled     bool
	provider    trace.TracerProvider
	sdkProvider *sdktrace.TracerProvider
	tracer      trace.Tracer
	propagator  propagation.TextMapPropagator
	logger      *slog.Logger

	shutdownOnce sync.Once
	shutdownErr  error
}

// Setup loads environment configuration and fails open to a no-op runtime
// when telemetry is absent or misconfigured. Exporter failures are handled by
// a bounded asynchronous processor and never fail an HTTP request.
func Setup(ctx context.Context, logger *slog.Logger) *Runtime {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		logWarning(logger, "telemetry.config_invalid", err)
		return disabledRuntime(logger)
	}

	runtime, err := New(ctx, cfg, logger)
	if err != nil {
		logWarning(logger, "telemetry.exporter_init_failed", err)
		return disabledRuntime(logger)
	}
	return runtime
}

// New constructs a runtime without changing OpenTelemetry globals. Call
// InstallGlobal once during process startup when dependency spans should use
// this provider.
func New(ctx context.Context, cfg Config, logger *slog.Logger) (*Runtime, error) {
	if !cfg.Enabled {
		return disabledRuntime(logger), nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	exporter, err := otlptracehttp.New(ctx, exporterOptions(cfg)...)
	if err != nil {
		return nil, err
	}
	return newRuntimeWithExporter(ctx, cfg, logger, exporter)
}

func newRuntimeWithExporter(
	ctx context.Context,
	cfg Config,
	logger *slog.Logger,
	exporter sdktrace.SpanExporter,
) (*Runtime, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if exporter == nil {
		return nil, errors.New("telemetry exporter is nil")
	}
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}

	serviceAttrs := []attribute.KeyValue{
		attribute.String("service.name", cfg.ServiceName),
	}
	if cfg.ServiceVersion != "" {
		serviceAttrs = append(serviceAttrs, attribute.String("service.version", cfg.ServiceVersion))
	}
	telemetryResource, err := resource.New(ctx, resource.WithAttributes(serviceAttrs...))
	if err != nil {
		return nil, err
	}

	resilient := &resilientExporter{
		exporter: exporter,
		logger:   logger,
	}
	processor := sdktrace.NewBatchSpanProcessor(
		resilient,
		sdktrace.WithBatchTimeout(cfg.BatchTimeout),
		sdktrace.WithExportTimeout(cfg.ExportTimeout),
		sdktrace.WithMaxQueueSize(cfg.QueueSize),
		sdktrace.WithMaxExportBatchSize(cfg.BatchSize),
	)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(telemetryResource),
		sdktrace.WithSpanProcessor(processor),
	)

	return &Runtime{
		enabled:     true,
		provider:    provider,
		sdkProvider: provider,
		tracer:      provider.Tracer(instrumentationName),
		propagator:  propagation.TraceContext{},
		logger:      logger,
	}, nil
}

// InstallGlobal makes dependency helpers use this runtime's provider and
// enables W3C trace-context propagation for outgoing requests.
func (r *Runtime) InstallGlobal() {
	if r == nil || !r.enabled {
		return
	}
	otel.SetTracerProvider(r.provider)
	otel.SetTextMapPropagator(r.propagator)
}

func (r *Runtime) Enabled() bool {
	return r != nil && r.enabled
}

func (r *Runtime) Provider() trace.TracerProvider {
	if r == nil || r.provider == nil {
		return trace.NewNoopTracerProvider()
	}
	return r.provider
}

// Shutdown flushes bounded pending spans without allowing telemetry to hold
// the process open indefinitely. Exporter errors are returned for startup
// logging but are never promoted to application failures.
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil || !r.enabled || r.sdkProvider == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultShutdownLimit)
		defer cancel()
	}

	r.shutdownOnce.Do(func() {
		if err := r.sdkProvider.ForceFlush(ctx); err != nil {
			logWarning(r.logger, "telemetry.flush_failed", err)
		}
		r.shutdownErr = r.sdkProvider.Shutdown(ctx)
	})
	if r.shutdownErr != nil {
		logWarning(r.logger, "telemetry.shutdown_failed", r.shutdownErr)
	}
	return r.shutdownErr
}

func disabledRuntime(logger *slog.Logger) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}
	provider := trace.NewNoopTracerProvider()
	return &Runtime{
		provider:   provider,
		tracer:     provider.Tracer(instrumentationName),
		propagator: propagation.TraceContext{},
		logger:     logger,
	}
}

func exporterOptions(cfg Config) []otlptracehttp.Option {
	options := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(cfg.Endpoint),
		otlptracehttp.WithHeaders(cfg.Headers),
		otlptracehttp.WithTimeout(cfg.ExportTimeout),
		otlptracehttp.WithMaxRequestSize(maxExportRequestSize),
	}
	if cfg.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	return options
}

func loadConfigFromEnv() (Config, error) {
	exporterMode := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_EXPORTER")))
	disabledByEnv := parseBool(os.Getenv("OTEL_SDK_DISABLED")) ||
		parseBool(os.Getenv("OTEL_TELEMETRY_DISABLED"))
	endpoint := firstNonEmpty(
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	)

	cfg := Config{
		Enabled:  endpoint != "" && exporterMode != "none" && !disabledByEnv,
		Endpoint: endpoint,
		Insecure: parseBool(firstNonEmpty(
			os.Getenv("OTEL_EXPORTER_OTLP_TRACES_INSECURE"),
			os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"),
		)),
		ServiceName:    boundedConfigValue(firstNonEmpty(os.Getenv("OTEL_SERVICE_NAME"), defaultServiceName), defaultServiceName),
		ServiceVersion: boundedConfigValue(firstNonEmpty(os.Getenv("OTEL_SERVICE_VERSION"), os.Getenv("APP_VERSION")), ""),
		BatchTimeout:   parseMilliseconds(firstNonEmpty(os.Getenv("OTEL_BSP_SCHEDULE_DELAY")), defaultBatchTimeout, maxBatchTimeout),
		ExportTimeout: parseMilliseconds(firstNonEmpty(
			os.Getenv("OTEL_EXPORTER_OTLP_TRACES_TIMEOUT"),
			os.Getenv("OTEL_EXPORTER_OTLP_TIMEOUT"),
		), defaultExportTimeout, maxExportTimeout),
		QueueSize: parseBoundedInt(os.Getenv("OTEL_BSP_MAX_QUEUE_SIZE"), defaultQueueSize, 1, maxQueueSize),
		BatchSize: parseBoundedInt(os.Getenv("OTEL_BSP_MAX_EXPORT_BATCH_SIZE"), defaultBatchSize, 1, maxBatchSize),
	}

	if exporterMode != "" && exporterMode != "otlp" && exporterMode != "none" {
		return Config{}, fmt.Errorf("unsupported trace exporter mode %q", exporterMode)
	}
	if !cfg.Enabled {
		return cfg, nil
	}

	rawHeaders := firstNonEmpty(
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS"),
		os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"),
	)
	headers, err := parseHeaders(rawHeaders)
	if err != nil {
		return Config{}, err
	}
	cfg.Headers = headers
	return normalizeConfig(cfg)
}

func normalizeConfig(cfg Config) (Config, error) {
	if !cfg.Enabled {
		return cfg, nil
	}
	if cfg.Endpoint == "" {
		return Config{}, errors.New("telemetry endpoint is empty")
	}
	endpoint, err := normalizeEndpoint(cfg.Endpoint, cfg.Insecure)
	if err != nil {
		return Config{}, err
	}
	if err := validateHeaders(cfg.Headers); err != nil {
		return Config{}, err
	}
	cfg.Endpoint = endpoint
	if strings.HasPrefix(cfg.Endpoint, "http://") {
		cfg.Insecure = true
	}
	cfg.ServiceName = boundedConfigValue(cfg.ServiceName, defaultServiceName)
	cfg.ServiceVersion = boundedConfigValue(cfg.ServiceVersion, "")
	if cfg.BatchTimeout <= 0 || cfg.BatchTimeout > maxBatchTimeout {
		cfg.BatchTimeout = defaultBatchTimeout
	}
	if cfg.ExportTimeout <= 0 || cfg.ExportTimeout > maxExportTimeout {
		cfg.ExportTimeout = defaultExportTimeout
	}
	if cfg.QueueSize <= 0 || cfg.QueueSize > maxQueueSize {
		cfg.QueueSize = defaultQueueSize
	}
	if cfg.BatchSize <= 0 || cfg.BatchSize > maxBatchSize {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.BatchSize > cfg.QueueSize {
		cfg.BatchSize = cfg.QueueSize
	}
	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	}
	return cfg, nil
}

func normalizeEndpoint(value string, insecure bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("telemetry endpoint is empty")
	}
	if !strings.Contains(value, "://") {
		scheme := "https"
		if insecure {
			scheme = "http"
		}
		value = scheme + "://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("telemetry endpoint parse failed: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("telemetry endpoint must use http or https with a host")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("telemetry endpoint contains unsupported credentials or query data")
	}
	endpointPath := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(endpointPath, "/v1/traces") {
		if endpointPath == "" {
			endpointPath = "/v1/traces"
		} else {
			endpointPath += "/v1/traces"
		}
		parsed.Path = endpointPath
	}
	return parsed.String(), nil
}

func parseHeaders(raw string) (map[string]string, error) {
	if len(raw) > maxHeaderBytes {
		return nil, errors.New("telemetry headers exceed the configured size limit")
	}
	headers := make(map[string]string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return headers, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxHeaderCount {
		return nil, errors.New("telemetry headers exceed the configured count limit")
	}
	for _, part := range parts {
		keyValue := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(keyValue) != 2 {
			return nil, errors.New("telemetry header is missing an equals sign")
		}
		key := strings.TrimSpace(keyValue[0])
		value := strings.TrimSpace(keyValue[1])
		if !validHeaderKey(key) || len(key) > maxHeaderKeyLen || len(value) > maxHeaderValueLen {
			return nil, errors.New("telemetry header is invalid or too large")
		}
		if strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("telemetry header contains control characters")
		}
		headers[key] = value
	}
	return headers, nil
}

func validateHeaders(headers map[string]string) error {
	if len(headers) > maxHeaderCount {
		return errors.New("telemetry headers exceed the configured count limit")
	}
	for key, value := range headers {
		if !validHeaderKey(key) || len(key) > maxHeaderKeyLen || len(value) > maxHeaderValueLen ||
			strings.ContainsAny(value, "\r\n") {
			return errors.New("telemetry header is invalid or too large")
		}
	}
	return nil
}

func validHeaderKey(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("!#$%&'*+-.^_`|~", character) {
			continue
		}
		return false
	}
	return true
}

func parseMilliseconds(value string, fallback, maximum time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	milliseconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || milliseconds <= 0 {
		return fallback
	}
	duration := time.Duration(milliseconds) * time.Millisecond
	if duration <= 0 || duration > maximum {
		return fallback
	}
	return duration
}

func parseBoundedInt(value string, fallback, minimum, maximum int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func boundedConfigValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxValueLen || strings.ContainsAny(value, "\r\n\t") {
		return fallback
	}
	return value
}

func StartClientSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return StartSpan(ctx, name, trace.SpanKindClient)
}

func StartProducerSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return StartSpan(ctx, name, trace.SpanKindProducer)
}

func StartSpan(ctx context.Context, name string, kind trace.SpanKind) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	return otel.Tracer(instrumentationName).Start(ctx, safeOperation(name), trace.WithSpanKind(kind))
}

// InjectHTTP adds W3C trace context only when a valid local span exists. This
// keeps disabled telemetry from changing outgoing request headers.
func InjectHTTP(ctx context.Context, headers http.Header) {
	if ctx == nil || headers == nil || !trace.SpanContextFromContext(ctx).IsValid() {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
}

func SetHTTPClientRequest(span trace.Span, method, rawURL string) {
	if span == nil || !span.IsRecording() {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("http.request.method", boundedHTTPMethod(method)),
	}
	if parsed, err := url.Parse(rawURL); err == nil {
		if host := boundedConfigValue(parsed.Hostname(), ""); host != "" {
			attrs = append(attrs, attribute.String("server.address", host))
		}
	}
	span.SetAttributes(attrs...)
}

func SetHTTPClientResponse(ctx context.Context, status, bodyBytes int) {
	if ctx == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.IsRecording() {
		return
	}
	span.SetAttributes(
		attribute.Int("http.response.status_code", status),
		attribute.Int("http.response.body.size", bodyBytes),
	)
	if status >= http.StatusInternalServerError {
		span.SetStatus(codes.Error, "upstream request failed")
	}
}

func SetMessagingAttributes(ctx context.Context, system, operation, destination string) {
	if ctx == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.IsRecording() {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", safeOperation(system)),
		attribute.String("messaging.operation.type", safeOperation(operation)),
	}
	if destination = safeOperation(destination); destination != "unknown" {
		attrs = append(attrs, attribute.String("messaging.destination.name", destination))
	}
	span.SetAttributes(attrs...)
}

// RecordError records only a bounded Go error type and a redacted message
// marker. It intentionally never calls Span.RecordError, which would export
// the raw error string (often SQL or upstream response text).
func RecordError(ctx context.Context, err error, operation string) {
	if ctx == nil || err == nil {
		return
	}
	recordSanitizedException(trace.SpanFromContext(ctx), logging.ErrorType(err), operation)
}

func RecordPanic(ctx context.Context, panicType string) {
	if ctx == nil {
		return
	}
	recordSanitizedException(trace.SpanFromContext(ctx), panicType, "panic")
}

func recordSanitizedException(span trace.Span, exceptionType, operation string) {
	if span == nil || !span.IsRecording() {
		return
	}
	exceptionType = boundedConfigValue(exceptionType, "unknown")
	if exceptionType == "" {
		exceptionType = "unknown"
	}
	attrs := []attribute.KeyValue{
		attribute.String("exception.type", exceptionType),
		attribute.String("exception.message", logging.RedactedValue),
	}
	if operation = safeOperation(operation); operation != "unknown" {
		attrs = append(attrs, attribute.String("exception.operation", operation))
	}
	span.AddEvent("exception", trace.WithAttributes(attrs...))
	span.SetStatus(codes.Error, "operation failed")
}

func boundedHTTPMethod(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return strings.ToUpper(method)
	default:
		return "OTHER"
	}
}

func safeOperation(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxOperationLen {
		return "unknown"
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("._/-", character) {
			continue
		}
		return "unknown"
	}
	return value
}

func logWarning(logger *slog.Logger, event string, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	attrs := make([]slog.Attr, 0, 1)
	if err != nil {
		attrs = append(attrs, logging.ErrorAttr(err))
	}
	logger.LogAttrs(context.Background(), slog.LevelWarn, event, attrs...)
}

type resilientExporter struct {
	exporter sdktrace.SpanExporter
	logger   *slog.Logger
	limiter  errorLogLimiter
}

func (e *resilientExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) (err error) {
	defer func() {
		if recover() != nil {
			e.log("telemetry.exporter_panic", nil, len(spans))
			err = nil
		}
	}()
	if err := e.exporter.ExportSpans(ctx, spans); err != nil {
		e.log("telemetry.export_failed", err, len(spans))
	}
	// Telemetry is explicitly fail-open: a failed batch is dropped rather than
	// surfacing through the SDK error handler or request path.
	return nil
}

func (e *resilientExporter) Shutdown(ctx context.Context) (err error) {
	defer func() {
		if recover() != nil {
			e.log("telemetry.exporter_shutdown_panic", nil, 0)
			err = nil
		}
	}()
	if err := e.exporter.Shutdown(ctx); err != nil {
		e.log("telemetry.exporter_shutdown_failed", err, 0)
	}
	return nil
}

func (e *resilientExporter) log(event string, err error, spanCount int) {
	now := time.Now()
	if !e.limiter.permit(now) {
		return
	}
	logger := e.logger
	if logger == nil {
		logger = slog.Default()
	}
	attrs := make([]slog.Attr, 0, 2)
	if err != nil {
		attrs = append(attrs, logging.ErrorAttr(err))
	}
	if spanCount > 0 {
		attrs = append(attrs, slog.Int("span_count", spanCount))
	}
	logger.LogAttrs(context.Background(), slog.LevelWarn, event, attrs...)
}

type errorLogLimiter struct {
	mu   sync.Mutex
	next time.Time
}

func (l *errorLogLimiter) permit(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.next.IsZero() && now.Before(l.next) {
		return false
	}
	l.next = now.Add(30 * time.Second)
	return true
}
