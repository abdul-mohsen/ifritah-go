package handlers

import (
	"context"
	"testing"
	"time"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestZatcaOutboxDrainFailureIsDurable(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	stream := &regressionJetStream{publishErr: errPublisherUnavailable}
	drainer := newZatcaOutboxDrainer(sqlDB, db.New(sqlDB), newRegressionPublisher(stream))
	drainer.batchSize = 1

	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").
		WithArgs(int32(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"event_id",
			"tenant_id",
			"merchant_id",
			"document_type",
			"document_id",
			"branch_id",
			"idempotency_key",
			"payload",
			"status",
			"attempt_count",
			"available_at",
			"claimed_at",
			"published_at",
			"last_error",
			"last_error_at",
			"created_at",
			"updated_at",
		}).AddRow(
			uint64(1),
			"evt-bill-1",
			"test_tenant",
			int32(7),
			zatcaDocBill,
			uint64(42),
			uint32(3),
			"test_tenant:3:bill:42",
			[]byte(`{"doc_type":"bill","id":42,"branch_id":3,"db_name":"test_tenant"}`),
			"pending",
			uint32(0),
			now,
			nil,
			nil,
			nil,
			nil,
			now,
			now,
		))
	mock.ExpectExec("UPDATE zatca_event_outbox").
		WithArgs(uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec("UPDATE zatca_event_outbox").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := drainer.Drain(context.Background()); err == nil {
		t.Fatal("Drain() error = nil, want durable publication failure")
	}
	if stream.calls != 1 {
		t.Fatalf("publisher calls = %d, want 1", stream.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("durable failure expectations: %v", err)
	}
}

func TestEnqueueDebitUsesCallerTransaction(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	stream := &regressionJetStream{}
	publisher := newRegressionPublisher(stream)
	queries := db.New(sqlDB)

	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	mock.ExpectQuery("SELECT").
		WithArgs(uint64(77), int32(7), int32(7), uint32(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"bill_id",
			"merchant_id",
			"branch_id",
			"sequence_number",
			"egs_id",
			"document_id",
			"stable_document_key",
		}).AddRow(
			uint64(77),
			uint64(99),
			int32(7),
			uint32(3),
			uint64(20260099),
			"egs-3",
			"debit:77",
			"test_tenant:3:debit:77",
		))
	mock.ExpectExec("INSERT INTO zatca_event_outbox").
		WillReturnResult(sqlmock.NewResult(4, 1))

	if err := publisher.EnqueueDebit(
		context.Background(),
		queries.WithTx(tx),
		7,
		77,
		3,
	); err != nil {
		t.Fatalf("EnqueueDebit: %v", err)
	}

	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if stream.calls != 0 {
		t.Fatalf("publisher calls = %d, want 0 during enqueue", stream.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("debit outbox expectations: %v", err)
	}
}
