package handlers

import (
	"encoding/json"
	"testing"
)

// The envelope a bill sends is the contract the daemon reads. It has these fields, and no tenant_id.
func TestBillEventCarriesTheCanonicalEnvelope(t *testing.T) {
	p := &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"}
	ident := billIdentity{
		PosID:         "pos-1",
		BillRequestID: "3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c",
		InvoiceUUID:   "7c9e6679-7425-40de-944b-e07fc1f90ae7",
	}
	event, err := p.BillEvent(42, 7, ident)
	if err != nil {
		t.Fatal(err)
	}
	if event.DocumentKey != "acme:7:bill:42" || event.Subject != "zatca.bill.acme_db.7" {
		t.Fatalf("event key %q and subject %q; want acme:7:bill:42 and zatca.bill.acme_db.7", event.DocumentKey, event.Subject)
	}

	var envelope map[string]any
	if err := json.Unmarshal(event.Payload, &envelope); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"doc_type":        "bill",
		"id":              float64(42),
		"branch_id":       float64(7),
		"db_name":         "acme_db",
		"tenant_key":      "acme",
		"pos_id":          ident.PosID,
		"bill_request_id": ident.BillRequestID,
		"event_id":        event.EventID,
		"egs_sequence_id": "7",
		"invoice_uuid":    ident.InvoiceUUID,
	}
	if len(envelope) != len(want) {
		t.Fatalf("envelope has %d fields %v; want %d", len(envelope), envelope, len(want))
	}
	for field, value := range want {
		if envelope[field] != value {
			t.Errorf("envelope %s = %v; want %v", field, envelope[field], value)
		}
	}
	if len(event.EventID) != 36 {
		t.Errorf("event id %q is not a UUID", event.EventID)
	}
}

// The daemon can start and the outbox can keep working while NATS is down. NATS is not contacted at startup.
func TestNewZATCAPublisherStartsWhileNATSIsDown(t *testing.T) {
	t.Setenv("DBNAME", "acme_db")
	t.Setenv("NATS_URL", "nats://127.0.0.1:1")

	p, err := NewZATCAPublisher("acme")
	if err != nil {
		t.Fatalf("NewZATCAPublisher with NATS down = %v; want the backend to start", err)
	}
	p.Close()
}
