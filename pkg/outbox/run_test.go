package outbox

import (
	"context"
	"testing"
	"time"

	"ifritah/web-service-gin/pkg/testdb"
)

// TestRunPublishesCommittedEventsUntilCancelled checks the worker loop: it publishes the events that
// are already committed, and it returns once its context is cancelled.
func TestRunPublishesCommittedEventsUntilCancelled(t *testing.T) {
	db := testdb.Open(t, "outbox")
	appendCommitted(t, db, testEvent(eventA), testEvent(eventB))
	pub := &fakePublisher{}
	d := &Drainer{DB: db, Publisher: pub, Owner: "worker-run", BatchSize: 10, Lease: time.Minute, MaxAttempts: 3}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		d.Run(ctx, 5*time.Millisecond)
		close(done)
	}()

	deadline := time.Now().Add(10 * time.Second)
	for len(pub.msgIDs()) < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
	if ids := pub.msgIDs(); len(ids) != 2 {
		t.Fatalf("published %v; want both committed events", ids)
	}
}
