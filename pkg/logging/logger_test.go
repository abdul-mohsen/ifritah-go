package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestJSONLoggerSchemaAndRedaction(t *testing.T) {
	var output bytes.Buffer
	logger := New(Config{Level: slog.LevelInfo, Format: "json"}, &output)

	logger.Info("test.event",
		slog.String("request_id", "req-1"),
		slog.Int("status", 200),
		slog.String("password", "secret"),
		slog.String("note", "line\nbreak"))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log record: %v", err)
	}
	if record["msg"] != "test.event" {
		t.Fatalf("msg = %v, want test.event", record["msg"])
	}
	if record["request_id"] != "req-1" {
		t.Fatalf("request_id = %v, want req-1", record["request_id"])
	}
	if record["status"] != float64(200) {
		t.Fatalf("status = %v, want 200", record["status"])
	}
	if record["password"] != RedactedValue {
		t.Fatalf("password = %v, want redacted", record["password"])
	}
	if record["note"] != "line break" {
		t.Fatalf("note = %q, want control characters normalized", record["note"])
	}
}

func TestRedactionHelpers(t *testing.T) {
	for _, key := range []string{
		"token", "refresh_token", "authorization", "otp", "request_body",
		"response_body", "request_payload", "message_body", "raw_payload",
		"sql", "raw_data", "user_data", "password_reset",
	} {
		if got := RedactString(key, "secret-value"); got != RedactedValue {
			t.Fatalf("RedactString(%q) = %q, want redacted", key, got)
		}
	}
	for _, key := range []string{
		"request_id", "request_count", "metadata", "database",
		"message_id", "status_message", "data_center", "tenant",
	} {
		if IsSensitiveKey(key) {
			t.Fatalf("IsSensitiveKey(%q) = true; stable field was over-redacted", key)
		}
	}
	if got := RedactString("event", "safe\nvalue"); got != "safe value" {
		t.Fatalf("RedactString(event) = %q, want normalized value", got)
	}
}

func TestErrorAttrDoesNotExposeErrorText(t *testing.T) {
	var output bytes.Buffer
	logger := New(Config{Level: slog.LevelInfo, Format: "json"}, &output)
	err := errors.New("SELECT password FROM users WHERE token = 'secret'")

	logger.Error("test.error", ErrorAttr(err))
	if strings.Contains(output.String(), "SELECT password") ||
		strings.Contains(output.String(), "secret") {
		t.Fatalf("raw error text leaked into log: %s", output.String())
	}
}

func TestRequestIDValidationAndGeneration(t *testing.T) {
	for _, value := range []string{"abc-123", "req_1.2", "0123456789abcdef"} {
		if !ValidRequestID(value) {
			t.Fatalf("ValidRequestID(%q) = false", value)
		}
	}
	for _, value := range []string{"", "bad id", "bad\nid", strings.Repeat("a", 65)} {
		if ValidRequestID(value) {
			t.Fatalf("ValidRequestID(%q) = true", value)
		}
	}
	if generated := NewRequestID(); !ValidRequestID(generated) {
		t.Fatalf("generated request ID %q is invalid", generated)
	}
}

func TestTrustedContextIsolation(t *testing.T) {
	if _, ok := TrustedUserIDFromContext(context.Background()); ok {
		t.Fatal("plain context unexpectedly has a trusted user")
	}

	ctx := WithTrustedUserID(context.Background(), 42)
	userID, ok := TrustedUserIDFromContext(ctx)
	if !ok || userID != 42 {
		t.Fatalf("trusted user = %d, %v; want 42, true", userID, ok)
	}

	ctx = WithServerContext(ctx, ServerContext{Tenant: "tenant-a", CompanyID: "7"})
	serverContext, ok := ServerContextFromContext(ctx)
	if !ok || serverContext.Tenant != "tenant-a" || serverContext.CompanyID != "7" {
		t.Fatalf("server context = %#v, %v", serverContext, ok)
	}
}

func TestErrorTypeIsStableAndNonSensitive(t *testing.T) {
	if got := ErrorType(fmt.Errorf("password=%s", "secret")); !strings.Contains(got, "errorString") {
		t.Fatalf("ErrorType() = %q, want error type", got)
	}
	if got := ErrorType(context.Canceled); got != "context_canceled" {
		t.Fatalf("ErrorType(context.Canceled) = %q", got)
	}
}

func TestStructuredHelpersPreserveTrustedContextWithoutErrorText(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(New(Config{Level: slog.LevelInfo, Format: "json"}, &output))
	t.Cleanup(func() { slog.SetDefault(previous) })

	ctx := WithRequestID(context.Background(), "req-context")
	ctx = WithTrustedUserID(ctx, 42)
	ctx = WithServerContext(ctx, ServerContext{Tenant: "tenant-a", CompanyID: "7"})
	LogError(ctx, "test.context", errors.New("SQL password token secret"),
		slog.String("request_count", "3"),
		slog.String("metadata", "stable"))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log record: %v", err)
	}
	if record["request_id"] != "req-context" ||
		record["user_id"] != float64(42) ||
		record["tenant"] != "tenant-a" ||
		record["company_id"] != "7" {
		t.Fatalf("context attrs = %#v", record)
	}
	if record["request_count"] != "3" || record["metadata"] != "stable" {
		t.Fatalf("stable attrs were changed: %#v", record)
	}
	if strings.Contains(output.String(), "SQL password token secret") {
		t.Fatalf("error text leaked into structured log: %s", output.String())
	}
}

func TestLoggerAddsTraceAndSpanIDsFromContext(t *testing.T) {
	var output bytes.Buffer
	logger := New(Config{Level: slog.LevelInfo, Format: "json"}, &output)
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("trace ID: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("span ID: %v", err)
	}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}))

	logger.InfoContext(ctx, "test.trace")

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode trace record: %v", err)
	}
	if record["trace_id"] != traceID.String() || record["span_id"] != spanID.String() {
		t.Fatalf("trace fields = %#v", record)
	}
}

func TestCompatibilityBridgePreservesSafeOperationAndErrorType(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(New(Config{Level: slog.LevelInfo, Format: "json"}, &output))
	t.Cleanup(func() { slog.SetDefault(previous) })

	ctx := WithRequestID(context.Background(), "req-legacy")
	PrintfContext(ctx, "CreateThing: id=%d, err=%v", int64(7), errors.New("raw SQL password token"))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode compatibility log: %v", err)
	}
	if record["msg"] != "legacy.log" ||
		record["operation"] != "CreateThing" ||
		record["request_id"] != "req-legacy" ||
		record["arg_0"] != float64(7) {
		t.Fatalf("compatibility record = %#v", record)
	}
	errorGroup, ok := record["error"].(map[string]any)
	if !ok || errorGroup["type"] == nil {
		t.Fatalf("error type missing from compatibility record: %#v", record)
	}
	if strings.Contains(output.String(), "raw SQL password token") {
		t.Fatalf("compatibility bridge leaked error text: %s", output.String())
	}
}

func TestCompatibilityPrintfAssignsSeverityFromSafePrefix(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		level  string
	}{
		{
			name:   "successful operation is info",
			format: "CreateThing completed: id=%d",
			args:   []any{int64(7)},
			level:  "INFO",
		},
		{
			name:   "warning prefix is warn",
			format: "GetMe permissions query failed (non-fatal): %v",
			args:   []any{errors.New("safe error type only")},
			level:  "WARN",
		},
		{
			name:   "error argument is error",
			format: "CreateThing: %v",
			args:   []any{errors.New("safe error type only")},
			level:  "ERROR",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(New(Config{Level: slog.LevelInfo, Format: "json"}, &output))
			t.Cleanup(func() { slog.SetDefault(previous) })

			PrintfContext(context.Background(), test.format, test.args...)

			var record map[string]any
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatalf("decode compatibility log: %v", err)
			}
			if record["level"] != test.level {
				t.Fatalf("level = %v, want %s; record=%#v", record["level"], test.level, record)
			}
		})
	}
}
