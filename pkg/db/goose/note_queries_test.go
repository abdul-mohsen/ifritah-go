package goosemigrations

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/shopspring/decimal"

	gen "ifritah/web-service-gin/pkg/db/gen"
)

// TestNoteQueriesRunOnTheMigratedSchema runs the queries that credit and debit notes make, and the queries whose joins changed for
// several credit notes on one bill. A note is read back as it was written, the bill's detail, list and dashboard show one row for the
// bill, and the sums of a note see the amounts of the earlier notes. It needs a MySQL server.
func TestNoteQueriesRunOnTheMigratedSchema(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_note_queries")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	companyID := execInsert(t, db, "INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')")
	execInsert(t, db, "INSERT INTO branches (id, name, company_id) VALUES (7, 'branch 7', ?)", companyID)
	execInsert(t, db, "INSERT INTO store (id, name, company_id) VALUES (1, 'store 1', ?)", companyID)

	queries := gen.New(db)
	const posID = "pos-1"
	const billRequestID = "3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c"
	const fingerprint = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	billID := createBillRow(t, queries)
	if err := writeSaleFields(queries, billID, 1, posID, billRequestID, fingerprint); err != nil {
		t.Fatalf("write sale fields: %v", err)
	}
	brakePad := addLine(t, db, queries, billID, "Brake pad", "10.00", "2")
	addLine(t, db, queries, billID, "maintenance_cost", "15.00", "1")
	if err := queries.RefreshBillTotals(ctx, billID); err != nil {
		t.Fatalf("refresh totals: %v", err)
	}

	checkLockedBill(t, queries, billID, posID, billRequestID)

	first := insertCreditNote(t, queries, billID, "11111111-1111-4111-8111-111111111111", "0c4f7a1e-2b3d-4e5f-8a9b-1c2d3e4f5a6b", "returned", "11.50")
	execInsertLine(t, queries, first, brakePad, "1", "10.00")
	second := insertCreditNote(t, queries, billID, "22222222-2222-4222-8222-222222222222", "7c9e6679-7425-40de-944b-e07fc1f90ae7", "second", "3.00")

	checkCreditNoteReads(t, queries, billID, first, posID)
	checkBillListings(t, queries, billID, second)

	// A debit note has the same identity rules, and its line is read back.
	debit := insertDebitNote(t, queries, billID, "33333333-3333-4333-8333-333333333333")
	if err := queries.InsertDebitNoteLine(ctx, gen.InsertDebitNoteLineParams{DebitNoteID: uint64(debit), Name: "Towing", Quantity: decimal.RequireFromString("1"), Price: decimal.RequireFromString("5.00"), Vat: decimal.RequireFromString("15.00")}); err != nil {
		t.Fatalf("insert debit line: %v", err)
	}
	dgot, err := queries.GetDebitNoteByRequestIdentity(ctx, gen.GetDebitNoteByRequestIdentityParams{PosID: strPtr(posID), BillRequestID: sql.NullString{String: "33333333-3333-4333-8333-333333333333", Valid: true}})
	if err != nil || int64(dgot.ID) != debit {
		t.Fatalf("debit note by request identity = %+v, %v; want note %d", dgot, err, debit)
	}
}

// checkLockedBill checks the bill that a note is verified against: its total, its branch and store, its lines, and its request identity.
func checkLockedBill(t *testing.T, queries *gen.Queries, billID uint64, posID, billRequestID string) {
	t.Helper()
	ctx := context.Background()
	lock, err := queries.LockBillForNote(ctx, billID)
	if err != nil || !lock.Total.Equal(decimal.RequireFromString("40.25")) || lock.BranchID == nil || *lock.BranchID != 7 {
		t.Fatalf("locked bill = %+v, %v; want total 40.25 on branch 7", lock, err)
	}
	if store, err := queries.GetBillStoreID(ctx, billID); err != nil || store != 1 {
		t.Fatalf("store of the bill = %d, %v; want 1", store, err)
	}
	lines, err := queries.ListBillLinesForNote(ctx, billID)
	if err != nil || len(lines) != 2 {
		t.Fatalf("bill lines = %d, %v; want 2", len(lines), err)
	}
	if found, err := queries.GetBillByRequestIdentity(ctx, gen.GetBillByRequestIdentityParams{PosID: strPtr(posID), BillRequestID: sql.NullString{String: billRequestID, Valid: true}}); err != nil || found.ID != billID {
		t.Fatalf("the bill's request identity = %+v, %v; want bill %d", found, err, billID)
	}
}

// checkCreditNoteReads checks the first credit note of a bill: its request identity, the lines that it credits, and the amounts of the
// notes on the bill.
func checkCreditNoteReads(t *testing.T, queries *gen.Queries, billID uint64, first int64, posID string) {
	t.Helper()
	ctx := context.Background()
	got, err := queries.GetCreditNoteByRequestIdentity(ctx, gen.GetCreditNoteByRequestIdentityParams{PosID: strPtr(posID), BillRequestID: sql.NullString{String: "11111111-1111-4111-8111-111111111111", Valid: true}})
	if err != nil || got.ID != uint64(first) || got.BillID != billID {
		t.Fatalf("credit note by request identity = %+v, %v; want note %d", got, err, first)
	}
	credited, err := queries.ListCreditedLinesForBill(ctx, billID)
	if err != nil || len(credited) != 1 || !credited[0].Quantity.Equal(decimal.RequireFromString("1")) {
		t.Fatalf("credited lines = %+v, %v; want one line of quantity 1", credited, err)
	}
	amounts, err := queries.ListCreditAmountsForBill(ctx, billID)
	if err != nil || len(amounts) != 2 {
		t.Fatalf("credit amounts = %v, %v; want the two notes", amounts, err)
	}
}

// checkBillListings checks that a bill with two credit notes is listed, shown and found for its PDF once. The detail shows the latest note.
func checkBillListings(t *testing.T, queries *gen.Queries, billID uint64, second int64) {
	t.Helper()
	ctx := context.Background()
	credit, err := queries.GetCreditBillByID(ctx, billID)
	if err != nil || credit.CreditNote == nil || *credit.CreditNote != "second" {
		t.Fatalf("credit detail = %v, %v; want the latest note, second", credit.CreditNote, err)
	}
	pdf, err := queries.GetBillPDFByID(ctx, billID)
	if err != nil || pdf.CreditID == nil || *pdf.CreditID != uint64(second) {
		t.Fatalf("PDF of the bill = %v, %v; want credit note %d", pdf.CreditID, err, second)
	}
	list, err := queries.GetAllBill(ctx, gen.GetAllBillParams{Limit: 50})
	if err != nil {
		t.Fatalf("list bills: %v", err)
	}
	if n := countBill(list, billID); n != 2 {
		t.Fatalf("rows of the bill in the list = %d; want 2, the bill and one credit variant", n)
	}
	recent, err := queries.GetDashboardRecentInvoices(ctx)
	if err != nil {
		t.Fatalf("recent invoices: %v", err)
	}
	if n := countRecent(recent, billID); n != 1 {
		t.Fatalf("rows of the bill in recent invoices = %d; want 1", n)
	}
	stats, err := queries.GetDashboardCreditNoteStats(ctx, gen.GetDashboardCreditNoteStatsParams{})
	if err != nil || stats.CreditNoteCount != 2 {
		t.Fatalf("credit note stats = %+v, %v; want two notes", stats, err)
	}
}

// addLine adds a line to the bill with no catalog product, and returns its id.
func addLine(t *testing.T, db *sql.DB, queries *gen.Queries, billID uint64, name, price, quantity string) int32 {
	t.Helper()
	if err := queries.AddProductToBill(context.Background(), gen.AddProductToBillParams{
		Name:     strPtr(name),
		Price:    decimal.RequireFromString(price),
		Quantity: decimal.RequireFromString(quantity),
		PartName: strPtr(""),
		BillID:   billID,
	}); err != nil {
		t.Fatalf("add line %s: %v", name, err)
	}
	var id int32
	scanOne(t, db, &id, "SELECT MAX(id) FROM bill_product WHERE bill_id = ?", billID)
	return id
}

// insertCreditNote writes a credit note on the bill, with its identity and its own invoice UUID, and returns its id.
func insertCreditNote(t *testing.T, queries *gen.Queries, billID uint64, requestID, invoiceUUID, reason, amount string) int64 {
	t.Helper()
	res, err := queries.InsertCreditNote(context.Background(), gen.InsertCreditNoteParams{
		BillID:                    billID,
		State:                     int32Ptr(1),
		Note:                      strPtr(reason),
		Amount:                    decimal.RequireFromString(amount),
		LinesTotal:                decimal.RequireFromString(amount),
		InvoiceUuid:               sql.NullString{String: invoiceUUID, Valid: true},
		PosID:                     strPtr("pos-1"),
		BillRequestID:             sql.NullString{String: requestID, Valid: true},
		RequestFingerprint:        sql.NullString{String: "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210", Valid: true},
		RequestFingerprintVersion: 1,
	})
	if err != nil {
		t.Fatalf("insert credit note: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// execInsertLine writes one line of a credit note, for a bill line of quantity and price.
func execInsertLine(t *testing.T, queries *gen.Queries, note int64, billProduct int32, quantity, price string) {
	t.Helper()
	if err := queries.InsertCreditNoteLine(context.Background(), gen.InsertCreditNoteLineParams{
		CreditNoteID:  uint64(note),
		BillProductID: billProduct,
		Name:          strPtr("Brake pad"),
		Quantity:      decimal.RequireFromString(quantity),
		Price:         decimal.RequireFromString(price),
		Discount:      decimal.Zero,
		Vat:           decimal.RequireFromString("15.00"),
	}); err != nil {
		t.Fatalf("insert credit line: %v", err)
	}
}

// insertDebitNote writes a debit note on the bill, with its identity, and returns its id.
func insertDebitNote(t *testing.T, queries *gen.Queries, billID uint64, requestID string) int64 {
	t.Helper()
	res, err := queries.InsertDebitNote(context.Background(), gen.InsertDebitNoteParams{
		BillID:                    billID,
		State:                     int32Ptr(1),
		Note:                      strPtr("extra service"),
		Amount:                    decimal.RequireFromString("5.75"),
		LinesTotal:                decimal.RequireFromString("5.75"),
		InvoiceUuid:               sql.NullString{String: "6f7a8b9c-0d1e-4f2a-8b3c-4d5e6f7a8b9c", Valid: true},
		PosID:                     strPtr("pos-1"),
		BillRequestID:             sql.NullString{String: requestID, Valid: true},
		RequestFingerprint:        sql.NullString{String: "0a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f9", Valid: true},
		RequestFingerprintVersion: 1,
	})
	if err != nil {
		t.Fatalf("insert debit note: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func countBill(rows []gen.GetAllBillRow, billID uint64) int {
	n := 0
	for _, row := range rows {
		if row.ID == billID {
			n++
		}
	}
	return n
}

func countRecent(rows []gen.GetDashboardRecentInvoicesRow, billID uint64) int {
	n := 0
	for _, row := range rows {
		if row.ID == billID {
			n++
		}
	}
	return n
}

func int32Ptr(v int32) *int32 {
	return &v
}
