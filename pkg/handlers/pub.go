package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"ifritah/web-service-gin/pkg/logging"
	"ifritah/web-service-gin/pkg/telemetry"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	defaultNATSPublishTimeout = 5 * time.Second
	maxNATSPublishTimeout     = time.Minute
)

type Message struct {
	DocType  string `json:"doc_type"`
	ID       int64  `json:"id"`
	BranchID int64  `json:"branch_id"`
	DBName   string `json:"db_name"`
	OTP      string `json:"otp,omitempty"`
}

type ZatcaPublisher struct {
	js             jetStreamPublisher
	nc             *nats.Conn
	dbName         string
	publishTimeout time.Duration
	once           sync.Once
}

type jetStreamPublisher interface {
	Publish(subject string, data []byte, options ...nats.PubOpt) (*nats.PubAck, error)
}

func NewZATCAPublisher() (*ZatcaPublisher, error) {
	natsURL := env("NATS_URL", "nats://localhost:4222")
	dbName := env("DBNAME", "")

	if dbName == "" {
		return nil, fmt.Errorf("DBNAME env var is required")
	}

	nc, err := nats.Connect(natsURL,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(10*time.Second),
		nats.ReconnectBufSize(8*1024*1024),
		nats.Timeout(5*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				logging.LogError(context.Background(), "nats.disconnected", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logging.LogInfo(context.Background(), "nats.reconnected",
				slog.Bool("connected", nc.IsConnected()))
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logging.LogInfo(context.Background(), "nats.closed")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}

	return &ZatcaPublisher{
		js:             js,
		nc:             nc,
		dbName:         dbName,
		publishTimeout: publishTimeoutFromEnv(),
	}, nil

}

func (p *ZatcaPublisher) Close() {
	p.once.Do(func() { p.nc.Close() })
}

func (p *ZatcaPublisher) SubmitBill(billID, branchID int64) error {
	return p.SubmitBillContext(context.Background(), billID, branchID)
}

func (p *ZatcaPublisher) SubmitBillContext(ctx context.Context, billID, branchID int64) error {
	return p.publish(ctx, Message{DocType: "bill", ID: billID, BranchID: branchID, DBName: p.dbName})
}

func (p *ZatcaPublisher) SubmitCredit(creditNotID, branchID int64) error {
	return p.SubmitCreditContext(context.Background(), creditNotID, branchID)
}

func (p *ZatcaPublisher) SubmitCreditContext(ctx context.Context, creditNotID, branchID int64) error {
	return p.publish(ctx, Message{DocType: "credit", ID: creditNotID, BranchID: branchID, DBName: p.dbName})
}

func (p *ZatcaPublisher) SubmitDebit(debitNoteID, branchID int64) error {
	return p.SubmitDebitContext(context.Background(), debitNoteID, branchID)
}

func (p *ZatcaPublisher) SubmitDebitContext(ctx context.Context, debitNoteID, branchID int64) error {
	return p.publish(ctx, Message{DocType: "debit", ID: debitNoteID, BranchID: branchID, DBName: p.dbName})
}

func (p *ZatcaPublisher) OnboadBranch(branchID int64, otp string) error {
	return p.OnboadBranchContext(context.Background(), branchID, otp)
}

func (p *ZatcaPublisher) OnboadBranchContext(ctx context.Context, branchID int64, otp string) error {
	return p.publish(ctx, Message{DocType: "onboard", ID: branchID, BranchID: branchID, DBName: p.dbName, OTP: otp})
}

func (p *ZatcaPublisher) publish(ctx context.Context, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	subject := fmt.Sprintf("zatca.%s.%s.%d", msg.DocType, msg.DBName, msg.BranchID)
	spanContext, span := telemetry.StartProducerSpan(ctx, "nats.publish")
	defer span.End()
	telemetry.SetMessagingAttributes(spanContext, "nats", "publish", msg.DocType)

	publishContext, cancel := boundedPublishContext(spanContext, p.publishTimeout)
	defer cancel()

	ack, err := p.js.Publish(subject, data, nats.Context(publishContext))
	if err != nil {
		telemetry.RecordError(publishContext, err, "nats.publish")
		return fmt.Errorf("nats publish: %w", err)
	}
	logPublished(spanContext, msg, ack != nil)
	return nil
}

func boundedPublishContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 || timeout > maxNATSPublishTimeout {
		timeout = defaultNATSPublishTimeout
	}
	return context.WithTimeout(parent, timeout)
}

func publishTimeoutFromEnv() time.Duration {
	value := strings.TrimSpace(os.Getenv("NATS_PUBLISH_TIMEOUT"))
	if value == "" {
		return defaultNATSPublishTimeout
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 || timeout > maxNATSPublishTimeout {
		return defaultNATSPublishTimeout
	}
	return timeout
}

func logPublished(ctx context.Context, msg Message, acknowledged bool) {
	logging.LogInfo(ctx, "zatca.message_published",
		slog.String("document_type", msg.DocType),
		slog.Int64("branch_id", msg.BranchID),
		slog.Bool("acknowledged", acknowledged))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
