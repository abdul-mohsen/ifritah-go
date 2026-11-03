package handlers

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"

	"ifritah/web-service-gin/pkg/identity"
)

// The envelope a document sends is the contract the daemon reads. It has these fields, and no tenant_id. A bill, a credit note and a debit
// note send the same fields, and differ in doc_type, in the id, and in the key and subject built from them.
func TestDocumentEventCarriesTheCanonicalEnvelope(t *testing.T) {
	cases := map[string]struct {
		docType, key, subject string
	}{
		"bill":   {identity.DocBill, "acme:7:bill:42", "zatca.bill.acme_db.7"},
		"credit": {identity.DocCredit, "acme:7:credit:42", "zatca.credit.acme_db.7"},
		"debit":  {identity.DocDebit, "acme:7:debit:42", "zatca.debit.acme_db.7"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"}
			ident := billIdentity{
				PosID:         "pos-1",
				BillRequestID: "3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c",
				InvoiceUUID:   "7c9e6679-7425-40de-944b-e07fc1f90ae7",
				Fingerprint:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			}
			event, err := p.DocumentEvent(tc.docType, 42, 7, ident)
			if err != nil {
				t.Fatal(err)
			}
			if event.DocumentKey != tc.key || event.Subject != tc.subject {
				t.Fatalf("event key %q and subject %q; want %q and %q", event.DocumentKey, event.Subject, tc.key, tc.subject)
			}

			var envelope map[string]any
			if err := json.Unmarshal(event.Payload, &envelope); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{
				"doc_type":            tc.docType,
				"id":                  float64(42),
				"branch_id":           float64(7),
				"db_name":             "acme_db",
				"tenant_key":          "acme",
				"pos_id":              ident.PosID,
				"bill_request_id":     ident.BillRequestID,
				"event_id":            event.EventID,
				"egs_sequence_id":     "7",
				"invoice_uuid":        ident.InvoiceUUID,
				"request_fingerprint": ident.Fingerprint,
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
		})
	}
}

// The envelope's id is a signed 64-bit integer, so a document id above that limit cannot be sent.
func TestDocumentEventRefusesAnIDTheEnvelopeCannotCarry(t *testing.T) {
	p := &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"}
	if _, err := p.DocumentEvent(identity.DocCredit, math.MaxInt64+1, 7, billIdentity{}); err == nil {
		t.Fatal("DocumentEvent accepted an id above the signed 64-bit envelope limit")
	}
	if _, err := p.DocumentEvent(identity.DocCredit, math.MaxInt64, 7, billIdentity{}); err != nil {
		t.Fatalf("DocumentEvent refused the largest envelope id: %v", err)
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

// The daemon decodes the same two fixtures (zatca_integration queue/testdata). The tests below are the backend's half of
// that contract: the envelope and the onboarding request must marshal to exactly those fields.
func readJSONFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func marshalFields(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func TestBillEnvelopeMatchesTheContractFixture(t *testing.T) {
	got := marshalFields(t, Message{
		DocType:            "bill",
		ID:                 42,
		BranchID:           7,
		DBName:             "tenant_acme",
		TenantKey:          "acme-0001",
		PosID:              "pos-1",
		BillRequestID:      "3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c",
		EventID:            "9b1d2c3e-4f5a-4b6c-8d7e-0f1a2b3c4d5e",
		EGSSequenceID:      "7",
		InvoiceUUID:        "0c4f7a1e-2b3d-4e5f-8a9b-1c2d3e4f5a6b",
		RequestFingerprint: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	})
	if want := readJSONFixture(t, "bill_envelope.json"); !reflect.DeepEqual(got, want) {
		t.Fatalf("envelope = %v; want the contract fixture %v", got, want)
	}
}

func TestCreditEnvelopeMatchesTheContractFixture(t *testing.T) {
	got := marshalFields(t, Message{
		DocType:            identity.DocCredit,
		ID:                 9,
		BranchID:           7,
		DBName:             "tenant_acme",
		TenantKey:          "acme-0001",
		PosID:              "pos-1",
		BillRequestID:      "5e1a7c2d-9b3f-4c8e-a6d1-2f0b9c4e7a13",
		EventID:            "1f6e2d3c-4b5a-4978-8c6d-5e4f3a2b1c0d",
		EGSSequenceID:      "7",
		InvoiceUUID:        "8a2b4c6d-1e3f-4a5b-9c7d-0e2f4a6b8c1d",
		RequestFingerprint: "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
	})
	if want := readJSONFixture(t, "credit_envelope.json"); !reflect.DeepEqual(got, want) {
		t.Fatalf("envelope = %v; want the contract fixture %v", got, want)
	}
}

func TestDebitEnvelopeMatchesTheContractFixture(t *testing.T) {
	got := marshalFields(t, Message{
		DocType:            identity.DocDebit,
		ID:                 10,
		BranchID:           7,
		DBName:             "tenant_acme",
		TenantKey:          "acme-0001",
		PosID:              "pos-1",
		BillRequestID:      "c3d4e5f6-0a1b-4c2d-8e3f-4a5b6c7d8e9f",
		EventID:            "2a3b4c5d-6e7f-4081-9a2b-3c4d5e6f7a8b",
		EGSSequenceID:      "7",
		InvoiceUUID:        "d4e5f6a7-b8c9-4d0e-8f1a-2b3c4d5e6f70",
		RequestFingerprint: "0a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f9",
	})
	if want := readJSONFixture(t, "debit_envelope.json"); !reflect.DeepEqual(got, want) {
		t.Fatalf("envelope = %v; want the contract fixture %v", got, want)
	}
}

func TestOnboardRequestMatchesTheContractFixture(t *testing.T) {
	got := marshalFields(t, OnboardRequest{DBName: "tenant_acme", TenantKey: "acme-0001", BranchID: 7, OTP: "123456"})
	if want := readJSONFixture(t, "onboard_request.json"); !reflect.DeepEqual(got, want) {
		t.Fatalf("onboarding request = %v; want the contract fixture %v", got, want)
	}
}

func TestOnboardReplyRefusalIsReportedWithTheDaemonsReason(t *testing.T) {
	if err := parseOnboardReply([]byte(`{"accepted":true}`)); err != nil {
		t.Fatalf("an accepted reply = %v", err)
	}
	err := parseOnboardReply([]byte(`{"accepted":false,"error":"onboarding of this branch is already running"}`))
	var rejected *OnboardRejectedError
	if !errors.As(err, &rejected) || rejected.Reason != "onboarding of this branch is already running" {
		t.Fatalf("refusal = %v; want an OnboardRejectedError with the daemon's reason", err)
	}
}

func TestOnboardingErrorsReachTheOperatorWithTheirStatus(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
	}{
		"refused by the daemon": {&OnboardRejectedError{Reason: "onboarding of this branch is already running"}, http.StatusUnprocessableEntity},
		"no daemon answers":     {nats.ErrNoResponders, http.StatusServiceUnavailable},
		"the daemon is slow":    {nats.ErrTimeout, http.StatusServiceUnavailable},
		"anything else":         {errors.New("connection reset"), http.StatusInternalServerError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			writeOnboardError(c, tc.err)
			if c.Writer.Status() != tc.status {
				t.Fatalf("status = %d; want %d", c.Writer.Status(), tc.status)
			}
		})
	}
}
