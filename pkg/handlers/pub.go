package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type ZatcaPublisher struct {
	js     nats.JetStreamContext
	nc     *nats.Conn
	dbName string
	once   sync.Once
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
				log.Printf("[zatca-publisher] nats disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[zatca-publisher] NATS reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			log.Printf("[zatca-publisher] NATS  connection closed")
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

	return &ZatcaPublisher{js: js, nc: nc, dbName: dbName}, nil

}

func (p *ZatcaPublisher) Close() {
	if p == nil {
		return
	}
	p.once.Do(func() {
		if p.nc != nil {
			p.nc.Close()
		}
	})
}

func (p *ZatcaPublisher) SubmitBill(billID, branchID int64) error {
	return p.publish(NewMessage(DocBill, p.dbName, branchID, billID, ""))
}

func (p *ZatcaPublisher) SubmitCredit(creditNotID, branchID int64) error {
	return p.publish(NewMessage(DocCredit, p.dbName, branchID, creditNotID, ""))
}

func (p *ZatcaPublisher) SubmitDebit(debitNoteID, branchID int64) error {
	return p.publish(NewMessage(DocDebit, p.dbName, branchID, debitNoteID, ""))
}

func (p *ZatcaPublisher) OnboardBranch(branchID int64, otp string) error {
	otp = strings.TrimSpace(otp)
	return p.publish(NewMessage(
		DocOnboard,
		p.dbName,
		branchID,
		branchID,
		OnboardingIdempotencyKey(p.dbName, branchID, otp),
	).withOTP(otp))
}

func (p *ZatcaPublisher) OnboadBranch(branchID int64, otp string) error {
	return p.OnboardBranch(branchID, otp)
}

func (p *ZatcaPublisher) publish(msg Message) error {
	return p.publishMessage(msg)
}

func (p *ZatcaPublisher) publishMessage(msg Message) error {
	if p == nil || p.js == nil {
		return fmt.Errorf("publish ZATCA message: NATS JetStream is unavailable")
	}

	normalized, err := msg.Normalize()
	if err != nil {
		return fmt.Errorf("validate ZATCA message: %w", err)
	}
	if err := normalized.Validate(); err != nil {
		return fmt.Errorf("validate ZATCA message: %w", err)
	}

	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	subject := normalized.Subject()
	messageID := normalized.NATSMessageID()
	if messageID == "" {
		return fmt.Errorf("validate ZATCA message: stable document key is required")
	}
	ack, err := p.js.Publish(subject, data, nats.MsgId(messageID))
	if err != nil {
		return fmt.Errorf("publish to %s: %w", subject, err)
	}
	log.Print(ack)
	log.Print(sanitizeForLog(subject))
	log.Printf(
		"[zatca-publisher] published contract=%s version=%s event_id=%s idempotency_key=%s tenant_id=%s branch_id=%d document_type=%s source_id=%d outbox_id=%d attempt=%d",
		normalized.Contract,
		normalized.Version,
		normalized.EventID,
		normalized.StableDocumentKey,
		normalized.TenantID,
		normalized.BranchID,
		normalized.DocumentType,
		normalized.SourceID,
		normalized.OutboxID,
		normalized.Attempt,
	)
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
