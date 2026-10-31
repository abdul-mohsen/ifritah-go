package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	generated "ifritah/web-service-gin/pkg/db/gen"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func TestEGSSequenceReservesDistinctICVsForConcurrentAllocations(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(4)
	mock.MatchExpectationsInOrder(false)

	identity := testEGSIdentity()
	requests := []EGSSequenceReservationRequest{
		{
			EGSSequenceIdentity: identity,
			ReservationKey:      "tenant-7:branch-9:bill:41",
			ReservationToken:    "reservation-41",
			DocumentUUID:        "00000000-0000-4000-8000-000000000041",
			OutboxEventID:       "00000000-0000-4000-8000-000000000141",
		},
		{
			EGSSequenceIdentity: identity,
			ReservationKey:      "tenant-7:branch-9:bill:42",
			ReservationToken:    "reservation-42",
			DocumentUUID:        "00000000-0000-4000-8000-000000000042",
			OutboxEventID:       "00000000-0000-4000-8000-000000000142",
		},
	}

	for _, request := range requests {
		expectEnsureEGSSequence(mock, request.EGSSequenceIdentity)
	}
	expectGetEGSSequence(mock, identity, 0)
	expectGetEGSSequence(mock, identity, 1)
	for _, request := range requests {
		expectReserveEGSSequence(mock, request)
	}

	allocator := NewEGSSequenceAllocator(sqlDB, generated.New(sqlDB))
	start := make(chan struct{})
	results := make(chan EGSSequenceReservation, len(requests))
	errs := make(chan error, len(requests))
	var group sync.WaitGroup
	for _, request := range requests {
		group.Add(1)
		go func(request EGSSequenceReservationRequest) {
			defer group.Done()
			<-start
			reservation, err := allocator.Reserve(context.Background(), request)
			if err != nil {
				errs <- err
				return
			}
			results <- reservation
		}(request)
	}
	close(start)
	group.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent reservation: %v", err)
	}
	seen := map[uint64]bool{}
	for reservation := range results {
		if seen[reservation.ICV] {
			t.Fatalf("duplicate ICV %d allocated concurrently", reservation.ICV)
		}
		seen[reservation.ICV] = true
	}
	if len(seen) != 2 || !seen[1] || !seen[2] {
		t.Fatalf("reserved ICVs = %#v, want exactly 1 and 2", seen)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("concurrent reservation expectations: %v", err)
	}
}

func TestEGSSequenceCommitRollsBackDocumentAndOutboxOnFailure(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	request := testEGSCommitRequest(EGSDocumentQueued)
	mock.ExpectBegin()
	mock.ExpectExec(`(?is)INSERT INTO zatca_egs_document`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`(?is)INSERT INTO outbox`).
		WillReturnError(&mysql.MySQLError{Number: 1456, Message: "outbox unavailable"})
	mock.ExpectRollback()

	err = NewEGSSequenceAllocator(sqlDB, generated.New(sqlDB)).Commit(context.Background(), request)
	if err == nil {
		t.Fatal("commit succeeded despite outbox failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("rollback expectations: %v", err)
	}
}

func TestEGSSequenceCommitRejectsRejectedAndAmbiguousDocuments(t *testing.T) {
	for _, state := range []string{EGSDocumentRejected, EGSDocumentAmbiguous} {
		t.Run(state, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock.New: %v", err)
			}
			defer sqlDB.Close()

			request := testEGSCommitRequest(state)
			mock.ExpectBegin()
			mock.ExpectExec(`(?is)INSERT INTO zatca_egs_document`).
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec(`(?is)INSERT INTO outbox`).
				WillReturnResult(sqlmock.NewResult(2, 1))
			mock.ExpectExec(`(?is)UPDATE egs_sequence AS s.*EXISTS`).
				WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectRollback()

			err = NewEGSSequenceAllocator(sqlDB, generated.New(sqlDB)).Commit(context.Background(), request)
			if !errors.Is(err, ErrEGSSequenceCommitNotApplied) {
				t.Fatalf("commit error = %v, want ErrEGSSequenceCommitNotApplied", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("%s expectations: %v", state, err)
			}
		})
	}
}

func TestEGSSequenceRejectsCrossTenantOrEGSOwnership(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	identity := testEGSIdentity()
	identity.TenantID = 8
	identity.EGSID = "egs-other"
	request := EGSSequenceReservationRequest{
		EGSSequenceIdentity: identity,
		ReservationKey:      "tenant-8:branch-9:bill:1",
		ReservationToken:    "reservation-tenant-8",
		DocumentUUID:        "00000000-0000-4000-8000-000000000081",
		OutboxEventID:       "00000000-0000-4000-8000-000000000181",
	}

	expectEnsureEGSSequence(mock, identity)
	mock.ExpectQuery(`(?is)SELECT.*FROM egs_sequence.*WHERE egs_sequence_id = \?.*FOR UPDATE`).
		WithArgs(identity.EGSSequenceID, identity.TenantID, identity.BranchID, identity.VATNumber, identity.EGSID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = NewEGSSequenceAllocator(sqlDB, generated.New(sqlDB)).Reserve(context.Background(), request)
	if !errors.Is(err, ErrEGSSequenceOwnership) {
		t.Fatalf("ownership error = %v, want ErrEGSSequenceOwnership", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ownership expectations: %v", err)
	}
}

func TestEGSSequenceCommitRejectsDuplicateICVOrUUID(t *testing.T) {
	for _, message := range []string{"duplicate ICV", "duplicate UUID"} {
		t.Run(message, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock.New: %v", err)
			}
			defer sqlDB.Close()

			request := testEGSCommitRequest(EGSDocumentQueued)
			mock.ExpectBegin()
			mock.ExpectExec(`(?is)INSERT INTO zatca_egs_document`).
				WillReturnError(&mysql.MySQLError{Number: 1062, Message: message})
			mock.ExpectRollback()

			err = NewEGSSequenceAllocator(sqlDB, generated.New(sqlDB)).Commit(context.Background(), request)
			if err == nil {
				t.Fatal("duplicate document commit succeeded")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("duplicate expectations: %v", err)
			}
		})
	}
}

func TestEGSSequenceSourceContract(t *testing.T) {
	schema := readDatabaseSource(t, "schema", "schema.sql")
	queries := readDatabaseSource(t, "queries", "egs_sequence.sql")
	sequence := tableSource(t, schema, "egs_sequence")
	document := tableSource(t, schema, "zatca_egs_document")
	outbox := tableSource(t, schema, "outbox")

	for _, column := range []string{
		"`egs_sequence_id`",
		"`tenant_id`",
		"`branch_id`",
		"`vat_number`",
		"`egs_id`",
		"`last_icv`",
		"`previous_invoice_hash`",
		"`reserved_icv`",
		"`reserved_previous_invoice_hash`",
	} {
		requireSourceContains(t, sequence, column)
	}
	requireSourceContains(t, sequence, "UNIQUE KEY `uq_egs_sequence_owner` (`tenant_id`,`branch_id`,`vat_number`,`egs_id`)")
	requireSourceContains(t, document, "UNIQUE KEY `uq_zatca_egs_document_icv` (`egs_sequence_id`,`icv`)")
	requireSourceContains(t, document, "UNIQUE KEY `uq_zatca_egs_document_uuid` (`tenant_id`,`document_uuid`)")
	requireSourceContains(t, outbox, "`icv` bigint unsigned DEFAULT NULL")
	requireSourceContains(t, outbox, "`pih` varchar(255)")
	requireSourceContains(t, outbox, "UNIQUE KEY `uq_outbox_egs_icv` (`tenant_id`,`egs_sequence_id`,`icv`)")

	if stringsContainsAny(sequence, "`user_id`", "`company_id`", "`pos_id`") {
		t.Fatal("EGS sequence ownership must not alias user, company, or POS identity")
	}
	for _, contract := range []string{
		"-- name: ReserveEGSSequence :execresult",
		"-- name: CommitEGSSequence :execresult",
		"AND EXISTS (",
		"d.state NOT IN ('rejected', 'ambiguous')",
		"AND s.tenant_id = sqlc.arg('tenant_id')",
		"AND s.egs_id = sqlc.arg('egs_id')",
	} {
		requireSourceContains(t, queries, contract)
	}
}

func expectEnsureEGSSequence(mock sqlmock.Sqlmock, identity EGSSequenceIdentity) {
	mock.ExpectBegin()
	mock.ExpectExec(`(?is)INSERT INTO egs_sequence`).
		WithArgs(identity.EGSSequenceID, identity.TenantID, identity.BranchID, identity.VATNumber, identity.EGSID).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func expectGetEGSSequence(mock sqlmock.Sqlmock, identity EGSSequenceIdentity, lastICV uint64) {
	mock.ExpectQuery(`(?is)SELECT.*FROM egs_sequence.*WHERE egs_sequence_id = \?.*FOR UPDATE`).
		WithArgs(identity.EGSSequenceID, identity.TenantID, identity.BranchID, identity.VATNumber, identity.EGSID).
		WillReturnRows(egsSequenceRows(identity, lastICV))
}

func expectReserveEGSSequence(mock sqlmock.Sqlmock, request EGSSequenceReservationRequest) {
	mock.ExpectExec(`(?is)UPDATE egs_sequence\s+SET.*reserved_icv = last_icv \+ 1`).
		WithArgs(
			request.ReservationKey,
			request.ReservationToken,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			request.EGSSequenceID,
			request.TenantID,
			request.BranchID,
			request.VATNumber,
			request.EGSID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func egsSequenceRows(identity EGSSequenceIdentity, lastICV uint64) *sqlmock.Rows {
	now := time.Unix(1, 0)
	return sqlmock.NewRows([]string{
		"egs_sequence_id",
		"tenant_id",
		"branch_id",
		"vat_number",
		"egs_id",
		"last_icv",
		"previous_invoice_hash",
		"reservation_key",
		"reservation_token",
		"reserved_icv",
		"reserved_previous_invoice_hash",
		"reserved_document_uuid",
		"reserved_outbox_event_id",
		"reserved_at",
		"lease_expires_at",
		"created_at",
		"updated_at",
	}).AddRow(
		identity.EGSSequenceID,
		identity.TenantID,
		identity.BranchID,
		identity.VATNumber,
		identity.EGSID,
		lastICV,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		now,
		now,
	)
}

func testEGSIdentity() EGSSequenceIdentity {
	return EGSSequenceIdentity{
		EGSSequenceID: "tenant-7:branch-9:vat-310000000000003:egs-1",
		TenantID:      7,
		BranchID:      9,
		VATNumber:     "310000000000003",
		EGSID:         "egs-1",
	}
}

func testEGSCommitRequest(state string) EGSSequenceCommitRequest {
	reservation := EGSSequenceReservation{
		EGSSequenceIdentity: testEGSIdentity(),
		ReservationKey:      "tenant-7:branch-9:bill:41",
		ReservationToken:    "reservation-41",
		DocumentUUID:        "00000000-0000-4000-8000-000000000041",
		OutboxEventID:       "00000000-0000-4000-8000-000000000141",
		ICV:                 1,
		PIH:                 stringPointer("previous-hash"),
		LeaseExpiresAt:      time.Now().Add(time.Minute),
	}
	return EGSSequenceCommitRequest{
		Reservation:         reservation,
		DocumentType:        "invoice",
		SourceID:            41,
		State:               state,
		Snapshot:            json.RawMessage(`{"document_uuid":"00000000-0000-4000-8000-000000000041"}`),
		Payload:             json.RawMessage(`{"event_id":"00000000-0000-4000-8000-000000000141"}`),
		PreviousInvoiceHash: stringPointer("signed-document-hash"),
	}
}

func stringsContainsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
