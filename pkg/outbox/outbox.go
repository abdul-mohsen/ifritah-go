// Package outbox writes events in the business transaction and publishes them after that
// transaction commits.
//
// Append runs inside the transaction that makes a change, so an event exists only if the change
// commits. DrainOnce claims due events under a lease, publishes them, and records the result. A
// crash after publishing and before recording the result publishes the event again, so delivery
// is at least once. Consumers deduplicate by EventID. JetStream also drops a repeat that carries
// the same message ID, inside the stream's duplicate window.
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"

	db "ifritah/web-service-gin/pkg/db/gen"
)

// Event is one message to publish after the transaction that creates it commits.
type Event struct {
	// EventID is a lowercase UUID that the caller creates once. Retries carry the same ID.
	EventID string
	// DocumentKey is the stable key of the document, from identity.DocumentKey.
	DocumentKey string
	// Subject is the NATS subject, for example zatca.bill.<db_name>.<branch_id>.
	Subject string
	// Payload is the message body, as JSON.
	Payload json.RawMessage
}

// Publisher sends one message. msgID is the event ID, so JetStream can drop a repeat.
type Publisher interface {
	Publish(subject string, data []byte, msgID string) error
}

const (
	maxErrorLen = 1024
	maxBackoff  = 5 * time.Minute
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Append writes e in tx. Call it inside the transaction that makes the business change.
func Append(ctx context.Context, tx *sql.Tx, e Event) error {
	if !uuidPattern.MatchString(e.EventID) {
		return fmt.Errorf("outbox: event id %q is not a lowercase UUID", e.EventID)
	}
	if e.DocumentKey == "" || e.Subject == "" || !json.Valid(e.Payload) {
		return errors.New("outbox: document key, subject and a JSON payload are required")
	}
	// The due time comes from the application clock, as the leases do. The column default would
	// use the database clock, whose time zone can differ from UTC.
	return db.New(tx).InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		EventID:       e.EventID,
		DocumentKey:   e.DocumentKey,
		Subject:       e.Subject,
		Payload:       e.Payload,
		NextAttemptAt: time.Now().UTC(),
	})
}

// Drainer publishes due events. Owner names this worker in the lease.
type Drainer struct {
	DB          *sql.DB
	Publisher   Publisher
	Owner       string
	BatchSize   int32
	Lease       time.Duration
	MaxAttempts uint32
	// Now returns the current time. When it is nil, the drainer uses the UTC clock.
	Now func() time.Time
}

// DrainOnce claims a batch of due events, publishes each one, and records the result. It
// returns how many events were published.
func (d *Drainer) DrainOnce(ctx context.Context) (int, error) {
	if d.Owner == "" || d.BatchSize <= 0 || d.Lease <= 0 || d.MaxAttempts == 0 {
		return 0, errors.New("outbox: drainer needs an owner, a batch size, a lease and max attempts")
	}
	now := d.now()
	events, err := d.claim(ctx, now)
	if err != nil {
		return 0, err
	}
	queries := db.New(d.DB)
	published := 0
	for _, ev := range events {
		if pubErr := d.Publisher.Publish(ev.Subject, ev.Payload, ev.EventID); pubErr != nil {
			if err := d.recordFailure(ctx, queries, ev, now, pubErr); err != nil {
				return published, err
			}
			continue
		}
		if err := queries.MarkOutboxPublished(ctx, db.MarkOutboxPublishedParams{
			ID:          ev.ID,
			LeaseOwner:  &d.Owner,
			PublishedAt: &now,
		}); err != nil {
			return published, err
		}
		published++
	}
	return published, nil
}

// claim leases the due events in one transaction, so no other worker can claim them meanwhile.
func (d *Drainer) claim(ctx context.Context, now time.Time) ([]db.ListDueOutboxEventsRow, error) {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	q := db.New(tx)
	events, err := q.ListDueOutboxEvents(ctx, db.ListDueOutboxEventsParams{DueAt: now, ExpiredAt: &now, Limit: d.BatchSize})
	if err != nil {
		return nil, err
	}
	leaseExpires := now.Add(d.Lease)
	for _, ev := range events {
		if err := q.LeaseOutboxEvent(ctx, db.LeaseOutboxEventParams{
			ID:             ev.ID,
			LeaseOwner:     &d.Owner,
			LeaseExpiresAt: &leaseExpires,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return events, nil
}

// recordFailure schedules a retry with backoff, or dead-letters the event once its last allowed
// attempt has failed. ev.AttemptCount counts the attempts before this one, because the lease
// increments it.
func (d *Drainer) recordFailure(ctx context.Context, q *db.Queries, ev db.ListDueOutboxEventsRow, now time.Time, cause error) error {
	lastError := truncate(cause.Error(), maxErrorLen)
	attempt := ev.AttemptCount + 1
	if attempt >= d.MaxAttempts {
		return q.MarkOutboxDeadLetter(ctx, db.MarkOutboxDeadLetterParams{
			ID:         ev.ID,
			LeaseOwner: &d.Owner,
			LastError:  &lastError,
		})
	}
	return q.MarkOutboxRetry(ctx, db.MarkOutboxRetryParams{
		ID:            ev.ID,
		LeaseOwner:    &d.Owner,
		NextAttemptAt: now.Add(backoff(attempt)),
		LastError:     &lastError,
	})
}

// backoff doubles with each attempt, from one second up to maxBackoff.
func backoff(attempts uint32) time.Duration {
	wait := time.Second
	for i := uint32(1); i < attempts && wait < maxBackoff; i++ {
		wait *= 2
	}
	return min(wait, maxBackoff)
}

// truncate shortens s to at most n bytes, cut on a rune boundary.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func (d *Drainer) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now().UTC()
}
