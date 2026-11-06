package goosemigrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	gen "ifritah/web-service-gin/pkg/db/gen"
)

// TestBillNumberCounter checks 00008 and the counter that a sale uses. Existing lower counters are raised to the
// highest bill number, existing higher counters are retained, a branch without a numbered bill starts at 0, and Down
// preserves the counters. It needs a MySQL server.
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
	execInsert(t, db, "INSERT INTO branch_sequence (branch_id, scope, `last_value`) VALUES (1, 'bill', 6), (2, 'bill', 9), (1, 'stock', 42)")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up to 00008: %v", err)
	}

	queries := gen.New(db)
	for _, tc := range []struct {
		branch uint32
		want   uint64
	}{{1, 8}, {2, 10}, {3, 1}, {3, 2}, {4, 1}} {
		if got := nextBillNumber(t, queries, tc.branch); got != tc.want {
			t.Fatalf("branch %d allocated %d; want %d", tc.branch, got, tc.want)
		}
	}

	if _, err := provider.DownTo(ctx, 7); err != nil {
		t.Fatalf("goose down to 00007: %v", err)
	}
	var billCounters, otherScope, branchOne, branchTwo int
	scanOne(t, db, &billCounters, "SELECT COUNT(*) FROM branch_sequence WHERE scope = 'bill'")
	scanOne(t, db, &otherScope, "SELECT COUNT(*) FROM branch_sequence WHERE scope = 'stock'")
	scanOne(t, db, &branchOne, "SELECT `last_value` FROM branch_sequence WHERE branch_id = 1 AND scope = 'bill'")
	scanOne(t, db, &branchTwo, "SELECT `last_value` FROM branch_sequence WHERE branch_id = 2 AND scope = 'bill'")
	if billCounters != 4 || otherScope != 1 || branchOne != 8 || branchTwo != 10 {
		t.Fatalf("after down, branch counters = bill %d, other %d, branch1 %d, branch2 %d; want bill 4, other 1, branch1 8, branch2 10",
			billCounters, otherScope, branchOne, branchTwo)
	}
}

// seedBillNumberFixture gives branch 1 bills 3 and 7, branch 2 bill 5, and branch 3 none.
func seedBillNumberFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	companyID := execInsert(t, db, "INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')")
	for id := 1; id <= 4; id++ {
		execInsert(t, db, "INSERT INTO branches (id, name, company_id) VALUES (?, ?, ?)", id, fmt.Sprintf("branch %d", id), companyID)
	}
	for _, bill := range [][2]any{{1, 3}, {1, 7}, {2, 5}, {4, nil}} {
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
