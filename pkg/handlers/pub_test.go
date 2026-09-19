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
)

func TestPublishedEventOmitsMessageSecretsAndKeepsRequestContext(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(logging.New(logging.Config{Level: slog.LevelInfo, Format: "json"}, &output))
	t.Cleanup(func() { slog.SetDefault(previous) })

	ctx := logging.WithRequestID(context.Background(), "req-nats")
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
	err := publisher.SubmitBill(1, 2)
	elapsed := time.Since(startedAt)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("publish error = %v, want deadline exceeded", err)
	}
	if elapsed > time.Second {
		t.Fatalf("publish took %s; timeout was %s", elapsed, timeout)
	}
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
