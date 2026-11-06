package goosemigrations

import (
	"context"
	"os"
	"testing"

	gen "ifritah/web-service-gin/pkg/db/gen"
)

// TestBillNumberCounter checks fresh-system allocation and that rolling back 00008 preserves counters written by sales.
// It needs a MySQL server.
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
	companyID := execInsert(t, db, "INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')")
	for id := 1; id <= 2; id++ {
		execInsert(t, db, "INSERT INTO branches (id, name, company_id) VALUES (?, ?, ?)", id, "branch", companyID)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up to 00008: %v", err)
	}

	queries := gen.New(db)
	for _, tc := range []struct {
		branch uint32
		want   uint64
	}{{1, 1}, {1, 2}, {2, 1}} {
		if got := nextBillNumber(t, queries, tc.branch); got != tc.want {
			t.Fatalf("branch %d allocated %d; want %d", tc.branch, got, tc.want)
		}
	}

	if _, err := provider.DownTo(ctx, 7); err != nil {
		t.Fatalf("goose down to 00007: %v", err)
	}
	var billCounters, branchOne, branchTwo int
	scanOne(t, db, &billCounters, "SELECT COUNT(*) FROM branch_sequence WHERE scope = 'bill'")
	scanOne(t, db, &branchOne, "SELECT `last_value` FROM branch_sequence WHERE branch_id = 1 AND scope = 'bill'")
	scanOne(t, db, &branchTwo, "SELECT `last_value` FROM branch_sequence WHERE branch_id = 2 AND scope = 'bill'")
	if billCounters != 2 || branchOne != 2 || branchTwo != 1 {
		t.Fatalf("after down, branch counters = bill %d, branch1 %d, branch2 %d; want bill 2, branch1 2, branch2 1",
			billCounters, branchOne, branchTwo)
	}
	if got := nextBillNumber(t, queries, 1); got != 3 {
		t.Fatalf("branch 1 allocated %d after down; want 3", got)
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
