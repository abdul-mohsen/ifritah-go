package goosemigrations

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	gen "ifritah/web-service-gin/pkg/db/gen"
)

// TestSaleWritesMatchTheSchema runs the two writes of a sale against the migrated schema: the bill row with only the
// columns it needs, then the update that writes the sale's fields, number and identity. The sqlmock tests in
// pkg/handlers check the statement order. This checks that the statements fit the columns and their NOT NULL rules.
// It needs a MySQL server.
func TestSaleWritesMatchTheSchema(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_sale_write")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	companyID := execInsert(t, db, "INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')")
	execInsert(t, db, "INSERT INTO branches (id, name, company_id) VALUES (7, 'branch 7', ?)", companyID)

	queries := gen.New(db)
	created, err := queries.CreateBill(ctx, gen.CreateBillParams{
		EffectiveDate:   time.Now(),
		StoreID:         1,
		MerchantID:      1,
		Discount:        decimal.Zero,
		MaintenanceCost: decimal.Zero,
	})
	if err != nil {
		t.Fatalf("create bill row: %v", err)
	}
	billID, err := created.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	branch, number, posID := uint32(7), uint64(1), "pos-1"
	requestID := "3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c"
	invoiceUUID := "7c9e6679-7425-40de-944b-e07fc1f90ae7"
	if err := queries.UpdateBillByID(ctx, gen.UpdateBillByIDParams{
		ID:              uint64(billID),
		EffectiveDate:   time.Now(),
		State:           1,
		Discount:        decimal.Zero,
		StoreID:         1,
		SequenceNumber:  &number,
		MerchantID:      1,
		MaintenanceCost: decimal.Zero,
		PaymentMethod:   10,
		BranchID:        &branch,
		PosID:           &posID,
		BillRequestID:   sql.NullString{String: requestID, Valid: true},
		InvoiceUuid:     sql.NullString{String: invoiceUUID, Valid: true},
	}); err != nil {
		t.Fatalf("update bill row: %v", err)
	}

	var state int32
	var seq uint64
	var gotPos, gotRequest, gotInvoice sql.NullString
	if err := db.QueryRow("SELECT state, sequence_number, pos_id, bill_request_id, invoice_uuid FROM bill WHERE id = ?", billID).
		Scan(&state, &seq, &gotPos, &gotRequest, &gotInvoice); err != nil {
		t.Fatalf("read bill row: %v", err)
	}
	if state != 1 || seq != 1 || gotPos.String != posID || gotRequest.String != requestID || gotInvoice.String != invoiceUUID {
		t.Fatalf("bill row = state %d, number %d, pos %q, request %q, uuid %q; want the sale's values",
			state, seq, gotPos.String, gotRequest.String, gotInvoice.String)
	}
}
