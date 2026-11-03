package goosemigrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestCreditDebitNoteContract checks what 00011 adds, on a database that already has a bill, a legacy credit note and a bill record. The
// bill record becomes the bill's source. A credit note from the old flow credited its whole bill. After Up, one bill can have several
// credit and debit notes, a note has one record in zatca_submission, and the note lines total as bill_product does. Down refuses while a
// note has a record or a bill has two credit notes, and otherwise restores the old shape. It needs a MySQL server.
func TestCreditDebitNoteContract(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_credit_debit")
	if _, err := provider.UpTo(ctx, 10); err != nil {
		t.Fatalf("goose up to 00010: %v", err)
	}
	companyID := execInsert(t, db, "INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')")
	execInsert(t, db, "INSERT INTO branches (id, name, company_id) VALUES (7, 'branch 7', ?)", companyID)
	billID := insertTestBill(t, db)
	if _, err := db.Exec("UPDATE bill SET total = 115.00, branch_id = 7, state = 1 WHERE id = ?", billID); err != nil {
		t.Fatal(err)
	}
	legacyCredit := execInsert(t, db, "INSERT INTO credit_note (bill_id, state, note) VALUES (?, 1, 'return')", billID)
	execInsert(t, db, "INSERT INTO zatca_submission (bill_id, status) VALUES (?, 2)", billID)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up to 00011: %v", err)
	}
	var amount string
	scanOne(t, db, &amount, "SELECT amount FROM credit_note WHERE id = ?", legacyCredit)
	if amount != "115.00" {
		t.Fatalf("legacy credit note amount = %s; want the bill total 115.00", amount)
	}
	var documentType string
	var sourceID int64
	if err := db.QueryRow("SELECT document_type, source_id FROM zatca_submission WHERE bill_id = ?", billID).Scan(&documentType, &sourceID); err != nil {
		t.Fatal(err)
	}
	if documentType != "bill" || sourceID != billID {
		t.Fatalf("bill record = %s %d; want bill %d", documentType, sourceID, billID)
	}

	// One bill can have several credit notes and several debit notes, each with its own request id and UUID.
	firstCredit, err := insertNote(db, "credit_note", billID, "11111111-1111-4111-8111-111111111111", "0c4f7a1e-2b3d-4e5f-8a9b-1c2d3e4f5a6b", strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("first credit note on a bill: %v", err)
	}
	if _, err := insertNote(db, "credit_note", billID, "22222222-2222-4222-8222-222222222222", "7c9e6679-7425-40de-944b-e07fc1f90ae7", strings.Repeat("b", 64)); err != nil {
		t.Fatalf("second credit note on a bill: %v", err)
	}
	debit, err := insertNote(db, "debit_note", billID, "33333333-3333-4333-8333-333333333333", "9d8c7b6a-5f4e-4d3c-8b2a-1f0e9d8c7b6a", strings.Repeat("c", 64))
	if err != nil {
		t.Fatalf("first debit note on a bill: %v", err)
	}
	if _, err := insertNote(db, "debit_note", billID, "44444444-4444-4444-8444-444444444444", "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d", strings.Repeat("d", 64)); err != nil {
		t.Fatalf("second debit note on a bill: %v", err)
	}
	_, err = insertNote(db, "credit_note", billID, "11111111-1111-4111-8111-111111111111", "5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b", strings.Repeat("e", 64))
	requireDBError(t, "a second credit note with the same request id", "uq_credit_note_request", err)
	_, err = insertNote(db, "debit_note", billID, "55555555-5555-4555-8555-555555555555", "6f7a8b9c-0d1e-4f2a-8b3c-4d5e6f7a8b9c", "not-a-fingerprint")
	requireDBError(t, "a debit note with a malformed fingerprint", "chk_debit_note_request_fingerprint", err)

	// The note's record in zatca_submission is keyed by its kind and its own id, and a bill keeps one record.
	execInsert(t, db, "INSERT INTO zatca_submission (bill_id, status, document_type, source_id) VALUES (?, 0, 'credit', ?)", billID, firstCredit)
	_, err = db.Exec("INSERT INTO zatca_submission (bill_id, status, document_type, source_id) VALUES (?, 0, 'credit', ?)", billID, firstCredit)
	requireDBError(t, "a second record for one credit note", "uq_zatca_document", err)
	_, err = db.Exec("INSERT INTO zatca_submission (bill_id, status, document_type, source_id) VALUES (?, 0, 'refund', ?)", billID, 99)
	requireDBError(t, "an unknown document type", "chk_zatca_document_type", err)
	_, err = db.Exec("INSERT INTO zatca_submission (bill_id, status, document_type, source_id) VALUES (?, 0, 'bill', ?)", billID, billID+1000)
	requireDBError(t, "a bill record whose source is not the bill", "chk_zatca_bill_source", err)
	_, err = db.Exec("INSERT INTO zatca_submission (bill_id, status, document_type, source_id) VALUES (?, 0, 'bill', ?)", billID, billID)
	requireDBError(t, "a second record for one bill", "uq_zatca_document", err)

	// Lines total as bill_product does: 1 unit at 10.00 with 0.33 of discount is 9.67 plus 15% VAT, and 3 units at 0.10 with 15% VAT
	// is 0.30 plus 0.05, because the VAT of 0.045 rounds half up.
	lineProduct := execInsert(t, db, "INSERT INTO bill_product (bill_id, name, price, quantity, vat, discount) VALUES (?, 'Brake pad', 10.00, 3.000, 15.00, 1.00)", billID)
	execInsert(t, db, "INSERT INTO credit_note_line (credit_note_id, bill_product_id, name, quantity, price, discount, vat) VALUES (?, ?, 'Brake pad', 1.000, 10.00, 0.33, 15.00)", firstCredit, lineProduct)
	scanOne(t, db, &amount, "SELECT total_including_vat FROM credit_note_line WHERE credit_note_id = ?", firstCredit)
	if amount != "11.12" {
		t.Fatalf("credit line total = %s; want 11.12", amount)
	}
	execInsert(t, db, "INSERT INTO debit_note_line (debit_note_id, name, quantity, price, vat) VALUES (?, 'Tow', 3.000, 0.10, 15.00)", debit)
	scanOne(t, db, &amount, "SELECT total_including_vat FROM debit_note_line WHERE debit_note_id = ?", debit)
	if amount != "0.35" {
		t.Fatalf("debit line total = %s; want 0.35", amount)
	}
	_, err = db.Exec("INSERT INTO credit_note_line (credit_note_id, bill_product_id, name, quantity, price, discount, vat) VALUES (?, ?, 'Brake pad', 0.000, 10.00, 0.00, 15.00)", firstCredit, lineProduct)
	requireDBError(t, "a credit line with no quantity", "chk_cnl_quantity", err)
	_, err = db.Exec("INSERT INTO debit_note_line (debit_note_id, name, quantity, price, vat) VALUES (?, 'Tow', 1.000, 5.00, 101.00)", debit)
	requireDBError(t, "a debit line with VAT above 100", "chk_dnl_vat", err)

	// Down refuses while a note has a ZATCA record, and it changes nothing.
	if _, err := provider.Down(ctx); err == nil || !strings.Contains(err.Error(), "chk_zatca_down_bills_only") {
		t.Fatalf("down with a note record = %v; want a refusal from chk_zatca_down_bills_only", err)
	}
	requireVersion(t, provider, 11)

	// Without the note records, Down still refuses while a bill has two credit notes, because the old shape allows one.
	if _, err := db.Exec("DELETE FROM zatca_submission WHERE document_type <> 'bill'"); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err == nil || !strings.Contains(err.Error(), "Duplicate entry") {
		t.Fatalf("down with two credit notes on a bill = %v; want a duplicate on unique_bill_id", err)
	}
	requireVersion(t, provider, 11)

	// Without the notes, Down restores the old shape, and Up applies it again.
	if _, err := db.Exec("DELETE FROM credit_note WHERE bill_id = ?", billID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM debit_note WHERE bill_id = ?", billID); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("goose down to 00010: %v", err)
	}
	requireVersion(t, provider, 10)
	var count int
	scanOne(t, db, &count, "SELECT COUNT(*) FROM information_schema.STATISTICS WHERE table_schema = DATABASE() AND table_name = 'credit_note' AND index_name = 'unique_bill_id' AND non_unique = 0")
	if count != 1 {
		t.Fatalf("unique_bill_id on credit_note after down = %d indexes; want 1", count)
	}
	scanOne(t, db, &count, "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE table_schema = DATABASE() AND table_name = 'zatca_submission' AND column_name IN ('document_type', 'source_id')")
	if count != 0 {
		t.Fatalf("zatca_submission key columns after down = %d; want none", count)
	}
	scanOne(t, db, &count, "SELECT COUNT(*) FROM information_schema.TABLES WHERE table_schema = DATABASE() AND table_name IN ('credit_note_line', 'debit_note_line')")
	if count != 0 {
		t.Fatalf("note line tables after down = %d; want none", count)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up to 00011 again: %v", err)
	}
	requireVersion(t, provider, 11)
}

// insertNote inserts a credit or debit note on the bill, with the given request id, invoice UUID and fingerprint. It returns the note's id
// or the database error.
func insertNote(db *sql.DB, table string, billID int64, requestID, invoiceUUID, fingerprint string) (int64, error) {
	res, err := db.Exec(fmt.Sprintf(`INSERT INTO %s (bill_id, state, note, amount, lines_total, invoice_uuid, pos_id, bill_request_id, request_fingerprint)
		VALUES (?, 1, 'return', 10.00, 10.00, ?, 'pos-1', ?, ?)`, table), billID, invoiceUUID, requestID, fingerprint)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// requireVersion fails unless the database is at version want.
func requireVersion(t *testing.T, provider interface {
	GetDBVersion(context.Context) (int64, error)
}, want int64) {
	t.Helper()
	got, err := provider.GetDBVersion(context.Background())
	if err != nil || got != want {
		t.Fatalf("database version = %d, %v; want %d", got, err, want)
	}
}
