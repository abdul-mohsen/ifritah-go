package egschain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"ifritah/web-service-gin/pkg/testdb"
)

// t0 is the clock for these tests. Leases are compared with it in Go, so no database clock is used.
var t0 = time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)

// egschainTables are the tables that the EGS tests write, children before parents.
var egschainTables = []string{"branch_zatca_config", "branches", "company"}

const committedHash = "hash-1"

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m, "egschain_test"))
}

// newBranch adds a branch to company 1. The test database starts with no company, so the helper
// creates it first.
func newBranch(t *testing.T, db *sql.DB, name string) uint32 {
	t.Helper()
	if _, err := db.Exec("INSERT IGNORE INTO company (id, name, vat_number) VALUES (1, 'test company', '')"); err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec("INSERT INTO branches (name, company_id) VALUES (?, 1)", name)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint32(id)
}

func newBranchWithEGS(t *testing.T, db *sql.DB, name string) uint32 {
	t.Helper()
	branchID := newBranch(t, db, name)
	if _, err := db.Exec("INSERT INTO branch_zatca_config (branch_id) VALUES (?)", branchID); err != nil {
		t.Fatal(err)
	}
	return branchID
}

// inTx runs fn in its own transaction, the way the daemon runs each step, and commits it when fn
// succeeds.
func inTx(db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func reserve(db *sql.DB, branchID uint32, key string, now time.Time) (Reservation, error) {
	var r Reservation
	err := inTx(db, func(tx *sql.Tx) error {
		var err error
		r, err = Reserve(context.Background(), tx, branchID, key, now, time.Minute)
		return err
	})
	return r, err
}

func commit(db *sql.DB, r Reservation, hash string) error {
	return inTx(db, func(tx *sql.Tx) error {
		return Commit(context.Background(), tx, r, hash)
	})
}

func release(db *sql.DB, r Reservation) error {
	return inTx(db, func(tx *sql.Tx) error {
		return Release(context.Background(), tx, r)
	})
}

// chainState reads the committed ICV and previous hash of a branch, and whether a reservation is
// held.
func chainState(t *testing.T, db *sql.DB, branchID uint32) (last uint64, previous string, reserved bool) {
	t.Helper()
	var prev, key sql.NullString
	if err := db.QueryRow("SELECT last_icv, previous_invoice_hash, icv_reservation_key FROM branch_zatca_config WHERE branch_id = ?", branchID).
		Scan(&last, &prev, &key); err != nil {
		t.Fatal(err)
	}
	return last, prev.String, key.Valid
}

func TestFirstDocumentStartsTheChain(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "first")

	r, err := reserve(db, branchID, "acme:first:bill:1", t0)
	if err != nil || r.ICV != 1 || r.PreviousHash != "" {
		t.Fatalf("first reservation = %+v, %v; want ICV 1 with no previous hash", r, err)
	}
	if err := commit(db, r, committedHash); err != nil {
		t.Fatalf("commit: %v", err)
	}
	last, previous, reserved := chainState(t, db, branchID)
	if last != 1 || previous != committedHash || reserved {
		t.Fatalf("after commit: ICV %d, previous %q, reserved %v; want 1, hash-1, false", last, previous, reserved)
	}
}

func TestNextDocumentChainsOnTheCommittedHash(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "next")

	first, err := reserve(db, branchID, "acme:next:bill:1", t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := commit(db, first, committedHash); err != nil {
		t.Fatal(err)
	}
	second, err := reserve(db, branchID, "acme:next:bill:2", t0)
	if err != nil || second.ICV != 2 || second.PreviousHash != committedHash {
		t.Fatalf("second reservation = %+v, %v; want ICV 2 after hash-1", second, err)
	}
}

func TestSameDocumentGetsItsLiveReservationBack(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "same")

	first, err := reserve(db, branchID, "acme:same:bill:1", t0)
	if err != nil {
		t.Fatal(err)
	}
	again, err := reserve(db, branchID, "acme:same:bill:1", t0.Add(10*time.Second))
	if err != nil || again.Token != first.Token || again.ICV != first.ICV {
		t.Fatalf("second call = %+v, %v; want the same token and ICV %d", again, err, first.ICV)
	}
	if last, _, _ := chainState(t, db, branchID); last != 0 {
		t.Fatalf("committed ICV = %d before any commit; want 0", last)
	}
}

func TestOtherDocumentWaitsForTheChain(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "wait")

	if _, err := reserve(db, branchID, "acme:wait:bill:1", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := reserve(db, branchID, "acme:wait:bill:2", t0.Add(10*time.Second)); !errors.Is(err, ErrBusy) {
		t.Fatalf("other document while the lease is live: %v; want ErrBusy", err)
	}
	// Once the first lease has expired, the second document still waits. Its ICV would be the
	// first document's ICV.
	if _, err := reserve(db, branchID, "acme:wait:bill:2", t0.Add(time.Hour)); !errors.Is(err, ErrBusy) {
		t.Fatalf("other document after the lease expired: %v; want ErrBusy", err)
	}
}

func TestExpiredReservationIsTakenOverByItsOwnDocumentOnly(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "takeover")

	old, err := reserve(db, branchID, "acme:takeover:bill:1", t0)
	if err != nil {
		t.Fatal(err)
	}
	taken, err := reserve(db, branchID, "acme:takeover:bill:1", t0.Add(time.Hour))
	if err != nil || taken.ICV != old.ICV || taken.Token == old.Token {
		t.Fatalf("takeover = %+v, %v; want ICV %d with a new token", taken, err, old.ICV)
	}
	if err := commit(db, old, "stale"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("commit by the previous owner: %v; want ErrNotOwner", err)
	}
	if err := commit(db, taken, committedHash); err != nil {
		t.Fatalf("commit by the new owner: %v", err)
	}
}

func TestReleaseKeepsTheICVForTheNextDocument(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "release")

	r, err := reserve(db, branchID, "acme:release:bill:1", t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := release(db, r); err != nil {
		t.Fatalf("release: %v", err)
	}
	if last, _, reserved := chainState(t, db, branchID); last != 0 || reserved {
		t.Fatalf("after release: ICV %d, reserved %v; want 0, false", last, reserved)
	}
	next, err := reserve(db, branchID, "acme:release:bill:2", t0)
	if err != nil || next.ICV != 1 {
		t.Fatalf("reservation after release = %+v, %v; want ICV 1", next, err)
	}
}

func TestCommitRefusesAStaleOrRepeatedOwner(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "stale")

	r, err := reserve(db, branchID, "acme:stale:bill:1", t0)
	if err != nil {
		t.Fatal(err)
	}
	wrongToken := r
	wrongToken.Token = "not-the-owner"
	wrongKey := r
	wrongKey.DocumentKey = "acme:stale:bill:9"
	for name, bad := range map[string]Reservation{"token": wrongToken, "key": wrongKey} {
		if err := commit(db, bad, "hash"); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("commit with a wrong %s: %v; want ErrNotOwner", name, err)
		}
	}
	if err := commit(db, r, committedHash); err != nil {
		t.Fatalf("commit by the owner: %v", err)
	}
	if err := commit(db, r, "hash-2"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("second commit: %v; want ErrNotOwner", err)
	}
	if err := release(db, r); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("release after commit: %v; want ErrNotOwner", err)
	}
	if last, previous, _ := chainState(t, db, branchID); last != 1 || previous != committedHash {
		t.Fatalf("chain after refused writes: ICV %d, previous %q; want 1, hash-1", last, previous)
	}
}

func TestBranchWithoutEGSIsRejected(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranch(t, db, "no-egs")

	if _, err := reserve(db, branchID, "acme:no-egs:bill:1", t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reserve for a branch without an EGS: %v; want ErrNotFound", err)
	}
}

func TestBranchesHaveIndependentChains(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	a := newBranchWithEGS(t, db, "branch-a")
	b := newBranchWithEGS(t, db, "branch-b")

	ra, err := reserve(db, a, "acme:branch-a:bill:1", t0)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := reserve(db, b, "acme:branch-b:bill:1", t0)
	if err != nil || ra.ICV != 1 || rb.ICV != 1 {
		t.Fatalf("reservations = ICV %d and %d, %v; want 1 and 1", ra.ICV, rb.ICV, err)
	}
}

func TestInvalidInputIsRefused(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "invalid")

	if _, err := reserve(db, branchID, "", t0); err == nil {
		t.Fatal("an empty document key was accepted")
	}
	if _, err := reserve(db, branchID, strings.Repeat("k", maxKeyLen+1), t0); err == nil {
		t.Fatal("a document key longer than the column was accepted")
	}
	if _, err := reserve(db, 0, "acme:invalid:bill:1", t0); err == nil {
		t.Fatal("branch 0 was accepted")
	}
	r, err := reserve(db, branchID, "acme:invalid:bill:1", t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := commit(db, r, ""); err == nil || errors.Is(err, ErrNotOwner) {
		t.Fatalf("an empty signed hash: %v; want a validation error", err)
	}
}

func TestReservationColumnsAreSetTogether(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "together")

	_, err := db.Exec("UPDATE branch_zatca_config SET icv_reservation_key = 'acme:together:bill:1' WHERE branch_id = ?", branchID)
	if err == nil || !strings.Contains(err.Error(), "chk_bzc_icv_reservation") {
		t.Fatalf("a reservation key without a token was accepted: %v", err)
	}
}

// TestReserveDoesNotQueueBehindALockedBranch checks that Reserve fails at once when another
// transaction holds the branch row. A caller that waited would hold a pool connection until the
// server's lock wait timeout, which is 50 seconds by default.
func TestReserveDoesNotQueueBehindALockedBranch(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "locked")

	holder, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback()
	if _, err := holder.Exec("SELECT branch_id FROM branch_zatca_config WHERE branch_id = ? FOR UPDATE", branchID); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, err = reserve(db, branchID, "acme:locked:bill:1", t0)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("reserve while another transaction holds the row: %v; want ErrBusy", err)
	}
	if waited := time.Since(start); waited > 5*time.Second {
		t.Fatalf("reserve took %s; it should fail at once", waited)
	}
}

// TestConcurrentDocumentsGetOneOwnerAtATime starts ten documents at once for the same EGS. One of
// them must get the reservation, and the others must be refused. Each round commits the owner, so
// the ICVs of the three rounds are 1, 2 and 3 with no repeats.
func TestConcurrentDocumentsGetOneOwnerAtATime(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "race")
	const documents = 10

	for round := 1; round <= 3; round++ {
		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			owners  []Reservation
			refused int
		)
		for i := 0; i < documents; i++ {
			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				r, err := reserve(db, branchID, key, t0)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					owners = append(owners, r)
				case errors.Is(err, ErrBusy):
					refused++
				default:
					t.Errorf("reserve %s: %v", key, err)
				}
			}(fmt.Sprintf("acme:race:bill:%d-%d", round, i))
		}
		wg.Wait()

		if len(owners) != 1 || refused != documents-1 {
			t.Fatalf("round %d: %d owners and %d refusals; want 1 and %d", round, len(owners), refused, documents-1)
		}
		if owners[0].ICV != uint64(round) {
			t.Fatalf("round %d: ICV %d; want %d", round, owners[0].ICV, round)
		}
		if err := commit(db, owners[0], fmt.Sprintf("hash-%d", round)); err != nil {
			t.Fatalf("round %d commit: %v", round, err)
		}
	}
}

// TestConcurrentCallsForOneDocumentShareOneReservation starts ten calls for one document at once.
// A call that finds the branch row locked fails with ErrBusy at once, so it may not get the
// reservation. Every call that gets one must get the same reservation, and a call made afterwards
// gets it too.
func TestConcurrentCallsForOneDocumentShareOneReservation(t *testing.T) {
	db := testdb.Open(t, egschainTables...)
	branchID := newBranchWithEGS(t, db, "same-race")
	const callers = 10

	results := make([]Reservation, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = reserve(db, branchID, "acme:same-race:bill:1", t0)
		}()
	}
	wg.Wait()

	shared, granted := sharedReservation(t, results, errs)
	if granted == 0 {
		t.Fatal("no call got the reservation")
	}
	again, err := reserve(db, branchID, "acme:same-race:bill:1", t0)
	if err != nil || again.Token != shared.Token || again.ICV != 1 {
		t.Fatalf("call after the race = %+v, %v; want the shared reservation", again, err)
	}
}

// sharedReservation checks that every call that got a reservation got the same one, with ICV 1,
// and that every other call failed with ErrBusy. It returns the shared reservation and how many
// calls got it.
func sharedReservation(t *testing.T, results []Reservation, errs []error) (Reservation, int) {
	t.Helper()
	var shared Reservation
	granted := 0
	for i, err := range errs {
		if err != nil {
			if !errors.Is(err, ErrBusy) {
				t.Fatalf("call %d: %v; want the reservation or ErrBusy", i, err)
			}
			continue
		}
		if granted == 0 {
			shared = results[i]
		}
		if results[i].Token != shared.Token || results[i].ICV != 1 {
			t.Fatalf("call %d = %+v; want the one shared reservation %+v", i, results[i], shared)
		}
		granted++
	}
	return shared, granted
}
