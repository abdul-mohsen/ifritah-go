package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"ifritah/web-service-gin/pkg/testdb"
)

func recoveryEventRows(eventID, status string, attempts uint32, lastError any, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"event_id",
		"document_key",
		"subject",
		"payload",
		"status",
		"attempt_count",
		"next_attempt_at",
		"lease_owner",
		"lease_expires_at",
		"last_error",
		"created_at",
		"published_at",
		"updated_at",
	}).AddRow(
		uint64(42),
		eventID,
		"acme:7:bill:42",
		"zatca.bill.acme.7",
		[]byte(`{"doc_type":"bill","id":42}`),
		status,
		attempts,
		now.Add(-time.Hour),
		nil,
		nil,
		lastError,
		now.Add(-2*time.Hour),
		nil,
		now.Add(-time.Minute),
	)
}

func recoveryAuditRows(eventID, key string, operatorID int32, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"outbox_id",
		"event_id",
		"idempotency_key",
		"operator_user_id",
		"operator_username",
		"reason",
		"previous_status",
		"previous_attempt_count",
		"previous_last_error",
		"recovered_at",
	}).AddRow(
		uint64(9),
		uint64(42),
		eventID,
		key,
		operatorID,
		"admin",
		"retry after NATS outage",
		"dead_letter",
		uint32(2),
		"nats unavailable",
		now,
	)
}

func TestRecoverDeadLetterIsIdempotentAndPreservesAuditSnapshot(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	now := time.Date(2026, 11, 6, 14, 0, 0, 0, time.UTC)
	operatorID := int32(7)
	request := RecoveryRequest{
		EventID:          eventA,
		IdempotencyKey:   "replay-2026-11-06-001",
		OperatorUserID:   &operatorID,
		OperatorUsername: "admin",
		Reason:           "retry after NATS outage",
		Now:              now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM outbox\s+WHERE event_id = \?`).
		WithArgs(eventA).
		WillReturnRows(recoveryEventRows(eventA, "dead_letter", 2, "nats unavailable", now))
	mock.ExpectQuery(`FROM outbox_recovery_audit\s+WHERE idempotency_key = \?`).
		WithArgs(request.IdempotencyKey).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`UPDATE outbox\s+SET status = 'pending'`).
		WithArgs(now, uint64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO outbox_recovery_audit")).
		WithArgs(uint64(42), eventA, request.IdempotencyKey, operatorID, "admin", request.Reason, "dead_letter", uint32(2), "nats unavailable", now).
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()

	first, err := RecoverDeadLetter(context.Background(), sqlDB, request)
	if err != nil {
		t.Fatalf("first recovery: %v", err)
	}
	if first.AlreadyApplied {
		t.Fatal("first recovery was reported as already applied")
	}
	if first.Event.Status != "pending" || first.Event.AttemptCount != 0 {
		t.Fatalf("recovered event = status %q, attempts %d; want pending, 0", first.Event.Status, first.Event.AttemptCount)
	}
	if !sameJSON(t, string(first.Event.Payload), `{"doc_type":"bill","id":42}`) {
		t.Fatalf("recovered payload changed: %s", first.Event.Payload)
	}
	if first.Audit.PreviousStatus != "dead_letter" || first.Audit.PreviousAttemptCount != 2 {
		t.Fatalf("audit snapshot = status %q, attempts %d; want dead_letter, 2", first.Audit.PreviousStatus, first.Audit.PreviousAttemptCount)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM outbox\s+WHERE event_id = \?`).
		WithArgs(eventA).
		WillReturnRows(recoveryEventRows(eventA, "pending", 0, nil, now))
	mock.ExpectQuery(`FROM outbox_recovery_audit\s+WHERE idempotency_key = \?`).
		WithArgs(request.IdempotencyKey).
		WillReturnRows(recoveryAuditRows(eventA, request.IdempotencyKey, operatorID, now))
	mock.ExpectCommit()

	second, err := RecoverDeadLetter(context.Background(), sqlDB, request)
	if err != nil {
		t.Fatalf("idempotent recovery: %v", err)
	}
	if !second.AlreadyApplied {
		t.Fatal("idempotent recovery was not reported as already applied")
	}
	if second.Event.Status != "pending" || second.Event.AttemptCount != 0 {
		t.Fatalf("idempotent event = status %q, attempts %d; want pending, 0", second.Event.Status, second.Event.AttemptCount)
	}
	if second.Audit.ID != 9 || second.Audit.PreviousAttemptCount != 2 {
		t.Fatalf("idempotent audit = %+v; want persisted audit snapshot", second.Audit)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRecoverDeadLetterPreservesPayloadAndDeliveryAudit(t *testing.T) {
	sqlDB := testdb.Open(t, "outbox_recovery_audit", "outbox")
	appendCommitted(t, sqlDB, testEvent(eventA))

	const lastError = "ZATCA daemon unavailable"
	if _, err := sqlDB.Exec(
		"UPDATE outbox SET status = 'dead_letter', attempt_count = 2, last_error = ? WHERE event_id = ?",
		lastError,
		eventA,
	); err != nil {
		t.Fatal(err)
	}

	operatorID := int32(7)
	now := time.Now().UTC().Truncate(time.Second)
	result, err := RecoverDeadLetter(context.Background(), sqlDB, RecoveryRequest{
		EventID:          eventA,
		IdempotencyKey:   "replay-integration-001",
		OperatorUserID:   &operatorID,
		OperatorUsername: "admin",
		Reason:           "retry after daemon restart",
		Now:              now,
	})
	if err != nil {
		t.Fatalf("RecoverDeadLetter: %v", err)
	}
	if result.Event.Status != "pending" || result.Event.AttemptCount != 0 {
		t.Fatalf("result event = %+v; want pending with zero attempts", result.Event)
	}

	var status, documentKey, payload string
	var attempts int
	if err := sqlDB.QueryRow(
		"SELECT status, attempt_count, document_key, payload FROM outbox WHERE event_id = ?",
		eventA,
	).Scan(&status, &attempts, &documentKey, &payload); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || attempts != 0 || documentKey != "acme:7:bill:42" {
		t.Fatalf("outbox delivery state = %q, %d, %q; want pending, 0, unchanged document key", status, attempts, documentKey)
	}
	var decoded any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatal(err)
	}
	if !sameJSON(t, payload, string(testEvent(eventA).Payload)) {
		t.Fatalf("outbox payload changed: %s", payload)
	}

	var previousStatus string
	var previousAttempts int
	var auditedError string
	if err := sqlDB.QueryRow(
		"SELECT previous_status, previous_attempt_count, previous_last_error FROM outbox_recovery_audit WHERE idempotency_key = ?",
		"replay-integration-001",
	).Scan(&previousStatus, &previousAttempts, &auditedError); err != nil {
		t.Fatal(err)
	}
	if previousStatus != "dead_letter" || previousAttempts != 2 || auditedError != lastError {
		t.Fatalf("audit snapshot = %q, %d, %q; want dead_letter, 2, %q", previousStatus, previousAttempts, auditedError, lastError)
	}
}

func TestRecoveredEventDispatchesWithItsOriginalEventID(t *testing.T) {
	sqlDB := testdb.Open(t, "outbox_recovery_audit", "outbox")
	appendCommitted(t, sqlDB, testEvent(eventA))
	if _, err := sqlDB.Exec(
		"UPDATE outbox SET status = 'dead_letter', attempt_count = 3, last_error = 'daemon unavailable' WHERE event_id = ?",
		eventA,
	); err != nil {
		t.Fatal(err)
	}

	operatorID := int32(7)
	if _, err := RecoverDeadLetter(context.Background(), sqlDB, RecoveryRequest{
		EventID:          eventA,
		IdempotencyKey:   "replay-dispatch-001",
		OperatorUserID:   &operatorID,
		OperatorUsername: "admin",
		Reason:           "retry after daemon restart",
		Now:              time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RecoverDeadLetter: %v", err)
	}

	publisher := &fakePublisher{}
	drainer := testDrainer(sqlDB, publisher, time.Now().UTC().Add(time.Minute))
	n, err := drainer.DrainOnce(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("first recovered drain = %d, %v; want 1, nil", n, err)
	}
	if ids := publisher.msgIDs(); len(ids) != 1 || ids[0] != eventA {
		t.Fatalf("published event IDs = %v; want [%s]", ids, eventA)
	}

	n, err = drainer.DrainOnce(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("second recovered drain = %d, %v; want 0, nil", n, err)
	}
	if ids := publisher.msgIDs(); len(ids) != 1 {
		t.Fatalf("second drain published duplicate event IDs %v", ids)
	}
}

func TestRecoverDeadLetterRejectsNonDeadLetterEvents(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	now := time.Date(2026, 11, 6, 14, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`FROM outbox\s+WHERE event_id = \?`).
		WithArgs(eventA).
		WillReturnRows(recoveryEventRows(eventA, "published", 1, nil, now))
	mock.ExpectQuery(`FROM outbox_recovery_audit\s+WHERE idempotency_key = \?`).
		WithArgs("replay-published").
		WillReturnError(sql.ErrNoRows)

	_, err = RecoverDeadLetter(context.Background(), sqlDB, RecoveryRequest{
		EventID:          eventA,
		IdempotencyKey:   "replay-published",
		OperatorUsername: "admin",
		Reason:           "should not replay published event",
		Now:              now,
	})
	if err != ErrRecoveryNotDeadLetter {
		t.Fatalf("recovery error = %v; want %v", err, ErrRecoveryNotDeadLetter)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRecoveryRejectsNonASCIIIdempotencyKeys(t *testing.T) {
	_, err := RecoverDeadLetter(context.Background(), nil, RecoveryRequest{
		EventID:          eventA,
		IdempotencyKey:   "إعادة-001",
		OperatorUsername: "admin",
		Reason:           "retry",
	})
	if err == nil || !errors.Is(err, ErrRecoveryInvalidRequest) {
		t.Fatalf("non-ASCII idempotency key error = %v; want invalid request", err)
	}
}
