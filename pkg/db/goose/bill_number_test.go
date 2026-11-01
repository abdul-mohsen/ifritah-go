package goosemigrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	gen "ifritah/web-service-gin/pkg/db/gen"
)

// TestBillNumberCounter checks 00008 and the counter that a sale uses. A branch's counter starts at its highest bill
// number, a branch without bills starts at 0, so its first number is 1, and Down removes the bill counters. It needs a
// MySQL server.
func TestBillNumberCounter(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_bill_number")
	if _, err := provider.UpTo(ctx, 7); err != nil {
		t.Fatalf("goose up to 00007: %v", err)
	}
	seedBillNumberFixture(t, db)
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up to 00008: %v", err)
	}

	queries := gen.New(db)
	for _, tc := range []struct {
		branch uint32
		want   uint64
	}{{1, 8}, {2, 6}, {3, 1}, {3, 2}} {
		if got := nextBillNumber(t, queries, tc.branch); got != tc.want {
			t.Fatalf("branch %d allocated %d; want %d", tc.branch, got, tc.want)
		}
	}

	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("goose down 00008: %v", err)
	}
	var remaining int
	scanOne(t, db, &remaining, "SELECT COUNT(*) FROM branch_sequence WHERE scope = 'bill'")
	if remaining != 0 {
		t.Fatalf("after down, %d bill counters remain; want 0", remaining)
	}
}

// seedBillNumberFixture gives branch 1 bills 3 and 7, branch 2 bill 5, and branch 3 none.
func seedBillNumberFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	companyID := execInsert(t, db, "INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')")
	for id := 1; id <= 3; id++ {
		execInsert(t, db, "INSERT INTO branches (id, name, company_id) VALUES (?, ?, ?)", id, fmt.Sprintf("branch %d", id), companyID)
	}
	for _, bill := range [][2]int{{1, 3}, {1, 7}, {2, 5}} {
		execInsert(t, db, "INSERT INTO bill (discount, store_id, merchant_id, maintenance_cost, branch_id, sequence_number) VALUES (0, 1, 1, 0, ?, ?)", bill[0], bill[1])
	}
}

// nextBillNumber takes the next number of a branch's bill series, as a sale does.
func nextBillNumber(t *testing.T, queries *gen.Queries, branch uint32) uint64 {
	t.Helper()
	res, err := queries.AllocateBranchSequence(context.Background(), gen.AllocateBranchSequenceParams{BranchID: branch, Scope: "bill"})
	if err != nil {
		t.Fatalf("allocate for branch %d: %v", branch, err)
	}
	number, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("number for branch %d: %v", branch, err)
	}
	return uint64(number)
}
