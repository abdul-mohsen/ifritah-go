package handlers

import (
	"context"
	"sync"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetNextSequenceNumberUsesBranchScope(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	mock.ExpectExec("INSERT INTO branch_sequence").
		WithArgs(uint32(7), "bill").
		WillReturnResult(sqlmock.NewResult(42, 1))

	got, err := getNextSequenceNumber(context.Background(), db.New(sqlDB), 7)
	if err != nil {
		t.Fatalf("getNextSequenceNumber: %v", err)
	}
	if got != 42 {
		t.Fatalf("sequence = %d, want 42", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("branch allocator expectations: %v", err)
	}
}

func TestGetNextSequenceNumberConcurrentBranchesRemainIndependent(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()
	mock.MatchExpectationsInOrder(false)

	const allocationsPerBranch = 6
	for branch := uint32(7); branch <= 8; branch++ {
		for sequence := int64(1); sequence <= allocationsPerBranch; sequence++ {
			mock.ExpectExec("INSERT INTO branch_sequence").
				WithArgs(branch, "bill").
				WillReturnResult(sqlmock.NewResult(sequence, 1))
		}
	}

	type result struct {
		branch   uint32
		sequence uint64
		err      error
	}
	results := make(chan result, allocationsPerBranch*2)
	var wg sync.WaitGroup
	for branch := uint32(7); branch <= 8; branch++ {
		for i := 0; i < allocationsPerBranch; i++ {
			wg.Add(1)
			go func(branch uint32) {
				defer wg.Done()
				sequence, err := getNextSequenceNumber(context.Background(), db.New(sqlDB), branch)
				results <- result{branch: branch, sequence: sequence, err: err}
			}(branch)
		}
	}
	wg.Wait()
	close(results)

	seen := map[uint32]map[uint64]bool{
		7: {},
		8: {},
	}
	for item := range results {
		if item.err != nil {
			t.Fatalf("branch %d allocation: %v", item.branch, item.err)
		}
		if seen[item.branch][item.sequence] {
			t.Fatalf("branch %d allocated duplicate sequence %d", item.branch, item.sequence)
		}
		seen[item.branch][item.sequence] = true
	}
	for branch := uint32(7); branch <= 8; branch++ {
		if len(seen[branch]) != allocationsPerBranch {
			t.Fatalf("branch %d allocations = %d, want %d", branch, len(seen[branch]), allocationsPerBranch)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("concurrent branch allocator expectations: %v", err)
	}
}
