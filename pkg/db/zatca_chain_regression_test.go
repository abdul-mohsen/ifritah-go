package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	db "ifritah/web-service-gin/pkg/db/gen"
)

func TestZatcaEGSChainOwnershipTransaction(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	ctx := context.Background()
	lease := time.Now().Add(5 * time.Minute)
	reservationKey := "tenant:7:bill:42"
	reservationToken := "owner-token"
	newHash := "signed-hash"

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO zatca_egs_chain").
		WithArgs(uint32(7), "egs-7").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)SELECT.*FOR UPDATE").
		WithArgs(uint32(7), "egs-7").
		WillReturnRows(sqlmock.NewRows([]string{
			"branch_id",
			"egs_id",
			"last_icv",
			"previous_invoice_hash",
			"reservation_key",
			"reservation_token",
			"reserved_icv",
			"reserved_previous_hash",
			"reserved_at",
			"lease_expires_at",
			"updated_at",
		}).AddRow(
			uint32(7),
			"egs-7",
			uint64(0),
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			time.Now(),
		))
	mock.ExpectExec("UPDATE zatca_egs_chain").
		WithArgs(reservationKey, reservationToken, sqlmock.AnyArg(), uint32(7), "egs-7").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	qtx := db.New(sqlDB).WithTx(tx)
	if err := qtx.EnsureZatcaEGSChain(ctx, db.EnsureZatcaEGSChainParams{
		BranchID: 7,
		EgsID:    "egs-7",
	}); err != nil {
		t.Fatalf("EnsureZatcaEGSChain: %v", err)
	}
	state, err := qtx.GetZatcaEGSChainForUpdate(ctx, db.GetZatcaEGSChainForUpdateParams{
		BranchID: 7,
		EgsID:    "egs-7",
	})
	if err != nil {
		t.Fatalf("GetZatcaEGSChainForUpdate: %v", err)
	}
	if state.LastIcv != 0 || state.EgsID != "egs-7" {
		t.Fatalf("unexpected chain state: %+v", state)
	}
	if _, err := qtx.ClaimZatcaEGSChain(ctx, db.ClaimZatcaEGSChainParams{
		ReservationKey:   &reservationKey,
		ReservationToken: &reservationToken,
		LeaseExpiresAt:   &lease,
		BranchID:         7,
		EgsID:            "egs-7",
	}); err != nil {
		t.Fatalf("ClaimZatcaEGSChain: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit reservation transaction: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("reservation transaction expectations: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE zatca_egs_chain").
		WithArgs(newHash, uint32(7), "egs-7", reservationKey, reservationToken, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx, err = sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx commit: %v", err)
	}
	qtx = db.New(sqlDB).WithTx(tx)
	reservedICV := uint64(1)
	if _, err := qtx.CommitZatcaEGSChain(ctx, db.CommitZatcaEGSChainParams{
		PreviousInvoiceHash: &newHash,
		BranchID:            7,
		EgsID:               "egs-7",
		ReservationKey:      &reservationKey,
		ReservationToken:    &reservationToken,
		ReservedIcv:         &reservedICV,
	}); err != nil {
		t.Fatalf("CommitZatcaEGSChain: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit chain transaction: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("commit transaction expectations: %v", err)
	}
}

func TestZatcaEGSChainOwnerMismatchCannotCommit(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	reservationKey := "tenant:7:bill:42"
	wrongToken := "wrong-owner"
	newHash := "must-not-commit"
	reservedICV := uint64(1)

	mock.ExpectExec("UPDATE zatca_egs_chain").
		WithArgs(newHash, uint32(7), "egs-7", reservationKey, wrongToken, reservedICV).
		WillReturnResult(sqlmock.NewResult(0, 0))

	result, err := db.New(sqlDB).CommitZatcaEGSChain(context.Background(), db.CommitZatcaEGSChainParams{
		PreviousInvoiceHash: &newHash,
		BranchID:            7,
		EgsID:               "egs-7",
		ReservationKey:      &reservationKey,
		ReservationToken:    &wrongToken,
		ReservedIcv:         &reservedICV,
	})
	if err != nil {
		t.Fatalf("CommitZatcaEGSChain: %v", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected: %v", err)
	}
	if affected != 0 {
		t.Fatalf("owner-mismatched commit affected %d rows, want 0", affected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("owner mismatch expectations: %v", err)
	}
}
