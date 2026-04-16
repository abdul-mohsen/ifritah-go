package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type Message struct {
	DocType  string `json:"doc_type"`
	ID       int64  `json:"id"`
	BranchID int64  `json:"branch_id"`
	DBName   string `json:"db_name"`
	OTP      string `json:"otp,omitempty"`
}

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
	p.once.Do(func() { p.nc.Close() })
}

func (p *ZatcaPublisher) SubmitBill(billID, branchID int64) error {
	return p.publish(Message{DocType: "bill", ID: billID, BranchID: branchID, DBName: p.dbName})
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

func (p *ZatcaPublisher) publish(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	subject := fmt.Sprint("zatca.%s.%s.%d", msg.DocType, msg.DBName, msg.BranchID)
	ack, err := p.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("publish to %s: %w", subject, err)
	}
	log.Print(ack)
	log.Print(subject)
	log.Print(data)
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
