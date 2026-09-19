package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

const (
	RequestIDHeader = "X-Request-ID"
	RedactedValue   = "[REDACTED]"
)

type Config struct {
	Level     slog.Level
	Format    string
	AddSource bool
}

type ServerContext struct {
	Tenant    string
	CompanyID string
}

type contextKey uint8

const (
	requestIDKey contextKey = iota
	trustedUserIDKey
	serverContextKey
)

func ConfigFromEnv() Config {
	return Config{
		Level:     ParseLevel(os.Getenv("LOG_LEVEL")),
		Format:    ParseFormat(os.Getenv("LOG_FORMAT")),
		AddSource: parseBool(os.Getenv("LOG_SOURCE")),
	}
}

func ParseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func ParseFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text":
		return "text"
	default:
		return "json"
	}
}

func New(cfg Config, writer io.Writer) *slog.Logger {
	if writer == nil {
		writer = os.Stderr
	}

	options := &slog.HandlerOptions{
		AddSource:   cfg.AddSource,
		Level:       cfg.Level,
		ReplaceAttr: RedactAttr,
	}
	if ParseFormat(cfg.Format) == "text" {
		return slog.New(newContextHandler(slog.NewTextHandler(writer, options)))
	}
	return slog.New(newContextHandler(slog.NewJSONHandler(writer, options)))
}

func NewFromEnv(writer io.Writer) *slog.Logger {
	return New(ConfigFromEnv(), writer)
}

func RedactAttr(groups []string, attr slog.Attr) slog.Attr {
	if IsSensitiveKey(attr.Key) {
		return slog.String(attr.Key, RedactedValue)
	}
	if attr.Value.Kind() == slog.KindString {
		return slog.String(attr.Key, sanitizeString(attr.Value.String()))
	}
	return attr
}

func IsSensitiveKey(key string) bool {
	key = normalizeKey(key)
	if key == "" || key == "requestid" {
		return false
	}

	switch key {
	case "password", "passwd", "passwordhash", "passphrase",
		"token", "resettoken", "accesstoken", "refreshtoken", "idtoken",
		"secret", "secretkey", "clientsecret",
		"authorization", "authheader", "cookie", "setcookie",
		"otp", "privatekey", "apikey", "authkey", "credential", "credentials",
		"request", "requestbody", "requestdata", "responsebody",
		"requestpayload", "responsepayload", "body", "payload", "rawpayload",
		"message", "messagebody", "rawmessage", "natsmessage",
		"query", "sql", "statement", "data", "rawdata", "userdata", "responsedata":
		return true
	}

	for _, suffix := range []string{
		"password", "passwd", "passwordhash", "passphrase",
		"token", "tokenhash", "secret", "secretkey",
		"authorization", "authheader", "cookie", "otp",
		"privatekey", "apikey", "authkey", "credential",
	} {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}

	if strings.HasPrefix(key, "authorization") || strings.HasPrefix(key, "cookie") {
		return true
	}
	if strings.HasPrefix(key, "password") || strings.HasPrefix(key, "passwd") ||
		strings.HasPrefix(key, "secret") {
		return true
	}

	return false
}

func RedactString(key, value string) string {
	if IsSensitiveKey(key) {
		return RedactedValue
	}
	return sanitizeString(value)
}

func ErrorAttr(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	return slog.Group("error", slog.String("type", ErrorType(err)))
}

func ErrorType(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context_deadline_exceeded"
	}

	errType := reflect.TypeOf(err)
	if errType == nil {
		return "unknown"
	}
	return errType.String()
}

func LogError(ctx context.Context, event string, err error, attrs ...slog.Attr) {
	if err != nil {
		attrs = append(attrs, ErrorAttr(err))
	}
	logAttrs(ctx, slog.LevelError, event, attrs...)
}

func LogWarn(ctx context.Context, event string, attrs ...slog.Attr) {
	logAttrs(ctx, slog.LevelWarn, event, attrs...)
}

func LogInfo(ctx context.Context, event string, attrs ...slog.Attr) {
	logAttrs(ctx, slog.LevelInfo, event, attrs...)
}

func ContextAttrs(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}

	attrs := make([]slog.Attr, 0, 4)
	if requestID := RequestIDFromContext(ctx); ValidRequestID(requestID) {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
		attrs = append(attrs,
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}
	if userID, ok := TrustedUserIDFromContext(ctx); ok {
		attrs = append(attrs, slog.Int64("user_id", userID))
	}
	if serverContext, ok := ServerContextFromContext(ctx); ok {
		if serverContext.Tenant != "" {
			attrs = append(attrs, slog.String("tenant", serverContext.Tenant))
			attrs = append(attrs, slog.String("tenant_id", serverContext.Tenant))
		}
		if serverContext.CompanyID != "" {
			attrs = append(attrs, slog.String("company_id", serverContext.CompanyID))
		}
	}
	return attrs
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}

func WithTrustedUserID(ctx context.Context, userID int64) context.Context {
	if userID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, trustedUserIDKey, userID)
}

func TrustedUserIDFromContext(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	userID, ok := ctx.Value(trustedUserIDKey).(int64)
	return userID, ok && userID > 0
}

func WithServerContext(ctx context.Context, serverContext ServerContext) context.Context {
	return context.WithValue(ctx, serverContextKey, normalizeServerContext(serverContext))
}

func WithCompanyID(ctx context.Context, companyID string) context.Context {
	serverContext, _ := ServerContextFromContext(ctx)
	serverContext.CompanyID = companyID
	return WithServerContext(ctx, serverContext)
}

func ServerContextFromContext(ctx context.Context) (ServerContext, bool) {
	if ctx == nil {
		return ServerContext{}, false
	}
	serverContext, ok := ctx.Value(serverContextKey).(ServerContext)
	return serverContext, ok
}

func ServerContextFromEnv() ServerContext {
	tenant := strings.TrimSpace(os.Getenv("TENANT_ID"))
	if tenant == "" {
		tenant = strings.TrimSpace(os.Getenv("DBNAME"))
	}
	return normalizeServerContext(ServerContext{
		Tenant:    tenant,
		CompanyID: strings.TrimSpace(os.Getenv("COMPANY_ID")),
	})
}

func NewRequestID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "req-" + hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return hex.EncodeToString(raw[:])
}

func ValidRequestID(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

// The old code used the standard log package throughout the handlers. These
// compatibility functions preserve a bounded operation name and error type,
// while never rendering arbitrary arguments.
func Printf(format string, args ...any) {
	PrintfContext(context.Background(), format, args...)
}

func PrintfContext(ctx context.Context, format string, args ...any) {
	legacyLog(ctx, legacyPrintfLevel(format, args...), format, args...)
}

func Print(args ...any) {
	PrintContext(context.Background(), args...)
}

func PrintContext(ctx context.Context, args ...any) {
	legacyLog(ctx, slog.LevelInfo, "", args...)
}

func Println(args ...any) {
	PrintlnContext(context.Background(), args...)
}

func PrintlnContext(ctx context.Context, args ...any) {
	legacyLog(ctx, slog.LevelInfo, "", args...)
}

func Fatalf(format string, args ...any) {
	legacyLog(context.Background(), slog.LevelError, format, args...)
	os.Exit(1)
}

func Fatal(args ...any) {
	legacyLog(context.Background(), slog.LevelError, "", args...)
	os.Exit(1)
}

func SetFlags(_ int) {}

func logAttrs(ctx context.Context, level slog.Level, event string, attrs ...slog.Attr) {
	if ctx == nil {
		ctx = context.Background()
	}
	attrs = appendContextAttrs(ctx, attrs)
	slog.Default().LogAttrs(ctx, level, event, attrs...)
}

func appendContextAttrs(ctx context.Context, attrs []slog.Attr) []slog.Attr {
	contextAttrs := ContextAttrs(ctx)
	if len(contextAttrs) == 0 {
		return attrs
	}

	existing := make(map[string]struct{}, len(attrs))
	for _, attr := range attrs {
		if attr.Key != "" {
			existing[attr.Key] = struct{}{}
		}
	}
	for _, attr := range contextAttrs {
		if _, ok := existing[attr.Key]; ok {
			continue
		}
		attrs = append(attrs, attr)
	}
	return attrs
}

func legacyLog(ctx context.Context, level slog.Level, format string, args ...any) {
	attrs := []slog.Attr{slog.String("source", "standard_log")}
	if operation := safeLegacyOperation(format, len(args)); operation != "" {
		attrs = append(attrs, slog.String("operation", operation))
	}
	for index, arg := range args {
		if err, ok := arg.(error); ok {
			attrs = append(attrs, ErrorAttr(err))
			continue
		}
		if attr, ok := safeLegacyScalar(index, arg); ok {
			attrs = append(attrs, attr)
		}
	}
	logAttrs(ctx, level, "legacy.log", attrs...)
}

func legacyPrintfLevel(format string, args ...any) slog.Level {
	prefix := strings.TrimSpace(format)
	if index := strings.IndexAny(prefix, "%:\r\n"); index >= 0 {
		prefix = prefix[:index]
	}
	if containsLegacySeverityWord(prefix, "warning", "warn") ||
		strings.Contains(strings.ReplaceAll(strings.ToLower(prefix), "-", ""), "nonfatal") {
		return slog.LevelWarn
	}
	if containsLegacySeverityWord(prefix, "error", "err", "failed", "failure", "fatal", "panic") ||
		legacyArgsContainError(args) {
		return slog.LevelError
	}
	return slog.LevelInfo
}

func containsLegacySeverityWord(value string, words ...string) bool {
	wordsByName := make(map[string]struct{}, len(words))
	for _, word := range words {
		wordsByName[word] = struct{}{}
	}
	for _, word := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		if _, ok := wordsByName[word]; ok {
			return true
		}
	}
	return false
}

func legacyArgsContainError(args []any) bool {
	for _, arg := range args {
		if _, ok := arg.(error); ok {
			return true
		}
	}
	return false
}

func safeLegacyScalar(index int, value any) (slog.Attr, bool) {
	key := "arg_" + strconv.Itoa(index)
	switch value := value.(type) {
	case int:
		return slog.Int(key, value), true
	case int8:
		return slog.Int64(key, int64(value)), true
	case int16:
		return slog.Int64(key, int64(value)), true
	case int32:
		return slog.Int64(key, int64(value)), true
	case int64:
		return slog.Int64(key, value), true
	case uint:
		return slog.Uint64(key, uint64(value)), true
	case uint8:
		return slog.Uint64(key, uint64(value)), true
	case uint16:
		return slog.Uint64(key, uint64(value)), true
	case uint32:
		return slog.Uint64(key, uint64(value)), true
	case uint64:
		return slog.Uint64(key, value), true
	case bool:
		return slog.Bool(key, value), true
	default:
		return slog.Attr{}, false
	}
}

func safeLegacyOperation(format string, argCount int) string {
	format = strings.TrimSpace(format)
	if format == "" {
		return ""
	}

	prefix := format
	if index := strings.IndexAny(prefix, "%:\r\n"); index >= 0 {
		prefix = prefix[:index]
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}

	hasFormatArgs := strings.Contains(format, "%")
	if !hasFormatArgs && argCount > 0 {
		return ""
	}
	if containsSensitiveOperationWord(prefix) {
		return ""
	}

	fields := strings.Fields(prefix)
	if len(fields) == 0 || len(fields) > 3 {
		return ""
	}
	for i, field := range fields {
		field = strings.Trim(field, "[]()")
		if field == "" || !safeOperationField(field) {
			return ""
		}
		fields[i] = field
	}
	return strings.Join(fields, "_")
}

func containsSensitiveOperationWord(value string) bool {
	for _, word := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		switch word {
		case "select", "insert", "update", "delete", "password", "token",
			"secret", "authorization", "cookie", "otp", "body", "payload",
			"query", "sql", "data", "message":
			return true
		}
	}
	return false
}

func safeOperationField(value string) bool {
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func normalizeKey(key string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "", " ", "").Replace(key))
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func sanitizeString(value string) string {
	return strings.Map(func(character rune) rune {
		switch character {
		case '\r', '\n', '\t':
			return ' '
		default:
			return character
		}
	}, value)
}

func normalizeServerContext(serverContext ServerContext) ServerContext {
	serverContext.Tenant = normalizeContextValue(serverContext.Tenant)
	serverContext.CompanyID = normalizeContextValue(serverContext.CompanyID)
	return serverContext
}

func normalizeContextValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		return ""
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return ""
		}
	}
	return value
}

type contextHandler struct {
	slog.Handler
}

func newContextHandler(handler slog.Handler) slog.Handler {
	return &contextHandler{Handler: handler}
}

func (h *contextHandler) Handle(ctx context.Context, record slog.Record) error {
	existing := make(map[string]struct{})
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key != "" {
			existing[attr.Key] = struct{}{}
		}
		return true
	})
	for _, attr := range ContextAttrs(ctx) {
		if _, ok := existing[attr.Key]; ok {
			continue
		}
		record.AddAttrs(attr)
	}
	return h.Handler.Handle(ctx, record)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return newContextHandler(h.Handler.WithAttrs(attrs))
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return newContextHandler(h.Handler.WithGroup(name))
}
