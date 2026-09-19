package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"ifritah/web-service-gin/pkg/logging"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestPublishedEventOmitsMessageSecretsAndKeepsRequestContext(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(logging.New(logging.Config{Level: slog.LevelInfo, Format: "json"}, &output))
	t.Cleanup(func() { slog.SetDefault(previous) })

	ctx := logging.WithRequestID(context.Background(), "req-nats")
	ctx = logging.WithTrustedServerContext(ctx, logging.ServerContext{Tenant: "tenant-a"})
	logPublished(ctx, Message{
		DocType:  "onboard",
		BranchID: 7,
		DBName:   "tenant-secret",
		OTP:      "otp-secret",
	}, true)

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode published event: %v", err)
	}
	if record["msg"] != "zatca.message_published" ||
		record["request_id"] != "req-nats" ||
		record["document_type"] != "onboard" ||
		record["branch_id"] != float64(7) ||
		record["acknowledged"] != true {
		t.Fatalf("published event = %#v", record)
	}
	if strings.Contains(output.String(), "tenant-secret") ||
		strings.Contains(output.String(), "otp-secret") {
		t.Fatalf("NATS message secrets leaked into event: %s", output.String())
	}
}

type contextAwarePublisher struct {
	publish func(context.Context) (*nats.PubAck, error)
}

func (p contextAwarePublisher) Publish(_ string, _ []byte, options ...nats.PubOpt) (*nats.PubAck, error) {
	for _, option := range options {
		if contextOption, ok := option.(nats.ContextOpt); ok {
			return p.publish(contextOption.Context)
		}
	}
	return nil, errors.New("missing publish context")
}

func TestPublishPreservesParentCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	parent = logging.WithTrustedServerContext(parent, logging.ServerContext{Tenant: "tenant-a"})

	publisher := &ZatcaPublisher{
		js:             contextAwarePublisher{publish: func(ctx context.Context) (*nats.PubAck, error) { return nil, ctx.Err() }},
		dbName:         "tenant-test",
		publishTimeout: time.Second,
	}

	err := publisher.publish(parent, Message{DocType: "bill", ID: 1, BranchID: 2, DBName: "tenant-test"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("publish error = %v, want context canceled", err)
	}
}

func TestPublishUsesBoundedTimeout(t *testing.T) {
	const timeout = 20 * time.Millisecond
	publisher := &ZatcaPublisher{
		js: contextAwarePublisher{publish: func(ctx context.Context) (*nats.PubAck, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}},
		dbName:         "tenant-test",
		publishTimeout: timeout,
	}

	startedAt := time.Now()
	ctx := logging.WithTrustedServerContext(context.Background(), logging.ServerContext{Tenant: "tenant-test"})
	err := publisher.SubmitBillContext(ctx, 1, 2)
	elapsed := time.Since(startedAt)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("publish error = %v, want deadline exceeded", err)
	}
	if elapsed > time.Second {
		t.Fatalf("publish took %s; timeout was %s", elapsed, timeout)
	}
}

func TestPublishRequiresTrustedTenantContext(t *testing.T) {
	publisher := &ZatcaPublisher{
		js:             contextAwarePublisher{publish: func(context.Context) (*nats.PubAck, error) { return nil, nil }},
		dbName:         "tenant-test",
		publishTimeout: time.Second,
	}

	err := publisher.SubmitBill(1, 2)
	if !errors.Is(err, ErrMissingTrustedTenantContext) {
		t.Fatalf("publish error = %v, want missing trusted tenant context", err)
	}
}

func TestJobEnvelopeCarriesBoundedCorrelationContext(t *testing.T) {
	ctx := logging.WithRequestID(context.Background(), "req-nats")
	ctx = logging.WithTrustedServerContext(ctx, logging.ServerContext{Tenant: "tenant-a"})
	envelope, err := newJobEnvelope(ctx, Message{
		DocType:  "bill",
		ID:       7,
		BranchID: 3,
		DBName:   "tenant-a",
	})
	if err != nil {
		t.Fatalf("new job envelope: %v", err)
	}
	if envelope.TenantID != "tenant-a" ||
		envelope.OriginRequestID != "req-nats" ||
		envelope.Operation != "zatca.submit_bill" ||
		envelope.JobID == "" {
		t.Fatalf("job envelope = %#v", envelope)
	}
	if len(envelope.JobID) > maxJobEnvelopeValueLength {
		t.Fatalf("job ID is unbounded: %q", envelope.JobID)
	}
	if err := envelope.Validate(); err != nil {
		t.Fatalf("validate envelope: %v", err)
	}
}

func TestJobEnvelopeRejectsUnboundedOrInvalidCorrelation(t *testing.T) {
	tests := []JobEnvelope{
		{TenantID: strings.Repeat("t", maxJobEnvelopeValueLength+1), Operation: "zatca.submit_bill", JobID: "job"},
		{TenantID: "tenant-a", Operation: "zatca.submit_bill", JobID: "job", OriginRequestID: "bad request"},
		{TenantID: "tenant-a", Operation: "zatca.submit_bill", JobID: "job", OriginTraceID: "not-a-trace"},
		{TenantID: "tenant-a", Operation: "unknown", JobID: "job"},
		{TenantID: "tenant-a", Operation: "zatca.submit_bill", JobID: "job", OriginTraceID: strings.Repeat("0", 32)},
	}
	for _, test := range tests {
		if err := test.Validate(); err == nil {
			t.Fatalf("envelope %#v unexpectedly validated", test)
		}
	}
}

func TestPublishAddsProducerCorrelationLinkAndEnvelope(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})

	parentContext, parentSpan := provider.Tracer("nats-test").Start(context.Background(), "request")
	parentTraceID := trace.SpanContextFromContext(parentContext).TraceID()
	parentContext = logging.WithRequestID(parentContext, "req-nats-link")
	parentContext = logging.WithTrustedServerContext(parentContext, logging.ServerContext{Tenant: "tenant-a"})
	publisher := &recordingPublisher{}
	zatca := &ZatcaPublisher{
		js:             publisher,
		dbName:         "tenant-a",
		publishTimeout: time.Second,
	}

	if err := zatca.publish(parentContext, Message{
		DocType:  "bill",
		ID:       9,
		BranchID: 3,
		DBName:   "tenant-a",
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	parentSpan.End()
	if err := provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	var envelope JobEnvelope
	if err := json.Unmarshal(publisher.data, &envelope); err != nil {
		t.Fatalf("decode envelope: %v; payload=%s", err, publisher.data)
	}
	if envelope.TenantID != "tenant-a" ||
		envelope.OriginRequestID != "req-nats-link" ||
		envelope.OriginTraceID != parentTraceID.String() ||
		envelope.Operation != "zatca.submit_bill" ||
		envelope.JobID == "" {
		t.Fatalf("published envelope = %#v", envelope)
	}

	var producer tracetest.SpanStub
	found := false
	for _, span := range exporter.GetSpans() {
		if span.Name == "nats.publish" {
			producer = span
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("producer span missing: %#v", exporter.GetSpans())
	}
	if len(producer.Links) != 1 ||
		producer.Links[0].SpanContext.TraceID() != parentTraceID {
		t.Fatalf("producer links = %#v", producer.Links)
	}
	attributes := make(map[string]string)
	for _, attr := range producer.Attributes {
		attributes[string(attr.Key)] = attr.Value.AsString()
	}
	if attributes["ifritah.tenant_id"] != "tenant-a" ||
		attributes["ifritah.origin_request_id"] != "req-nats-link" ||
		attributes["ifritah.origin_trace_id"] != parentTraceID.String() ||
		attributes["ifritah.operation"] != "zatca.submit_bill" ||
		attributes["ifritah.job_id"] == "" {
		t.Fatalf("producer attributes = %#v", attributes)
	}
}

type recordingPublisher struct {
	data []byte
}

func (p *recordingPublisher) Publish(_ string, data []byte, _ ...nats.PubOpt) (*nats.PubAck, error) {
	p.data = append([]byte(nil), data...)
	return &nats.PubAck{}, nil
}

func TestPublishTimeoutFromEnvUsesSafeBounds(t *testing.T) {
	t.Setenv("NATS_PUBLISH_TIMEOUT", "25ms")
	if got := publishTimeoutFromEnv(); got != 25*time.Millisecond {
		t.Fatalf("configured timeout = %s, want 25ms", got)
	}

	t.Setenv("NATS_PUBLISH_TIMEOUT", "not-a-duration")
	if got := publishTimeoutFromEnv(); got != defaultNATSPublishTimeout {
		t.Fatalf("invalid timeout = %s, want default %s", got, defaultNATSPublishTimeout)
	}

	t.Setenv("NATS_PUBLISH_TIMEOUT", "2m")
	if got := publishTimeoutFromEnv(); got != defaultNATSPublishTimeout {
		t.Fatalf("unbounded timeout = %s, want default %s", got, defaultNATSPublishTimeout)
	}
}
