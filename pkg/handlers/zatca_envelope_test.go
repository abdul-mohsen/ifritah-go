package handlers

import (
	"encoding/json"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/nats-io/nats.go"
)

type envelopeCaptureJetStream struct {
	nats.JetStreamContext
	subject string
	data    []byte
	opts    int
}

func (s *envelopeCaptureJetStream) Publish(subject string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error) {
	s.subject = subject
	s.data = append(s.data[:0], data...)
	s.opts = len(opts)
	return &nats.PubAck{}, nil
}

func TestZatcaPublisherEmitsVersionedEnvelopeAndDeterministicIdentity(t *testing.T) {
	stream := &envelopeCaptureJetStream{}
	publisher := newRegressionPublisher(&regressionJetStream{})
	publisher.js = stream

	if err := publisher.SubmitBill(42, 7); err != nil {
		t.Fatalf("SubmitBill: %v", err)
	}
	firstPayload := append([]byte(nil), stream.data...)

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(stream.data, &fields); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	for _, field := range []string{
		"contract",
		"version",
		"schema_version",
		"event_id",
		"tenant_id",
		"branch_id",
		"egs_id",
		"document_type",
		"source_id",
		"outbox_id",
		"stable_document_key",
		"attempt",
		"attempt_count",
		"attempt_id",
	} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("published payload omitted %q: %s", field, stream.data)
		}
	}
	if stream.opts != 1 {
		t.Fatalf("JetStream publish options = %d, want one deterministic MsgId option", stream.opts)
	}
	if stream.subject != "zatca.bill.test_tenant.7" {
		t.Fatalf("subject = %q, want zatca.bill.test_tenant.7", stream.subject)
	}

	var stableDocumentKey string
	if err := json.Unmarshal(fields["stable_document_key"], &stableDocumentKey); err != nil {
		t.Fatalf("decode stable document key: %v", err)
	}
	if stableDocumentKey != "test_tenant:7:bill:42" {
		t.Fatalf("stable document key = %q, want test_tenant:7:bill:42", stableDocumentKey)
	}
	var legacy struct {
		DocType  string `json:"doc_type"`
		ID       int64  `json:"id"`
		BranchID int64  `json:"branch_id"`
		DBName   string `json:"db_name"`
		OTP      string `json:"otp"`
	}
	if err := json.Unmarshal(stream.data, &legacy); err != nil {
		t.Fatalf("decode legacy daemon payload: %v", err)
	}
	if legacy.DocType != "bill" || legacy.ID != 42 ||
		legacy.BranchID != 7 || legacy.DBName != "test_tenant" {
		t.Fatalf("legacy decoder fields = %#v, want bill/42/7/test_tenant", legacy)
	}
	var versioned Message
	if err := json.Unmarshal(stream.data, &versioned); err != nil {
		t.Fatalf("decode versioned message: %v", err)
	}
	if got := versioned.NATSMessageID(); got != stableDocumentKey {
		t.Fatalf("NATS message ID = %q, want stable document key %q", got, stableDocumentKey)
	}

	if err := publisher.SubmitBill(42, 7); err != nil {
		t.Fatalf("SubmitBill retry: %v", err)
	}
	if string(stream.data) != string(firstPayload) {
		t.Fatalf("same document produced a different payload on retry:\nfirst=%s\nsecond=%s", firstPayload, stream.data)
	}
}

func TestZatcaOutboxPayloadContainsEnvelopeContextAndLegacyAliases(t *testing.T) {
	publisher := newRegressionPublisher(&regressionJetStream{})
	params, err := publisher.buildZatcaOutboxParams(7, zatcaDocCredit, 42, 3, "")
	if err != nil {
		t.Fatalf("buildZatcaOutboxParams: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(params.Payload, &fields); err != nil {
		t.Fatalf("decode outbox payload: %v", err)
	}
	for _, field := range []string{
		"contract",
		"version",
		"schema_version",
		"event_id",
		"tenant_id",
		"branch_id",
		"egs_id",
		"document_type",
		"source_id",
		"outbox_id",
		"stable_document_key",
		"attempt",
		"attempt_count",
		"attempt_id",
		"doc_type",
		"id",
		"db_name",
	} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("outbox payload omitted %q: %s", field, params.Payload)
		}
	}

	var docType, dbName string
	var documentID, branchID int64
	if err := json.Unmarshal(fields["doc_type"], &docType); err != nil {
		t.Fatalf("decode doc_type: %v", err)
	}
	if err := json.Unmarshal(fields["id"], &documentID); err != nil {
		t.Fatalf("decode id: %v", err)
	}
	if err := json.Unmarshal(fields["branch_id"], &branchID); err != nil {
		t.Fatalf("decode branch_id: %v", err)
	}
	if err := json.Unmarshal(fields["db_name"], &dbName); err != nil {
		t.Fatalf("decode db_name: %v", err)
	}
	if docType != zatcaDocCredit || documentID != 42 ||
		branchID != 3 || dbName != "test_tenant" {
		t.Fatalf("legacy aliases = %s/%d/%d/%s, want credit/42/3/test_tenant",
			docType, documentID, branchID, dbName)
	}
}

func TestZatcaOutboxEnvelopeUsesRowIdentityAndStableRetryMessageID(t *testing.T) {
	stream := &envelopeCaptureJetStream{}
	publisher := newRegressionPublisher(&regressionJetStream{})
	publisher.js = stream

	row := db.ZatcaEventOutbox{
		ID:             99,
		EventID:        "evt-credit-99",
		TenantID:       "test_tenant",
		DocumentType:   zatcaDocCredit,
		DocumentID:     42,
		BranchID:       3,
		IdempotencyKey: "test_tenant:3:credit:42",
		Payload: []byte(`{
			"doc_type":"credit",
			"id":42,
			"branch_id":3,
			"db_name":"test_tenant"
		}`),
		AttemptCount: 2,
	}

	if err := publisher.publishOutboxRow(row); err != nil {
		t.Fatalf("publishOutboxRow: %v", err)
	}

	var message Message
	if err := json.Unmarshal(stream.data, &message); err != nil {
		t.Fatalf("decode published outbox envelope: %v", err)
	}
	if message.OutboxID != row.ID {
		t.Fatalf("outbox ID = %d, want %d", message.OutboxID, row.ID)
	}
	if message.EventID != row.EventID {
		t.Fatalf("event ID = %q, want %q", message.EventID, row.EventID)
	}
	if message.StableDocumentKey != row.IdempotencyKey {
		t.Fatalf("stable document key = %q, want %q", message.StableDocumentKey, row.IdempotencyKey)
	}
	if message.Attempt != row.AttemptCount+1 {
		t.Fatalf("attempt = %d, want %d", message.Attempt, row.AttemptCount+1)
	}
	if message.AttemptCount != row.AttemptCount {
		t.Fatalf("attempt count = %d, want %d", message.AttemptCount, row.AttemptCount)
	}
	if message.AttemptID != "99:3" {
		t.Fatalf("attempt ID = %q, want 99:3", message.AttemptID)
	}
	if got := message.NATSMessageID(); got != row.IdempotencyKey {
		t.Fatalf("NATS message ID = %q, want %q", got, row.IdempotencyKey)
	}
	if stream.opts != 1 {
		t.Fatalf("JetStream publish options = %d, want one deterministic MsgId option", stream.opts)
	}
}

func TestZatcaOnboardingEnvelopeRetainsLegacyOTPCompatibility(t *testing.T) {
	stream := &envelopeCaptureJetStream{}
	publisher := newRegressionPublisher(&regressionJetStream{})
	publisher.js = stream

	if err := publisher.OnboadBranch(7, "123456"); err != nil {
		t.Fatalf("OnboadBranch: %v", err)
	}

	var message Message
	if err := json.Unmarshal(stream.data, &message); err != nil {
		t.Fatalf("decode onboarding envelope: %v", err)
	}
	if message.DocType != DocOnboard || message.ID != 7 ||
		message.BranchID != 7 || message.DBName != "test_tenant" ||
		message.OTP != "123456" {
		t.Fatalf("legacy onboarding fields = %#v", message)
	}
	if message.StableDocumentKey == "" ||
		message.StableDocumentKey == "test_tenant:7:onboard:7:otp-123456" {
		t.Fatalf("onboarding stable key is missing or leaks OTP: %q", message.StableDocumentKey)
	}
	if message.NATSMessageID() != message.StableDocumentKey {
		t.Fatalf("onboarding NATS message ID = %q, want %q",
			message.NATSMessageID(), message.StableDocumentKey)
	}
}
