package handlers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

type recordingOutboxPublisher struct {
	err       error
	onPublish func()
	called    bool
	tenantID  uint64
	eventID   string
}

func (p *recordingOutboxPublisher) PublishOutbox(_ context.Context, tenantID uint64, eventID string) error {
	p.called = true
	p.tenantID = tenantID
	p.eventID = eventID
	if p.onPublish != nil {
		p.onPublish()
	}
	return p.err
}

type recordingOutboxTransport struct {
	err     error
	payload []byte
}

func (t *recordingOutboxTransport) PublishOutboxPayload(payload []byte) error {
	t.payload = append([]byte(nil), payload...)
	return t.err
}

func TestAddBillRetainsCommittedSaleWhenPublisherFails(t *testing.T) {
	h, mock, cleanup := newAtomicSaleTestHandler(t)
	defer cleanup()

	publisherErr := errors.New("nats unavailable")
	publisher := &recordingOutboxPublisher{err: publisherErr}
	h.outboxPublisher = publisher

	expectNewAtomicSale(mock)
	mock.ExpectExec(`(?i)INSERT INTO outbox`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	ctx, recorder := atomicSaleContext(atomicSaleBody("100"))
	h.AddBill(ctx)

	if recorder.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
	}
	if !publisher.called {
		t.Fatal("publisher was not called after the committed sale")
	}
	if publisher.tenantID != 42 || publisher.eventID == "" {
		t.Fatalf("publisher identity = tenant %d event %q", publisher.tenantID, publisher.eventID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAddBillDoesNotPublishBeforeCommit(t *testing.T) {
	h, mock, cleanup := newAtomicSaleTestHandler(t)
	defer cleanup()

	var callbackErr error
	publisher := &recordingOutboxPublisher{
		onPublish: func() {
			_, callbackErr = h.DB.Query("SELECT 1")
		},
	}
	h.outboxPublisher = publisher

	expectNewAtomicSale(mock)
	mock.ExpectExec(`(?i)INSERT INTO outbox`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1))

	ctx, recorder := atomicSaleContext(atomicSaleBody("100"))
	h.AddBill(ctx)

	if recorder.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
	}
	if callbackErr != nil {
		t.Fatalf("publisher ran before commit: %v", callbackErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestInsertAtomicSaleOutboxAcceptsSameEventReplay(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	payload := []byte(`{"event_id":"event-1","tenant_id":42}`)
	params := testOutboxParams(payload)
	mock.ExpectExec(`(?i)INSERT INTO outbox`).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate event"})
	mock.ExpectQuery(`(?is)SELECT.*FROM outbox`).
		WithArgs(params.TenantID, params.EventID).
		WillReturnRows(outboxRows(outboxEventFromParams(params, outboxStatusPending), outboxStatusPending))

	err = insertAtomicSaleOutbox(context.Background(), db.New(sqlDB), params)
	if err != nil {
		t.Fatalf("same event replay returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestInsertAtomicSaleOutboxRejectsChangedContent(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	params := testOutboxParams([]byte(`{"event_id":"event-1","tenant_id":42}`))
	existing := params
	existing.Payload = json.RawMessage(`{"event_id":"event-1","tenant_id":42,"changed":true}`)
	existing.PayloadFingerprint = sql.NullString{
		String: hashPayload(existing.Payload),
		Valid:  true,
	}
	existing.ContentHash = existing.PayloadFingerprint

	mock.ExpectExec(`(?i)INSERT INTO outbox`).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate event"})
	mock.ExpectQuery(`(?is)SELECT.*FROM outbox`).
		WithArgs(params.TenantID, params.EventID).
		WillReturnRows(outboxRows(outboxEventFromParams(existing, outboxStatusPending), outboxStatusPending))

	err = insertAtomicSaleOutbox(context.Background(), db.New(sqlDB), params)
	if !errors.Is(err, ErrOutboxContentConflict) {
		t.Fatalf("error = %v, want ErrOutboxContentConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestOutboxDispatcherReclaimsExpiredLease(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	payload := []byte(`{"doc_type":"bill","branch_id":7,"db_name":"tenant_db"}`)
	params := testOutboxParams(payload)
	event := outboxEventFromParams(params, outboxStatusInFlight)
	event.AttemptCount = 1
	transport := &recordingOutboxTransport{}
	dispatcher := newOutboxDispatcher(sqlDB, db.New(sqlDB), transport)
	dispatcher.owner = "test-owner"
	dispatcher.now = func() time.Time { return time.Unix(1000, 0) }

	mock.ExpectBegin()
	mock.ExpectExec(`(?is)UPDATE outbox.*status = 'in_flight'`).
		WithArgs("test-owner", sqlmock.AnyArg(), params.TenantID, params.EventID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?is)SELECT.*FROM outbox`).
		WithArgs(params.TenantID, params.EventID).
		WillReturnRows(outboxRows(event, outboxStatusInFlight))
	mock.ExpectCommit()
	mock.ExpectExec(`(?is)UPDATE outbox.*status = 'published'`).
		WithArgs(params.TenantID, params.EventID, "test-owner").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := dispatcher.PublishOutbox(context.Background(), params.TenantID, params.EventID); err != nil {
		t.Fatalf("PublishOutbox: %v", err)
	}
	if string(transport.payload) != string(payload) {
		t.Fatalf("published payload = %s, want %s", transport.payload, payload)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestOutboxDispatcherRetainsRetryableEventAfterPublishFailure(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	payload := []byte(`{"doc_type":"bill","branch_id":7,"db_name":"tenant_db"}`)
	params := testOutboxParams(payload)
	event := outboxEventFromParams(params, outboxStatusInFlight)
	event.AttemptCount = 1
	transport := &recordingOutboxTransport{err: errors.New("nats unavailable")}
	dispatcher := newOutboxDispatcher(sqlDB, db.New(sqlDB), transport)
	dispatcher.owner = "test-owner"
	dispatcher.now = func() time.Time { return time.Unix(1000, 0) }

	mock.ExpectBegin()
	mock.ExpectExec(`(?is)UPDATE outbox.*status = 'in_flight'`).
		WithArgs("test-owner", sqlmock.AnyArg(), params.TenantID, params.EventID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?is)SELECT.*FROM outbox`).
		WithArgs(params.TenantID, params.EventID).
		WillReturnRows(outboxRows(event, outboxStatusInFlight))
	mock.ExpectCommit()
	mock.ExpectExec(`(?is)UPDATE outbox.*status = ?`).
		WithArgs(
			outboxStatusRetryable,
			sqlmock.AnyArg(),
			"nats unavailable",
			nil,
			nil,
			params.TenantID,
			params.EventID,
			"test-owner",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := dispatcher.PublishOutbox(context.Background(), params.TenantID, params.EventID); err == nil {
		t.Fatal("PublishOutbox succeeded despite transport failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func testOutboxParams(payload []byte) db.InsertSaleOutboxParams {
	hash := hashPayload(payload)
	return db.InsertSaleOutboxParams{
		TenantID:      42,
		EventID:       "event-1",
		AggregateType: "bill",
		AggregateID:   99,
		PosID:         stringPtr("pos-1"),
		BillRequestID: sql.NullString{String: "request-1", Valid: true},
		InvoiceUuid:   sql.NullString{String: "invoice-99", Valid: true},
		EgsSequenceID: stringPtr("7:1"),
		Payload:       payload,
		PayloadFingerprint: sql.NullString{
			String: hash,
			Valid:  true,
		},
		ContentHash: sql.NullString{
			String: hash,
			Valid:  true,
		},
	}
}

func outboxEventFromParams(params db.InsertSaleOutboxParams, status string) db.Outbox {
	return db.Outbox{
		ID:                 1,
		TenantID:           params.TenantID,
		EventID:            params.EventID,
		AggregateType:      params.AggregateType,
		AggregateID:        params.AggregateID,
		PosID:              params.PosID,
		BillRequestID:      params.BillRequestID,
		InvoiceUuid:        params.InvoiceUuid,
		EgsSequenceID:      params.EgsSequenceID,
		Payload:            params.Payload,
		PayloadFingerprint: params.PayloadFingerprint,
		ContentHash:        params.ContentHash,
		Status:             status,
		NextAttemptAt:      time.Unix(1, 0),
		LeaseOwner:         stringPtr("test-owner"),
		LeaseExpiresAt:     timePtr(time.Unix(900, 0)),
		CreatedAt:          time.Unix(1, 0),
		UpdatedAt:          time.Unix(1, 0),
	}
}

func outboxRows(event db.Outbox, status string) *sqlmock.Rows {
	event.Status = status
	return sqlmock.NewRows([]string{
		"id",
		"tenant_id",
		"event_id",
		"aggregate_type",
		"aggregate_id",
		"pos_id",
		"bill_request_id",
		"invoice_uuid",
		"egs_sequence_id",
		"payload",
		"payload_fingerprint",
		"content_hash",
		"status",
		"attempt_count",
		"next_attempt_at",
		"lease_owner",
		"lease_expires_at",
		"last_error",
		"published_at",
		"dead_lettered_at",
		"manual_review_at",
		"created_at",
		"updated_at",
	}).AddRow(
		event.ID,
		event.TenantID,
		event.EventID,
		event.AggregateType,
		event.AggregateID,
		nullableValue(event.PosID),
		nullStringValue(event.BillRequestID),
		nullStringValue(event.InvoiceUuid),
		nullableValue(event.EgsSequenceID),
		[]byte(event.Payload),
		nullStringValue(event.PayloadFingerprint),
		nullStringValue(event.ContentHash),
		event.Status,
		event.AttemptCount,
		event.NextAttemptAt,
		nullableValue(event.LeaseOwner),
		nullableTimeValue(event.LeaseExpiresAt),
		nullableValue(event.LastError),
		nullableTimeValue(event.PublishedAt),
		nullableTimeValue(event.DeadLetteredAt),
		nullableTimeValue(event.ManualReviewAt),
		event.CreatedAt,
		event.UpdatedAt,
	)
}

func hashPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func nullableValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullStringValue(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func nullableTimeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func TestOutboxPayloadHashIsStableJSON(t *testing.T) {
	payload := atomicSaleOutboxPayload{
		DocType:       "bill",
		ID:            99,
		BranchID:      7,
		DBName:        "tenant_db",
		TenantID:      42,
		PosID:         "pos-1",
		BillRequestID: "request-1",
		EventID:       "event-1",
		EGSSequenceID: "7:1",
		InvoiceUUID:   "invoice-99",
	}
	first, firstHash, err := atomicSaleOutboxPayloadBytes(payload)
	if err != nil {
		t.Fatalf("first payload: %v", err)
	}
	second, secondHash, err := atomicSaleOutboxPayloadBytes(payload)
	if err != nil {
		t.Fatalf("second payload: %v", err)
	}
	if string(first) != string(second) || firstHash != secondHash {
		t.Fatalf("payload hash is not stable: %s/%s vs %s/%s", first, firstHash, second, secondHash)
	}
	if len(firstHash) != 64 {
		t.Fatalf("payload hash = %q, want a 64-character SHA-256 hex value", firstHash)
	}
}

func TestOutboxRetryBackoffIsExponentialAndBounded(t *testing.T) {
	if got := outboxRetryBackoff(1); got != time.Second {
		t.Fatalf("first retry backoff = %s, want 1s", got)
	}
	if got := outboxRetryBackoff(2); got != 2*time.Second {
		t.Fatalf("second retry backoff = %s, want 2s", got)
	}
	if got := outboxRetryBackoff(100); got != 1024*time.Second {
		t.Fatalf("bounded retry backoff = %s, want 1024s", got)
	}
}
