package handlers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"os"
	"strconv"
	"time"
)

const (
	outboxStatusPending      = "pending"
	outboxStatusInFlight     = "in_flight"
	outboxStatusRetryable    = "retryable"
	outboxStatusPublished    = "published"
	outboxStatusDeadLetter   = "dead_letter"
	outboxStatusManualReview = "manual_review"

	defaultOutboxLease       = 30 * time.Second
	defaultOutboxMaxAttempts = uint32(5)
)

var ErrOutboxContentConflict = errors.New("outbox event content conflict")

type outboxEventPublisher interface {
	PublishOutbox(context.Context, uint64, string) error
}

type outboxTransport interface {
	PublishOutboxPayload([]byte) error
}

type outboxDispatcher struct {
	db            *sql.DB
	queries       *db.Queries
	transport     outboxTransport
	owner         string
	now           func() time.Time
	leaseDuration time.Duration
	maxAttempts   uint32
}

func newOutboxDispatcher(sqlDB *sql.DB, queries *db.Queries, transport outboxTransport) *outboxDispatcher {
	return &outboxDispatcher{
		db:            sqlDB,
		queries:       queries,
		transport:     transport,
		owner:         "backend-" + strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		now:           time.Now,
		leaseDuration: defaultOutboxLease,
		maxAttempts:   defaultOutboxMaxAttempts,
	}
}

func (d *outboxDispatcher) PublishOutbox(ctx context.Context, tenantID uint64, eventID string) error {
	if d == nil || d.db == nil || d.queries == nil || d.transport == nil {
		return errors.New("outbox dispatcher is not configured")
	}

	claimed, event, err := d.claim(ctx, tenantID, eventID)
	if err != nil || !claimed {
		return err
	}

	if err := validateOutboxPayload(event); err != nil {
		if errors.Is(err, ErrOutboxContentConflict) {
			_ = d.queries.MarkOutboxManualReview(ctx, db.MarkOutboxManualReviewParams{
				TenantID: event.TenantID,
				EventID:  event.EventID,
			})
		} else {
			d.recordFailure(ctx, event, err)
		}
		return err
	}
	if err := d.transport.PublishOutboxPayload(event.Payload); err != nil {
		d.recordFailure(ctx, event, err)
		return err
	}
	if err := d.queries.MarkOutboxPublished(ctx, db.MarkOutboxPublishedParams{
		TenantID:   event.TenantID,
		EventID:    event.EventID,
		LeaseOwner: event.LeaseOwner,
	}); err != nil {
		return fmt.Errorf("mark outbox event %s published: %w", event.EventID, err)
	}
	return nil
}

func (d *outboxDispatcher) claim(ctx context.Context, tenantID uint64, eventID string) (bool, db.Outbox, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return false, db.Outbox{}, fmt.Errorf("begin outbox claim: %w", err)
	}
	defer tx.Rollback()

	qtx := d.queries.WithTx(tx)
	owner := d.owner
	leaseExpiresAt := d.now().Add(d.leaseDuration)
	result, err := qtx.ClaimOutboxEvent(ctx, db.ClaimOutboxEventParams{
		LeaseOwner:     &owner,
		LeaseExpiresAt: &leaseExpiresAt,
		TenantID:       tenantID,
		EventID:        eventID,
	})
	if err != nil {
		return false, db.Outbox{}, fmt.Errorf("claim outbox event %s: %w", eventID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, db.Outbox{}, fmt.Errorf("inspect outbox claim %s: %w", eventID, err)
	}
	if rows == 0 {
		return false, db.Outbox{}, nil
	}

	event, err := qtx.GetOutboxEventByTenant(ctx, db.GetOutboxEventByTenantParams{
		TenantID: tenantID,
		EventID:  eventID,
	})
	if err != nil {
		return false, db.Outbox{}, fmt.Errorf("load claimed outbox event %s: %w", eventID, err)
	}
	if err := tx.Commit(); err != nil {
		return false, db.Outbox{}, fmt.Errorf("commit outbox claim %s: %w", eventID, err)
	}
	return true, event, nil
}

func (d *outboxDispatcher) recordFailure(ctx context.Context, event db.Outbox, publishErr error) {
	now := d.now()
	status := outboxStatusRetryable
	var deadLetteredAt *time.Time
	var manualReviewAt *time.Time
	if event.AttemptCount >= d.maxAttempts {
		status = outboxStatusDeadLetter
		deadLetteredAt = &now
		manualReviewAt = &now
	}
	nextAttemptAt := now.Add(outboxRetryBackoff(event.AttemptCount))
	message := publishErr.Error()
	if len(message) > 4000 {
		message = message[:4000]
	}
	if err := d.queries.MarkOutboxPublishFailed(ctx, db.MarkOutboxPublishFailedParams{
		Status:         status,
		NextAttemptAt:  nextAttemptAt,
		LastError:      &message,
		DeadLetteredAt: deadLetteredAt,
		ManualReviewAt: manualReviewAt,
		TenantID:       event.TenantID,
		EventID:        event.EventID,
		LeaseOwner:     event.LeaseOwner,
	}); err != nil {
		// The original publish error is the actionable failure. A later lease
		// reclaim will recover an in-flight row if this update is unavailable.
		_ = err
	}
}

func outboxRetryBackoff(attempt uint32) time.Duration {
	if attempt == 0 {
		attempt = 1
	}
	if attempt > 11 {
		attempt = 11
	}
	return time.Duration(uint64(1)<<(attempt-1)) * time.Second
}

func validateOutboxPayload(event db.Outbox) error {
	if !event.PayloadFingerprint.Valid && !event.ContentHash.Valid {
		return nil
	}
	sum := sha256.Sum256(event.Payload)
	actual := hex.EncodeToString(sum[:])
	if event.PayloadFingerprint.Valid && event.PayloadFingerprint.String != actual {
		return fmt.Errorf("%w: payload_fingerprint does not match payload", ErrOutboxContentConflict)
	}
	if event.ContentHash.Valid && event.ContentHash.String != actual {
		return fmt.Errorf("%w: content_hash does not match payload", ErrOutboxContentConflict)
	}
	if event.PayloadFingerprint.Valid && event.ContentHash.Valid &&
		event.PayloadFingerprint.String != event.ContentHash.String {
		return fmt.Errorf("%w: payload fingerprints disagree", ErrOutboxContentConflict)
	}
	return nil
}

func insertAtomicSaleOutbox(ctx context.Context, qtx *db.Queries, params db.InsertZatcaOutboxParams) error {
	if _, err := qtx.InsertZatcaOutbox(ctx, params); err == nil {
		return nil
	} else if !IsDuplicate(err) {
		return err
	}

	existing, err := qtx.GetOutboxEventByTenant(ctx, db.GetOutboxEventByTenantParams{
		TenantID: params.TenantID,
		EventID:  params.EventID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		existing, err = qtx.GetOutboxEventByRequestIdentity(ctx, db.GetOutboxEventByRequestIdentityParams{
			TenantID:  params.TenantID,
			PosID:     params.PosID,
			RequestID: params.RequestID,
		})
	}
	if err != nil {
		return err
	}
	if outboxEventMatches(existing, params) {
		return nil
	}
	return fmt.Errorf("%w: event_id %s", ErrOutboxContentConflict, params.EventID)
}

func outboxEventMatches(existing db.Outbox, params db.InsertZatcaOutboxParams) bool {
	if existing.TenantID != params.TenantID ||
		existing.EventID != params.EventID ||
		existing.AggregateType != params.AggregateType ||
		existing.AggregateID != params.AggregateID ||
		existing.PayloadFingerprint != params.PayloadFingerprint ||
		existing.ContentHash != params.ContentHash ||
		string(existing.Payload) != string(params.Payload) {
		return false
	}
	if !nullableStringEqual(existing.PosID, params.PosID) ||
		existing.DocumentType != params.DocumentType ||
		existing.SourceID != params.SourceID ||
		!nullStringEqual(existing.RequestID, params.RequestID) ||
		!nullStringEqual(existing.InvoiceUuid, params.InvoiceUuid) ||
		!nullableStringEqual(existing.OriginalDocumentType, params.OriginalDocumentType) ||
		!nullableUint64Equal(existing.OriginalSourceID, params.OriginalSourceID) ||
		!nullStringEqual(existing.OriginalInvoiceUuid, params.OriginalInvoiceUuid) ||
		!nullableStringEqual(existing.EgsSequenceID, params.EgsSequenceID) ||
		!nullableUint64Equal(existing.Icv, params.Icv) ||
		!nullableStringEqual(existing.Pih, params.Pih) {
		return false
	}
	return true
}

func nullableStringEqual(left, right *string) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func nullableUint64Equal(left, right *uint64) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func nullStringEqual(left, right sql.NullString) bool {
	return left.Valid == right.Valid && (!left.Valid || left.String == right.String)
}

func atomicSaleOutboxPayloadBytes(payload atomicSaleOutboxPayload) ([]byte, string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}
