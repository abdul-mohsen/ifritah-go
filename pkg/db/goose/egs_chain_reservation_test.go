package goosemigrations

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

type egsReservationRow struct {
	branchID     uint32
	lastICV      uint64
	previousHash sql.NullString
	key          sql.NullString
	token        sql.NullString
	reservedICV  sql.NullInt64
	lease        sql.NullTime
}

// TestEGSReservationContract verifies the SQL contract owned by queue.SQLChain in the external
// ZATCA daemon.
// The backend deliberately has no runtime reservation owner after its chain package was removed.
func TestEGSReservationContract(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_egs_chain")
	if _, err := provider.UpTo(ctx, 5); err != nil {
		t.Fatalf("goose up to 00005: %v", err)
	}

	companyID := execInsert(t, db, "INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')")
	execInsert(t, db, "INSERT INTO branches (id, name, company_id) VALUES (7, 'branch 7', ?)", companyID)
	execInsert(t, db, "INSERT INTO branch_zatca_config (branch_id) VALUES (7)")
	mustExec(t, db, "UPDATE branch_zatca_config SET last_icv = 7, previous_invoice_hash = 'previous' WHERE branch_id = 7")

	if _, err := db.Exec("UPDATE branch_zatca_config SET icv_reservation_key = 'partial' WHERE branch_id = 7"); err == nil {
		t.Fatal("partial reservation was accepted")
	} else {
		requireDBError(t, "partial reservation", "chk_bzc_icv_reservation", err)
	}

	lockAndCheckNowait(t, db)

	now := time.Now().UTC().Truncate(time.Second)
	liveLease := now.Add(time.Hour)
	if err := bookEGSReservation(ctx, db, 7, "document-1", "token-1", now, liveLease); err != nil {
		t.Fatalf("book document-1: %v", err)
	}
	row := readEGSReservation(t, db)
	if row.lastICV != 7 || row.key.String != "document-1" || row.token.String != "token-1" ||
		row.reservedICV.Int64 != 8 || !row.lease.Valid {
		t.Fatalf("booked reservation = %#v; want ICV 8 owned by document-1", row)
	}

	if err := bookEGSReservation(ctx, db, 7, "document-1", "token-redelivery", now, liveLease); err != nil {
		t.Fatalf("same-document re-delivery: %v", err)
	}
	row = readEGSReservation(t, db)
	if row.token.String != "token-1" || row.reservedICV.Int64 != 8 {
		t.Fatalf("same-document re-delivery changed reservation = %#v", row)
	}
	if err := bookEGSReservation(ctx, db, 7, "document-2", "token-2", now, liveLease); !errors.Is(err, errEGSReservationBusy) {
		t.Fatalf("different live owner = %v; want busy", err)
	}

	mustExec(t, db, "UPDATE branch_zatca_config SET reservation_lease_expires_at = '2000-01-01 00:00:00' WHERE branch_id = 7")
	if err := bookEGSReservation(ctx, db, 7, "document-2", "token-2", now, liveLease); err != nil {
		t.Fatalf("expired reservation takeover: %v", err)
	}
	row = readEGSReservation(t, db)
	if row.key.String != "document-2" || row.token.String != "token-2" || row.reservedICV.Int64 != 8 {
		t.Fatalf("taken-over reservation = %#v; want same ICV 8 with document-2", row)
	}

	if affected := releaseEGSReservation(t, db, 7, "document-2", "wrong-token"); affected != 0 {
		t.Fatalf("release by stale owner affected %d rows; want 0", affected)
	}
	if affected := releaseEGSReservation(t, db, 7, "document-2", "token-2"); affected != 1 {
		t.Fatalf("release by owner affected %d rows; want 1", affected)
	}
	row = readEGSReservation(t, db)
	if row.lastICV != 7 || row.previousHash.String != "previous" || row.key.Valid || row.token.Valid || row.reservedICV.Valid {
		t.Fatalf("released reservation = %#v; want chain unchanged and reservation cleared", row)
	}

	if err := bookEGSReservation(ctx, db, 7, "document-3", "token-3", now, liveLease); err != nil {
		t.Fatalf("book document-3: %v", err)
	}
	if affected := commitEGSReservation(t, db, 7, "document-3", "token-3", 8, "signed-3"); affected != 1 {
		t.Fatalf("commit by owner affected %d rows; want 1", affected)
	}
	row = readEGSReservation(t, db)
	if row.lastICV != 8 || row.previousHash.String != "signed-3" || row.key.Valid || row.token.Valid || row.reservedICV.Valid {
		t.Fatalf("committed reservation = %#v; want ICV 8, signed-3, and reservation cleared", row)
	}
	if affected := commitEGSReservation(t, db, 7, "document-3", "token-3", 8, "redelivered"); affected != 0 {
		t.Fatalf("stale commit re-delivery affected %d rows; want 0", affected)
	}
	if affected := releaseEGSReservation(t, db, 7, "document-3", "token-3"); affected != 0 {
		t.Fatalf("stale release re-delivery affected %d rows; want 0", affected)
	}

	if err := bookEGSReservation(ctx, db, 7, "document-4", "token-4", now, liveLease); err != nil {
		t.Fatalf("book document-4 after commit: %v", err)
	}
	row = readEGSReservation(t, db)
	if row.reservedICV.Int64 != 9 {
		t.Fatalf("next reservation ICV = %d; want 9", row.reservedICV.Int64)
	}
}

var errEGSReservationBusy = errors.New("egschain reservation is busy")

func lockAndCheckNowait(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	first, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback()
	var branchID uint32
	if err := first.QueryRowContext(ctx,
		"SELECT branch_id FROM branch_zatca_config WHERE branch_id = 7 FOR UPDATE",
	).Scan(&branchID); err != nil {
		t.Fatalf("lock branch row: %v", err)
	}

	second, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Rollback()
	if err := second.QueryRowContext(ctx,
		"SELECT branch_id FROM branch_zatca_config WHERE branch_id = 7 FOR UPDATE NOWAIT",
	).Scan(&branchID); err == nil {
		t.Fatal("second booking acquired a row locked by the first transaction")
	} else {
		var mysqlErr *mysql.MySQLError
		if !errors.As(err, &mysqlErr) || mysqlErr.Number != 3572 {
			t.Fatalf("second booking lock error = %v; want MySQL NOWAIT error 3572", err)
		}
	}
}

func bookEGSReservation(ctx context.Context, db *sql.DB, branchID uint32, documentKey, token string, now, lease time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var row egsReservationRow
	if err := tx.QueryRowContext(ctx, `SELECT branch_id, last_icv, previous_invoice_hash,
		icv_reservation_key, icv_reservation_token, reserved_icv, reservation_lease_expires_at
		FROM branch_zatca_config WHERE branch_id = ? FOR UPDATE NOWAIT`, branchID).Scan(
		&row.branchID, &row.lastICV, &row.previousHash, &row.key, &row.token, &row.reservedICV, &row.lease,
	); err != nil {
		return err
	}
	if row.key.Valid {
		if row.key.String != documentKey {
			return errEGSReservationBusy
		}
		if row.lease.Valid && row.lease.Time.After(now) {
			return tx.Commit()
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE branch_zatca_config
		SET icv_reservation_key = ?, icv_reservation_token = ?, reserved_icv = ?, reservation_lease_expires_at = ?
		WHERE branch_id = ?`, documentKey, token, row.lastICV+1, lease, branchID); err != nil {
		return err
	}
	return tx.Commit()
}

func readEGSReservation(t *testing.T, db *sql.DB) egsReservationRow {
	t.Helper()
	var row egsReservationRow
	if err := db.QueryRow(`SELECT branch_id, last_icv, previous_invoice_hash,
		icv_reservation_key, icv_reservation_token, reserved_icv, reservation_lease_expires_at
		FROM branch_zatca_config WHERE branch_id = 7`).Scan(
		&row.branchID, &row.lastICV, &row.previousHash, &row.key, &row.token, &row.reservedICV, &row.lease,
	); err != nil {
		t.Fatal(err)
	}
	return row
}

func releaseEGSReservation(t *testing.T, db *sql.DB, branchID uint32, documentKey, token string) int64 {
	t.Helper()
	result, err := db.Exec(`UPDATE branch_zatca_config
		SET icv_reservation_key = NULL, icv_reservation_token = NULL, reserved_icv = NULL,
			reservation_lease_expires_at = NULL
		WHERE branch_id = ? AND icv_reservation_token = ? AND icv_reservation_key = ?`,
		branchID, token, documentKey)
	if err != nil {
		t.Fatal(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatal(err)
	}
	return affected
}

func commitEGSReservation(t *testing.T, db *sql.DB, branchID uint32, documentKey, token string, reservedICV uint64, signedHash string) int64 {
	t.Helper()
	result, err := db.Exec(`UPDATE branch_zatca_config
		SET last_icv = reserved_icv, previous_invoice_hash = ?, icv_reservation_key = NULL,
			icv_reservation_token = NULL, reserved_icv = NULL, reservation_lease_expires_at = NULL
		WHERE branch_id = ? AND icv_reservation_token = ? AND icv_reservation_key = ? AND reserved_icv = ?`,
		signedHash, branchID, token, documentKey, reservedICV)
	if err != nil {
		t.Fatal(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatal(err)
	}
	return affected
}
