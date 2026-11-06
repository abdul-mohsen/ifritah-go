package outbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/go-sql-driver/mysql"
)

const (
	recoveryStatusDeadLetter = "dead_letter"
	recoveryStatusPending    = "pending"
	DefaultRecoveryListLimit = int32(50)
	MaxRecoveryListLimit     = int32(100)
	maxRecoveryReasonBytes   = 1024
	maxRecoveryKeyBytes      = 128
	maxRecoveryUsernameBytes = 45
)

var (
	ErrRecoveryInvalidRequest      = errors.New("outbox: invalid recovery request")
	ErrRecoveryEventNotFound       = errors.New("outbox: recovery event not found")
	ErrRecoveryNotDeadLetter       = errors.New("outbox: event is not dead-lettered")
	ErrRecoveryIdempotencyConflict = errors.New("outbox: recovery idempotency key belongs to another event")
)

// RecoveryRequest describes one explicit operator recovery action.
type RecoveryRequest struct {
	EventID          string
	IdempotencyKey   string
	OperatorUserID   *int32
	OperatorUsername string
	Reason           string
	Now              time.Time
}

// RecoveryResult contains the event after the recovery transition and its audit row.
type RecoveryResult struct {
	Event          db.Outbox
	Audit          db.OutboxRecoveryAudit
	AlreadyApplied bool
}

// IsValidEventID reports whether eventID is the lowercase UUID used by the outbox.
func IsValidEventID(eventID string) bool {
	return uuidPattern.MatchString(eventID)
}

// ListDeadLetterEvents returns operator-visible metadata for dead-lettered events.
func ListDeadLetterEvents(ctx context.Context, sqlDB *sql.DB, limit int32) ([]db.Outbox, error) {
	if sqlDB == nil {
		return nil, errors.New("outbox: database is not configured")
	}
	if limit <= 0 || limit > MaxRecoveryListLimit {
		return nil, fmt.Errorf("%w: limit must be between 1 and %d", ErrRecoveryInvalidRequest, MaxRecoveryListLimit)
	}
	return db.New(sqlDB).ListDeadLetterOutboxEvents(ctx, limit)
}

// GetEvent returns one outbox event by its stable event ID.
func GetEvent(ctx context.Context, sqlDB *sql.DB, eventID string) (db.Outbox, error) {
	if sqlDB == nil {
		return db.Outbox{}, errors.New("outbox: database is not configured")
	}
	if !IsValidEventID(eventID) {
		return db.Outbox{}, fmt.Errorf("%w: event id must be a lowercase UUID", ErrRecoveryInvalidRequest)
	}
	return db.New(sqlDB).GetOutboxEventByEventID(ctx, eventID)
}

// ListRecoveryAudit returns the append-only recovery history for eventID.
func ListRecoveryAudit(ctx context.Context, sqlDB *sql.DB, eventID string) ([]db.OutboxRecoveryAudit, error) {
	if sqlDB == nil {
		return nil, errors.New("outbox: database is not configured")
	}
	if !IsValidEventID(eventID) {
		return nil, fmt.Errorf("%w: event id must be a lowercase UUID", ErrRecoveryInvalidRequest)
	}
	return db.New(sqlDB).ListOutboxRecoveryAudit(ctx, eventID)
}

// RecoverDeadLetter atomically requeues one dead-lettered event and appends its audit record.
// The committed business row and event payload are never modified.
func RecoverDeadLetter(ctx context.Context, sqlDB *sql.DB, request RecoveryRequest) (RecoveryResult, error) {
	request, err := normalizeRecoveryRequest(request)
	if err != nil {
		return RecoveryResult{}, err
	}
	if sqlDB == nil {
		return RecoveryResult{}, errors.New("outbox: database is not configured")
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("outbox: begin recovery: %w", err)
	}
	defer tx.Rollback()

	q := db.New(tx)
	event, err := q.GetOutboxEventForRecovery(ctx, request.EventID)
	if errors.Is(err, sql.ErrNoRows) {
		return RecoveryResult{}, ErrRecoveryEventNotFound
	}
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("outbox: load recovery event: %w", err)
	}

	audit, auditErr := q.GetOutboxRecoveryAuditByIdempotencyKey(ctx, request.IdempotencyKey)
	if auditErr == nil {
		if audit.EventID != request.EventID || audit.OutboxID != event.ID {
			return RecoveryResult{}, ErrRecoveryIdempotencyConflict
		}
		if err := tx.Commit(); err != nil {
			return RecoveryResult{}, fmt.Errorf("outbox: commit idempotent recovery: %w", err)
		}
		return RecoveryResult{
			Event:          event,
			Audit:          audit,
			AlreadyApplied: true,
		}, nil
	}
	if !errors.Is(auditErr, sql.ErrNoRows) {
		return RecoveryResult{}, fmt.Errorf("outbox: load recovery audit: %w", auditErr)
	}
	if event.Status != recoveryStatusDeadLetter {
		return RecoveryResult{}, ErrRecoveryNotDeadLetter
	}

	previousStatus := event.Status
	previousAttemptCount := event.AttemptCount
	previousLastError := event.LastError
	updated, err := q.RecoverOutboxEvent(ctx, db.RecoverOutboxEventParams{
		NextAttemptAt: request.Now,
		ID:            event.ID,
	})
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("outbox: requeue event: %w", err)
	}
	rows, err := updated.RowsAffected()
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("outbox: inspect requeue: %w", err)
	}
	if rows != 1 {
		return RecoveryResult{}, ErrRecoveryNotDeadLetter
	}

	inserted, err := q.InsertOutboxRecoveryAudit(ctx, db.InsertOutboxRecoveryAuditParams{
		OutboxID:             event.ID,
		EventID:              event.EventID,
		IdempotencyKey:       request.IdempotencyKey,
		OperatorUserID:       request.OperatorUserID,
		OperatorUsername:     request.OperatorUsername,
		Reason:               request.Reason,
		PreviousStatus:       previousStatus,
		PreviousAttemptCount: previousAttemptCount,
		PreviousLastError:    previousLastError,
		RecoveredAt:          request.Now,
	})
	if err != nil {
		if isDuplicate(err) {
			return RecoveryResult{}, ErrRecoveryIdempotencyConflict
		}
		return RecoveryResult{}, fmt.Errorf("outbox: write recovery audit: %w", err)
	}
	recoveryID, err := inserted.LastInsertId()
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("outbox: inspect recovery audit: %w", err)
	}
	if recoveryID < 1 {
		return RecoveryResult{}, errors.New("outbox: recovery audit returned an invalid ID")
	}

	if err := tx.Commit(); err != nil {
		return RecoveryResult{}, fmt.Errorf("outbox: commit recovery: %w", err)
	}

	event.Status = recoveryStatusPending
	event.AttemptCount = 0
	event.NextAttemptAt = request.Now
	event.LeaseOwner = nil
	event.LeaseExpiresAt = nil
	audit = db.OutboxRecoveryAudit{
		ID:                   uint64(recoveryID),
		OutboxID:             event.ID,
		EventID:              event.EventID,
		IdempotencyKey:       request.IdempotencyKey,
		OperatorUserID:       request.OperatorUserID,
		OperatorUsername:     request.OperatorUsername,
		Reason:               request.Reason,
		PreviousStatus:       previousStatus,
		PreviousAttemptCount: previousAttemptCount,
		PreviousLastError:    previousLastError,
		RecoveredAt:          request.Now,
	}
	return RecoveryResult{Event: event, Audit: audit}, nil
}

func normalizeRecoveryRequest(request RecoveryRequest) (RecoveryRequest, error) {
	request.EventID = strings.TrimSpace(request.EventID)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.OperatorUsername = strings.TrimSpace(request.OperatorUsername)
	request.Reason = strings.TrimSpace(request.Reason)
	switch {
	case !IsValidEventID(request.EventID):
		return RecoveryRequest{}, fmt.Errorf("%w: event id must be a lowercase UUID", ErrRecoveryInvalidRequest)
	case !validRecoveryASCII(request.IdempotencyKey, maxRecoveryKeyBytes):
		return RecoveryRequest{}, fmt.Errorf("%w: idempotency key must be 1 to %d ASCII bytes without control characters", ErrRecoveryInvalidRequest, maxRecoveryKeyBytes)
	case !validRecoveryText(request.OperatorUsername, maxRecoveryUsernameBytes):
		return RecoveryRequest{}, fmt.Errorf("%w: operator username must be 1 to %d bytes without control characters", ErrRecoveryInvalidRequest, maxRecoveryUsernameBytes)
	case !validRecoveryText(request.Reason, maxRecoveryReasonBytes):
		return RecoveryRequest{}, fmt.Errorf("%w: reason must be 1 to %d bytes without control characters", ErrRecoveryInvalidRequest, maxRecoveryReasonBytes)
	}
	if request.Now.IsZero() {
		request.Now = time.Now().UTC()
	} else {
		request.Now = request.Now.UTC()
	}
	return request, nil
}

func validRecoveryText(value string, maxBytes int) bool {
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validRecoveryASCII(value string, maxBytes int) bool {
	if !validRecoveryText(value, maxBytes) {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] > 0x7f {
			return false
		}
	}
	return true
}

func isDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
