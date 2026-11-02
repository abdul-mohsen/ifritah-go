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
)

// The envelope a bill sends is the contract the daemon reads. It has these fields, and no tenant_id.
func TestBillEventCarriesTheCanonicalEnvelope(t *testing.T) {
	p := &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"}
	ident := billIdentity{
		PosID:         "pos-1",
		BillRequestID: "3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c",
		InvoiceUUID:   "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		Fingerprint:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
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
		"doc_type":            "bill",
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
}

// The envelope's id is a signed 64-bit integer, so a bill id above that limit cannot be sent.
func TestBillEventRefusesAnIDTheEnvelopeCannotCarry(t *testing.T) {
	p := &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"}
	if _, err := p.BillEvent(math.MaxInt64+1, 7, billIdentity{}); err == nil {
		t.Fatal("BillEvent accepted a bill id above the signed 64-bit envelope limit")
	}
	if _, err := p.BillEvent(math.MaxInt64, 7, billIdentity{}); err != nil {
		t.Fatalf("BillEvent refused the largest envelope id: %v", err)
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
