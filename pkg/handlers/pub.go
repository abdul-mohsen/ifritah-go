package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/outbox"
)

// The outbox drainer publishes through ZatcaPublisher.
var _ outbox.Publisher = (*ZatcaPublisher)(nil)

// Message is the envelope the ZATCA daemon reads. The backend sets every field of a bill message. The
// daemon never generates tenant, request, event, EGS or invoice identity. An onboarding message carries
// only the first group of fields and the OTP.
type Message struct {
	DocType       string `json:"doc_type"`
	ID            int64  `json:"id"`
	BranchID      int64  `json:"branch_id"`
	DBName        string `json:"db_name"`
	OTP           string `json:"otp,omitempty"`
	TenantKey     string `json:"tenant_key,omitempty"`
	PosID         string `json:"pos_id,omitempty"`
	BillRequestID string `json:"bill_request_id,omitempty"`
	EventID       string `json:"event_id,omitempty"`
	EGSSequenceID string `json:"egs_sequence_id,omitempty"`
	InvoiceUUID   string `json:"invoice_uuid,omitempty"`
}

type ZatcaPublisher struct {
	js        nats.JetStreamContext
	nc        *nats.Conn
	dbName    string
	tenantKey string
	once      sync.Once
}

// NewZATCAPublisher connects to NATS for the tenant. DBNAME names the tenant database in every subject, and
// tenantKey is company.tenant_key, as LoadTenantKey reads it. A NATS server that is down at startup does not
// stop the backend. The connection keeps retrying, and the outbox keeps the events until they are published.
func NewZATCAPublisher(tenantKey string) (*ZatcaPublisher, error) {
	natsURL := env("NATS_URL", "nats://localhost:4222")
	dbName := env("DBNAME", "")

	if dbName == "" {
		return nil, fmt.Errorf("DBNAME env var is required")
	}

	nc, err := nats.Connect(natsURL,
		nats.RetryOnFailedConnect(true),
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
			log.Printf("[zatca-publisher] NATS connection closed")
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

// BillEvent returns the outbox event that asks ZATCA to submit a final sale. The sale writes the event in its
// own transaction, and the drainer publishes it after the commit. The event ID is created here, once, and
// every retry publishes with the same ID.
func (p *ZatcaPublisher) BillEvent(billID uint64, branchID uint32, ident billIdentity) (outbox.Event, error) {
	if billID > math.MaxInt64 {
		return outbox.Event{}, fmt.Errorf("bill id %d is above the envelope limit %d", billID, math.MaxInt64)
	}
	eventID, err := identity.NewUUID()
	if err != nil {
		return outbox.Event{}, err
	}
	documentKey, err := identity.DocumentKey(p.tenantKey, uint64(branchID), identity.DocBill, billID)
	if err != nil {
		return outbox.Event{}, err
	}
	payload, err := json.Marshal(Message{
		DocType:       identity.DocBill,
		ID:            int64(billID),
		BranchID:      int64(branchID),
		DBName:        p.dbName,
		TenantKey:     p.tenantKey,
		PosID:         ident.PosID,
		BillRequestID: ident.BillRequestID,
		EventID:       eventID,
		EGSSequenceID: strconv.FormatUint(uint64(branchID), 10),
		InvoiceUUID:   ident.InvoiceUUID,
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

// OnboadBranch asks the daemon to onboard a branch with the OTP from the Fatoora portal. Onboarding is a control
// message, not part of a sale, so it is published directly. The OTP is not stored in the outbox.
func (p *ZatcaPublisher) OnboadBranch(branchID int64, otp string) error {
	data, err := json.Marshal(Message{
		DocType:   "onboard",
		ID:        branchID,
		BranchID:  branchID,
		DBName:    p.dbName,
		TenantKey: p.tenantKey,
		OTP:       otp,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	subject := fmt.Sprintf("zatca.onboard.%s.%d", p.dbName, branchID)
	if _, err := p.js.Publish(subject, data); err != nil {
		return fmt.Errorf("publish to %s: %w", subject, err)
	}
	log.Printf("[zatca-publisher] published onboarding for branch %d", branchID)
	return nil
}

// Publish sends one outbox event. msgID is the event ID, so JetStream drops a repeat of the same event inside
// its duplicate window. Consumers also deduplicate by it.
func (p *ZatcaPublisher) Publish(subject string, data []byte, msgID string) error {
	if _, err := p.js.Publish(subject, data, nats.MsgId(msgID)); err != nil {
		return fmt.Errorf("publish to %s: %w", subject, err)
	}
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
