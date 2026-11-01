package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"ifritah/web-service-gin/pkg/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m, "outbox_test"))
}

const (
	eventA = "1d2b3c4d-0000-4000-8000-00000000000a"
	eventB = "1d2b3c4d-0000-4000-8000-00000000000b"
	eventC = "1d2b3c4d-0000-4000-8000-00000000000c"
)

// fakePublisher records each message and fails for the event IDs it is told to fail.
type fakePublisher struct {
	mu     sync.Mutex
	failOn map[string]bool
	sent   []sentMessage
}

type sentMessage struct {
	subject string
	data    string
	msgID   string
}

func (f *fakePublisher) Publish(subject string, data []byte, msgID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn[msgID] {
		return errors.New("nats unavailable")
	}
	f.sent = append(f.sent, sentMessage{subject: subject, data: string(data), msgID: msgID})
	return nil
}

func (f *fakePublisher) msgIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, 0, len(f.sent))
	for _, m := range f.sent {
		ids = append(ids, m.msgID)
	}
	return ids
}

func testEvent(eventID string) Event {
	return Event{
		EventID:     eventID,
		DocumentKey: "acme:7:bill:42",
		Subject:     "zatca.bill.acme.7",
		Payload:     json.RawMessage(`{"doc_type":"bill","id":42}`),
	}
}

// appendCommitted writes the events in one committed transaction.
func appendCommitted(t *testing.T, db *sql.DB, events ...Event) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if err := Append(ctx, tx, e); err != nil {
			tx.Rollback()
			t.Fatalf("append %s: %v", e.EventID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func testDrainer(db *sql.DB, pub Publisher, now time.Time) *Drainer {
	return &Drainer{
		DB:          db,
		Publisher:   pub,
		Owner:       "worker-1",
		BatchSize:   10,
		Lease:       time.Minute,
		MaxAttempts: 3,
		Now:         func() time.Time { return now },
	}
}

// drainClock returns a time an hour after now. Append stamps each event with the current time,
// and the drainer publishes only due events, so the drainer's clock must come after the appends.
// The hour is a margin for MySQL rounding DATETIME to whole seconds.
func drainClock() time.Time {
	return time.Now().UTC().Add(time.Hour)
}

// sameJSON reports whether two JSON texts hold the same value. MySQL returns JSON in its own
// spacing, so the text can differ from what was appended.
func sameJSON(t *testing.T, a, b string) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal([]byte(a), &x); err != nil {
		t.Fatalf("%q is not JSON: %v", a, err)
	}
	if err := json.Unmarshal([]byte(b), &y); err != nil {
		t.Fatalf("%q is not JSON: %v", b, err)
	}
	return reflect.DeepEqual(x, y)
}

// row returns the status, attempt count and last error of one event.
func row(t *testing.T, db *sql.DB, eventID string) (status string, attempts int, lastError string) {
	t.Helper()
	if err := db.QueryRow("SELECT status, attempt_count, COALESCE(last_error, '') FROM outbox WHERE event_id = ?", eventID).
		Scan(&status, &attempts, &lastError); err != nil {
		t.Fatal(err)
	}
	return status, attempts, lastError
}

func countWhere(t *testing.T, db *sql.DB, where string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM outbox WHERE " + where).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAppendIsRolledBackWithItsTransaction(t *testing.T) {
	db := testdb.Open(t, "outbox")
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Append(ctx, tx, testEvent(eventA)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if n := countWhere(t, db, "1 = 1"); n != 0 {
		t.Fatalf("an event from a rolled-back transaction is visible: %d rows", n)
	}
}

func TestDrainOncePublishesCommittedEventsOnce(t *testing.T) {
	db := testdb.Open(t, "outbox")
	appendCommitted(t, db, testEvent(eventA), testEvent(eventB))
	pub := &fakePublisher{}
	d := testDrainer(db, pub, drainClock())

	n, err := d.DrainOnce(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("DrainOnce = %d, %v; want 2, nil", n, err)
	}
	if ids := pub.msgIDs(); len(ids) != 2 || ids[0] != eventA || ids[1] != eventB {
		t.Fatalf("published %v, want [%s %s] in order", ids, eventA, eventB)
	}
	if got := pub.sent[0]; got.subject != "zatca.bill.acme.7" || !sameJSON(t, got.data, `{"doc_type":"bill","id":42}`) {
		t.Fatalf("published %+v, want the event subject and payload", got)
	}
	if n := countWhere(t, db, "status = 'published'"); n != 2 {
		t.Fatalf("%d events are published in the table, want 2", n)
	}

	n, err = d.DrainOnce(context.Background())
	if err != nil || n != 0 || len(pub.msgIDs()) != 2 {
		t.Fatalf("second DrainOnce = %d, %v, sent %v; want nothing new", n, err, pub.msgIDs())
	}
}

func TestFailedPublishIsRetriedThenDeadLettered(t *testing.T) {
	db := testdb.Open(t, "outbox")
	appendCommitted(t, db, testEvent(eventA))
	base := drainClock()
	pub := &fakePublisher{failOn: map[string]bool{eventA: true}}
	d := testDrainer(db, pub, base)
	d.MaxAttempts = 2
	ctx := context.Background()

	if n, err := d.DrainOnce(ctx); err != nil || n != 0 {
		t.Fatalf("first DrainOnce = %d, %v; want 0, nil", n, err)
	}
	if status, attempts, lastError := row(t, db, eventA); status != "pending" || attempts != 1 || lastError == "" {
		t.Fatalf("after one failure: status %q, attempts %d, last error %q", status, attempts, lastError)
	}

	d.Now = func() time.Time { return base.Add(10 * time.Minute) }
	if n, err := d.DrainOnce(ctx); err != nil || n != 0 {
		t.Fatalf("second DrainOnce = %d, %v; want 0, nil", n, err)
	}
	if status, attempts, _ := row(t, db, eventA); status != "dead_letter" || attempts != 2 {
		t.Fatalf("after max attempts: status %q, attempts %d; want dead_letter, 2", status, attempts)
	}

	d.Now = func() time.Time { return base.Add(20 * time.Minute) }
	if n, err := d.DrainOnce(ctx); err != nil || n != 0 {
		t.Fatalf("DrainOnce after dead letter = %d, %v; want 0, nil", n, err)
	}
}

func TestExpiredLeaseIsReclaimedAndLiveLeaseIsNot(t *testing.T) {
	db := testdb.Open(t, "outbox")
	appendCommitted(t, db, testEvent(eventA), testEvent(eventB))
	base := drainClock()
	// A worker that crashed while holding eventA, and a worker that still holds eventB.
	for _, lease := range []struct {
		eventID string
		owner   string
		expires time.Time
	}{
		{eventA, "crashed", base.Add(-time.Minute)},
		{eventB, "busy", base.Add(time.Minute)},
	} {
		if _, err := db.Exec("UPDATE outbox SET status = 'in_flight', lease_owner = ?, lease_expires_at = ?, attempt_count = 1 WHERE event_id = ?",
			lease.owner, lease.expires, lease.eventID); err != nil {
			t.Fatal(err)
		}
	}
	pub := &fakePublisher{}
	n, err := testDrainer(db, pub, base).DrainOnce(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("DrainOnce = %d, %v; want 1, nil", n, err)
	}
	if ids := pub.msgIDs(); len(ids) != 1 || ids[0] != eventA {
		t.Fatalf("published %v, want only the event with the expired lease", ids)
	}
	if status, _, _ := row(t, db, eventB); status != "in_flight" {
		t.Fatalf("the event with a live lease is %q, want in_flight", status)
	}
}

func TestAppendRejectsMalformedEvents(t *testing.T) {
	valid := testEvent(eventA)
	cases := map[string]Event{
		"upper-case event id":         {EventID: strings.ToUpper(eventA), DocumentKey: valid.DocumentKey, Subject: valid.Subject, Payload: valid.Payload},
		"event id that is not a UUID": {EventID: "evt-1", DocumentKey: valid.DocumentKey, Subject: valid.Subject, Payload: valid.Payload},
		"missing document key":        {EventID: eventA, Subject: valid.Subject, Payload: valid.Payload},
		"missing subject":             {EventID: eventA, DocumentKey: valid.DocumentKey, Payload: valid.Payload},
		"payload that is not JSON":    {EventID: eventA, DocumentKey: valid.DocumentKey, Subject: valid.Subject, Payload: json.RawMessage("{")},
	}
	for name, e := range cases {
		// Append checks the event before it touches the transaction, so a nil transaction is safe here.
		if err := Append(context.Background(), nil, e); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestBackoffDoublesAndStopsAtFiveMinutes(t *testing.T) {
	want := map[uint32]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second, 9: 256 * time.Second, 10: 5 * time.Minute, 50: 5 * time.Minute}
	for attempts, wait := range want {
		if got := backoff(attempts); got != wait {
			t.Fatalf("backoff(%d) = %v, want %v", attempts, got, wait)
		}
	}
}

// loadBacklog stores published events and then due pending events, so the due events are the newest
// rows, as they are after NATS has been down for a while. The due events are older than the clock
// that the tests drain with.
func loadBacklog(t *testing.T, db *sql.DB, due, published int) {
	t.Helper()
	at := time.Now().UTC().Add(-time.Minute)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < due+published; i++ {
		status := "published"
		if i >= published {
			status = "pending"
		}
		if _, err := tx.Exec("INSERT INTO outbox (event_id, document_key, subject, payload, status, next_attempt_at) VALUES (?, ?, 'zatca.bill.acme.1', '{}', ?, ?)",
			fmt.Sprintf("1d2b3c4d-0001-4000-8000-%012d", i), fmt.Sprintf("acme:1:bill:%d", i), status, at); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// recordLocks counts the record locks that open transactions hold on the outbox table.
func recordLocks(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM performance_schema.data_locks WHERE OBJECT_SCHEMA = DATABASE() AND OBJECT_NAME = 'outbox' AND LOCK_TYPE = 'RECORD'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// appendFromOtherConnection writes e on its own connection, with a short lock wait. An insert that
// the claim blocks then fails the test, instead of waiting for the server's 50 second default.
func appendFromOtherConnection(db *sql.DB, e Event) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SET SESSION innodb_lock_wait_timeout = 2"); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, "SET SESSION innodb_lock_wait_timeout = DEFAULT")
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := Append(ctx, tx, e); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// TestSaleInsertIsNotBlockedByAClaim checks that an event written while a claim is open goes through
// at once. Under REPEATABLE READ the claim locked the gap after its last due event, so the insert of
// a sale's event had to wait for the claim to finish.
func TestSaleInsertIsNotBlockedByAClaim(t *testing.T) {
	db := testdb.Open(t, "outbox")
	appendCommitted(t, db, testEvent(eventA))
	loadBacklog(t, db, 0, 1)
	d := testDrainer(db, &fakePublisher{}, drainClock())
	var insertErr error
	d.afterLease = func(*sql.Tx) {
		insertErr = appendFromOtherConnection(db, testEvent(eventB))
	}

	if n, err := d.DrainOnce(context.Background()); err != nil || n != 1 {
		t.Fatalf("DrainOnce = %d, %v; want 1, nil", n, err)
	}
	if insertErr != nil {
		t.Fatalf("an insert while the claim was open failed: %v", insertErr)
	}
}

// TestClaimLocksOnlyItsBatchUnderBacklog checks how many locks a claim holds while a backlog waits.
// The claim must lock about its batch, not every row that it reads on the way. Under REPEATABLE READ
// the claim used to lock every published event it walked past, which is thousands of locks with a
// backlog.
func TestClaimLocksOnlyItsBatchUnderBacklog(t *testing.T) {
	db := testdb.Open(t, "outbox")
	loadBacklog(t, db, 200, 300)
	d := testDrainer(db, &fakePublisher{}, drainClock())
	var locks int
	d.afterLease = func(*sql.Tx) {
		locks = recordLocks(t, db)
	}

	n, err := d.DrainOnce(context.Background())
	if err != nil || n != int(d.BatchSize) {
		t.Fatalf("DrainOnce = %d, %v; want %d, nil", n, err, d.BatchSize)
	}
	t.Logf("record locks held by a claim of %d events, with 200 due and 300 published: %d", d.BatchSize, locks)
	// A claimed event is locked twice: its entry in the index the claim reads, and its row. The limit
	// allows twice that, so a small change to the claim passes, while a scan of the backlog fails.
	if limit := 4 * int(d.BatchSize); locks > limit {
		t.Fatalf("the claim holds %d record locks; want at most %d for a batch of %d", locks, limit, d.BatchSize)
	}
}

// TestMixedBatchMarksSuccessesAndRetriesTheFailure checks that one failed publish in a batch does not
// hold back the events published before and after it.
func TestMixedBatchMarksSuccessesAndRetriesTheFailure(t *testing.T) {
	db := testdb.Open(t, "outbox")
	appendCommitted(t, db, testEvent(eventA), testEvent(eventB), testEvent(eventC))
	pub := &fakePublisher{failOn: map[string]bool{eventB: true}}

	n, err := testDrainer(db, pub, drainClock()).DrainOnce(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("DrainOnce = %d, %v; want 2, nil", n, err)
	}
	if ids := pub.msgIDs(); len(ids) != 2 || ids[0] != eventA || ids[1] != eventC {
		t.Fatalf("published %v, want [%s %s]", ids, eventA, eventC)
	}
	for _, id := range []string{eventA, eventC} {
		if status, _, _ := row(t, db, id); status != "published" {
			t.Fatalf("event %s is %q, want published", id, status)
		}
	}
	if status, attempts, lastError := row(t, db, eventB); status != "pending" || attempts != 1 || lastError == "" {
		t.Fatalf("failed event: status %q, attempts %d, last error %q; want pending, 1, and an error", status, attempts, lastError)
	}
}
