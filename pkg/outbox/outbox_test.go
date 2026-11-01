package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"ifritah/web-service-gin/pkg/testdb"
)

const (
	eventA = "1d2b3c4d-0000-4000-8000-00000000000a"
	eventB = "1d2b3c4d-0000-4000-8000-00000000000b"
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

// newOutboxDB returns a fresh migrated database for one test.
func newOutboxDB(t *testing.T) *sql.DB {
	t.Helper()
	return testdb.Open(t, "outbox_test")
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
	db := newOutboxDB(t)
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
	db := newOutboxDB(t)
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
	db := newOutboxDB(t)
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
	db := newOutboxDB(t)
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
