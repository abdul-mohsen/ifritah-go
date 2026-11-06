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

// onboardTimeout bounds the wait for the daemon's answer to an onboarding request. The daemon answers at once, and the
// onboarding itself runs after the answer.
const onboardTimeout = 10 * time.Second

// Message is the envelope the ZATCA daemon reads for one final sale. The backend sets every field. The daemon never generates
// tenant, request, event, EGS, or invoice identity.
type Message struct {
	DocType            string `json:"doc_type"`
	ID                 int64  `json:"id"`
	BranchID           int64  `json:"branch_id"`
	DBName             string `json:"db_name"`
	TenantKey          string `json:"tenant_key"`
	PosID              string `json:"pos_id"`
	BillRequestID      string `json:"bill_request_id"`
	EventID            string `json:"event_id"`
	EGSSequenceID      string `json:"egs_sequence_id"`
	InvoiceUUID        string `json:"invoice_uuid"`
	RequestFingerprint string `json:"request_fingerprint"`
}

// OnboardRequest asks the daemon to onboard a branch with the OTP from the Fatoora portal. It is a request and not an outbox
// event, because the OTP is a one-time secret: it is never written to MySQL, and the daemon keeps it in memory only (D18).
type OnboardRequest struct {
	DBName    string `json:"db_name"`
	TenantKey string `json:"tenant_key"`
	BranchID  int64  `json:"branch_id"`
	OTP       string `json:"otp"`
}

// OnboardReply is the daemon's answer to an OnboardRequest. It never repeats the OTP.
type OnboardReply struct {
	Accepted bool   `json:"accepted"`
	Error    string `json:"error,omitempty"`
}

// OnboardRejectedError means the daemon refused an onboarding request, with the reason it gave.
type OnboardRejectedError struct {
	Reason string
}

func (e *OnboardRejectedError) Error() string {
	return "onboarding refused: " + e.Reason
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
		DocType:            identity.DocBill,
		ID:                 int64(billID),
		BranchID:           int64(branchID),
		DBName:             p.dbName,
		TenantKey:          p.tenantKey,
		PosID:              ident.PosID,
		BillRequestID:      ident.BillRequestID,
		EventID:            eventID,
		EGSSequenceID:      strconv.FormatUint(uint64(branchID), 10),
		InvoiceUUID:        ident.InvoiceUUID,
		RequestFingerprint: ident.Fingerprint,
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

// OnboardBranch asks the daemon to onboard a branch with the OTP from the Fatoora portal. The daemon checks the request and
// answers at once, and the onboarding itself runs after the answer. A refusal is an *OnboardRejectedError. A daemon that does
// not answer is nats.ErrNoResponders or nats.ErrTimeout.
func (p *ZatcaPublisher) OnboardBranch(branchID int64, otp string) error {
	data, err := json.Marshal(OnboardRequest{DBName: p.dbName, TenantKey: p.tenantKey, BranchID: branchID, OTP: otp})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	subject := onboardSubject(p.dbName, branchID)
	reply, err := p.nc.Request(subject, data, onboardTimeout)
	if err != nil {
		return fmt.Errorf("onboarding request %s: %w", subject, err)
	}
	if err := parseOnboardReply(reply.Data); err != nil {
		return err
	}
	log.Printf("[zatca-publisher] onboarding of branch %d accepted", branchID)
	return nil
}

// onboardSubject is the subject of one branch's onboarding request. It sits outside the ZATCA stream (zatca.>), so JetStream
// never stores the request.
func onboardSubject(dbName string, branchID int64) string {
	return fmt.Sprintf("control.zatca.onboard.%s.%d", dbName, branchID)
}

// parseOnboardReply reads the daemon's answer. A refusal is an *OnboardRejectedError.
func parseOnboardReply(data []byte) error {
	var reply OnboardReply
	if err := json.Unmarshal(data, &reply); err != nil {
		return fmt.Errorf("read onboarding reply: %w", err)
	}
	if !reply.Accepted {
		return &OnboardRejectedError{Reason: reply.Error}
	}
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
