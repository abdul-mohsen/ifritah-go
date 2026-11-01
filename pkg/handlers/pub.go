package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/outbox"
)

// The outbox drainer publishes through ZatcaPublisher.
var _ outbox.Publisher = (*ZatcaPublisher)(nil)

type Message struct {
	DocType  string `json:"doc_type"`
	ID       int64  `json:"id"`
	BranchID int64  `json:"branch_id"`
	DBName   string `json:"db_name"`
	OTP      string `json:"otp,omitempty"`
}

type ZatcaPublisher struct {
	js        nats.JetStreamContext
	nc        *nats.Conn
	dbName    string
	tenantKey string
	once      sync.Once
}

// NewZATCAPublisher connects to NATS. DBNAME names the tenant database in every subject. TENANT_KEY
// is the tenant's stable key, the value of zatca_master.tenant.tenant_key, and it is required.
func NewZATCAPublisher() (*ZatcaPublisher, error) {
	natsURL := env("NATS_URL", "nats://localhost:4222")
	dbName := env("DBNAME", "")

	if dbName == "" {
		return nil, fmt.Errorf("DBNAME env var is required")
	}
	tenantKey := env("TENANT_KEY", "")
	if err := identity.ValidateTenantKey(tenantKey); err != nil {
		return nil, fmt.Errorf("TENANT_KEY env var: %w", err)
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

	return &ZatcaPublisher{js: js, nc: nc, dbName: dbName, tenantKey: tenantKey}, nil

}

func (p *ZatcaPublisher) Close() {
	p.once.Do(func() { p.nc.Close() })
}

// BillEvent returns the outbox event that asks ZATCA to submit a bill. A sale writes the event in
// its own transaction, and the drainer publishes it after that transaction commits. The subject and
// the message are the ones the daemon has always read.
func (p *ZatcaPublisher) BillEvent(billID uint64, branchID uint32) (outbox.Event, error) {
	eventID, err := outbox.NewEventID()
	if err != nil {
		return outbox.Event{}, err
	}
	documentKey, err := identity.DocumentKey(p.tenantKey, uint64(branchID), identity.DocBill, billID)
	if err != nil {
		return outbox.Event{}, err
	}
	payload, err := json.Marshal(Message{
		DocType:  identity.DocBill,
		ID:       int64(billID),
		BranchID: int64(branchID),
		DBName:   p.dbName,
	})
	if err != nil {
		return outbox.Event{}, fmt.Errorf("marshal: %w", err)
	}
	return outbox.Event{
		EventID:     eventID,
		DocumentKey: documentKey,
		Subject:     fmt.Sprintf("zatca.%s.%s.%d", identity.DocBill, p.dbName, branchID),
		Payload:     payload,
	}, nil
}

func (p *ZatcaPublisher) SubmitCredit(creditNotID, branchID int64) error {
	return p.publish(Message{DocType: "credit", ID: creditNotID, BranchID: branchID, DBName: p.dbName})
}

func (p *ZatcaPublisher) SubmitDebit(debitNoteID, branchID int64) error {
	return p.publish(Message{DocType: "debit", ID: debitNoteID, BranchID: branchID, DBName: p.dbName})
}

func (p *ZatcaPublisher) OnboadBranch(branchID int64, otp string) error {
	return p.publish(Message{DocType: "onboard", ID: branchID, BranchID: branchID, DBName: p.dbName, OTP: otp})
}

// Publish sends a raw message. msgID is the event ID, so JetStream drops a repeat of the same
// event inside its duplicate window.
func (p *ZatcaPublisher) Publish(subject string, data []byte, msgID string) error {
	if _, err := p.js.Publish(subject, data, nats.MsgId(msgID)); err != nil {
		return fmt.Errorf("publish to %s: %w", subject, err)
	}
	return nil
}

func (p *ZatcaPublisher) publish(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	subject := fmt.Sprintf("zatca.%s.%s.%d", msg.DocType, msg.DBName, msg.BranchID)
	if _, err := p.js.Publish(subject, data); err != nil {
		return fmt.Errorf("publish to %s: %w", subject, err)
	}
	log.Printf("[zatca-publisher] published subject=%s type=%s id=%d branch_id=%d",
		sanitizeForLog(subject), sanitizeForLog(msg.DocType), msg.ID, msg.BranchID)
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
