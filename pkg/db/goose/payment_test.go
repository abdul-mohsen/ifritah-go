package goosemigrations

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	gen "ifritah/web-service-gin/pkg/db/gen"
)

// TestPaymentWritesMatchTheSchema checks 00009 and the payment queries against the migrated schema: SAR is seeded and found by
// code, a payment row is written with its bill and currency, the bill total can be read back, and a draft's payments can be
// replaced. It needs a MySQL server.
func TestPaymentWritesMatchTheSchema(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_payment")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	companyID := execInsert(t, db, "INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')")
	execInsert(t, db, "INSERT INTO branches (id, name, company_id) VALUES (7, 'branch 7', ?)", companyID)

	queries := gen.New(db)
	currency, err := queries.GetCurrencyIDByCode(ctx, "SAR")
	if err != nil || currency == 0 {
		t.Fatalf("SAR lookup = %d, %v; want the seeded currency", currency, err)
	}
	billID := createBillRow(t, queries)
	if err := writeSaleFields(queries, billID, 1, "pos-1", "3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("write sale: %v", err)
	}

	paidAt := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	if err := queries.InsertBillPayment(ctx, gen.InsertBillPaymentParams{
		BillID:        billID,
		Date:          paidAt,
		PaidAt:        paidAt,
		Amount:        decimal.RequireFromString("10.00"),
		CurrencyID:    currency,
		PaymentMethod: 10,
	}); err != nil {
		t.Fatalf("insert payment: %v", err)
	}

	var count int
	scanOne(t, db, &count, "SELECT COUNT(*) FROM bill_payment WHERE bill_id = ? AND currency_id = ?", billID, currency)
	if count != 1 {
		t.Fatalf("payment rows for the bill = %d; want 1", count)
	}
	total, err := queries.GetBillTotal(ctx, billID)
	if err != nil || !total.Equal(decimal.Zero) {
		t.Fatalf("bill total = %s, %v; want 0 for a bill with no lines", total, err)
	}
	if err := queries.DeleteBillPayments(ctx, billID); err != nil {
		t.Fatalf("delete payments: %v", err)
	}
	scanOne(t, db, &count, "SELECT COUNT(*) FROM bill_payment WHERE bill_id = ?", billID)
	if count != 0 {
		t.Fatalf("payment rows after delete = %d; want 0", count)
	}
}
