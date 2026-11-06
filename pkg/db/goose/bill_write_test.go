package goosemigrations

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"

	gen "ifritah/web-service-gin/pkg/db/gen"
)

// TestSaleWritesMatchTheSchema runs a sale's two writes against the migrated schema: the bill row with only the columns it needs,
// then the update that writes its fields, number, identity and request fingerprint. It then reads the sale back by its request
// identity, the way a retry does, and checks that a second bill cannot take the same identity. The sqlmock tests in pkg/handlers
// check the statement order. This checks that the statements fit the columns, their NOT NULL rules, and the unique key. It needs
// a MySQL server.
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
	const posID = "pos-1"
	const requestID = "3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c"
	const fingerprint = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	billID := createBillRow(t, queries)
	if err := writeSaleFields(queries, billID, 1, posID, requestID, fingerprint); err != nil {
		t.Fatalf("update bill row: %v", err)
	}

	var state int32
	var seq uint64
	var gotPos, gotRequest, gotInvoice, gotFingerprint sql.NullString
	var gotVersion uint16
	if err := db.QueryRow(
		"SELECT state, sequence_number, pos_id, bill_request_id, invoice_uuid, request_fingerprint, request_fingerprint_version FROM bill WHERE id = ?",
		billID).Scan(&state, &seq, &gotPos, &gotRequest, &gotInvoice, &gotFingerprint, &gotVersion); err != nil {
		t.Fatalf("read bill row: %v", err)
	}
	if state != 1 || seq != 1 || gotPos.String != posID || gotRequest.String != requestID || !gotInvoice.Valid ||
		gotFingerprint.String != fingerprint || gotVersion != 1 {
		t.Fatalf("bill row = state %d, number %d, pos %q, request %q, invoice %v, fingerprint %q, version %d; want the sale's values",
			state, seq, gotPos.String, gotRequest.String, gotInvoice.Valid, gotFingerprint.String, gotVersion)
	}

	found, err := queries.GetBillByRequestIdentity(ctx, gen.GetBillByRequestIdentityParams{
		PosID:         strPtr(posID),
		BillRequestID: sql.NullString{String: requestID, Valid: true},
	})
	if err != nil || found.ID != billID || found.RequestFingerprint.String != fingerprint || found.RequestFingerprintVersion != 1 {
		t.Fatalf("lookup by request identity = %+v, %v; want bill %d with its fingerprint", found, err, billID)
	}

	// A second bill on the same branch, with the next number, but the same request identity, must fail on that identity.
	second := createBillRow(t, queries)
	err = writeSaleFields(queries, second, 2, posID, requestID, fingerprint)
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 || !strings.Contains(mysqlErr.Message, "uq_bill_pos_request") {
		t.Fatalf("a second bill with the same request identity = %v; want a duplicate on uq_bill_pos_request", err)
	}
}

// createBillRow runs the first write of a sale, the bill row with only its required columns, and returns the new bill id.
func createBillRow(t *testing.T, queries *gen.Queries) uint64 {
	t.Helper()
	res, err := queries.CreateBill(context.Background(), gen.CreateBillParams{
		EffectiveDate:   time.Now(),
		StoreID:         1,
		MerchantID:      1,
		Discount:        decimal.Zero,
		MaintenanceCost: decimal.Zero,
	})
	if err != nil {
		t.Fatalf("create bill row: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(id)
}

// writeSaleFields runs the second write of a sale: the fields, number, and identity of a final sale on branch 7.
func writeSaleFields(queries *gen.Queries, billID, number uint64, posID, requestID, fingerprint string) error {
	branch := uint32(7)
	return queries.UpdateBillByID(context.Background(), gen.UpdateBillByIDParams{
		ID:                        billID,
		EffectiveDate:             time.Now(),
		State:                     1,
		Discount:                  decimal.Zero,
		StoreID:                   1,
		SequenceNumber:            &number,
		MerchantID:                1,
		MaintenanceCost:           decimal.Zero,
		PaymentMethod:             10,
		BranchID:                  &branch,
		PosID:                     strPtr(posID),
		BillRequestID:             sql.NullString{String: requestID, Valid: true},
		InvoiceUuid:               sql.NullString{String: "7c9e6679-7425-40de-944b-e07fc1f90ae7", Valid: true},
		RequestFingerprint:        sql.NullString{String: fingerprint, Valid: true},
		RequestFingerprintVersion: 1,
	})
}

func strPtr(s string) *string {
	return &s
}
