package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	db "ifritah/web-service-gin/pkg/db/gen"
)

const (
	zatcaDocBill    = "bill"
	zatcaDocCredit  = "credit"
	zatcaDocDebit   = "debit"
	zatcaDocOnboard = "onboard"
)

const (
	defaultZatcaOutboxBatchSize = int32(32)
	zatcaOutboxPollInterval     = time.Second
	zatcaOutboxClaimLease       = 5 * time.Minute
	zatcaOutboxMaxRetryDelay    = 5 * time.Minute
)

// buildZatcaOutboxParams creates the exact event payload that will be retried.
// The payload is created before the business transaction commits, but it is
// not sent to NATS until a later drain has observed the committed row.
func (p *ZatcaPublisher) buildZatcaOutboxParams(
	merchantID int32,
	documentType string,
	documentID, branchID int64,
	otp string,
) (db.InsertZatcaEventOutboxParams, error) {
	return p.buildZatcaOutboxParamsWithContext(
		merchantID,
		documentType,
		documentID,
		branchID,
		otp,
		"",
		nil,
	)
}

func (p *ZatcaPublisher) buildZatcaOutboxParamsWithContext(
	merchantID int32,
	documentType string,
	documentID, branchID int64,
	otp string,
	egsID string,
	sequenceNumber *uint64,
) (db.InsertZatcaEventOutboxParams, error) {
	return p.buildZatcaOutboxParamsWithStableKey(
		merchantID,
		documentType,
		documentID,
		branchID,
		otp,
		"",
		egsID,
		sequenceNumber,
	)
}

func (p *ZatcaPublisher) buildZatcaOutboxParamsWithStableKey(
	merchantID int32,
	documentType string,
	documentID, branchID int64,
	otp, stableDocumentKey, egsID string,
	sequenceNumber *uint64,
) (db.InsertZatcaEventOutboxParams, error) {
	if p == nil {
		return db.InsertZatcaEventOutboxParams{}, fmt.Errorf("ZATCA publisher is unavailable")
	}
	if strings.TrimSpace(p.dbName) == "" {
		return db.InsertZatcaEventOutboxParams{}, fmt.Errorf("DBNAME is required for ZATCA outbox events")
	}
	if merchantID <= 0 {
		return db.InsertZatcaEventOutboxParams{}, fmt.Errorf("merchant id must be greater than zero")
	}
	if documentID <= 0 || branchID <= 0 {
		return db.InsertZatcaEventOutboxParams{}, fmt.Errorf("ZATCA document and branch ids must be greater than zero")
	}
	if documentType != zatcaDocBill &&
		documentType != zatcaDocCredit &&
		documentType != zatcaDocDebit &&
		documentType != zatcaDocOnboard {
		return db.InsertZatcaEventOutboxParams{}, fmt.Errorf("unsupported ZATCA document type %q", documentType)
	}
	if documentType == zatcaDocOnboard {
		otp = strings.TrimSpace(otp)
		if otp == "" {
			return db.InsertZatcaEventOutboxParams{}, fmt.Errorf("onboarding OTP is required")
		}
	} else if otp != "" {
		return db.InsertZatcaEventOutboxParams{}, fmt.Errorf("OTP is only valid for onboarding")
	}

	idempotencyKey := strings.TrimSpace(stableDocumentKey)
	if idempotencyKey == "" {
		idempotencyKey = zatcaOutboxIdempotencyKey(p.dbName, documentType, documentID, branchID, otp)
	}
	message := NewMessage(
		documentType,
		p.dbName,
		branchID,
		documentID,
		idempotencyKey,
	).withOTP(otp)
	if strings.TrimSpace(egsID) != "" {
		message.EGSID = strings.TrimSpace(egsID)
	}
	message.SequenceNumber = sequenceNumber
	message.EventID = "evt-" + sha256Hex(idempotencyKey)
	message.Attempt = 1
	message.AttemptCount = 0
	message.AttemptID = fmt.Sprintf("%s:1", idempotencyKey)
	message, err := message.Normalize()
	if err != nil {
		return db.InsertZatcaEventOutboxParams{}, fmt.Errorf("normalize ZATCA outbox event: %w", err)
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return db.InsertZatcaEventOutboxParams{}, fmt.Errorf("marshal ZATCA outbox event: %w", err)
	}

	return db.InsertZatcaEventOutboxParams{
		EventID:        "evt-" + sha256Hex(idempotencyKey),
		TenantID:       p.dbName,
		MerchantID:     merchantID,
		DocumentType:   documentType,
		DocumentID:     uint64(documentID),
		BranchID:       uint32(branchID),
		IdempotencyKey: idempotencyKey,
		Payload:        json.RawMessage(payload),
	}, nil
}

func zatcaOutboxIdempotencyKey(tenant, documentType string, documentID, branchID int64, otp string) string {
	key := fmt.Sprintf("%s:%d:%s:%d", strings.TrimSpace(tenant), branchID, documentType, documentID)
	if documentType == zatcaDocOnboard {
		key += ":otp-" + sha256Hex(strings.TrimSpace(otp))[:16]
	}
	return key
}

func enqueueZatcaEvent(
	ctx context.Context,
	qtx *db.Queries,
	publisher *ZatcaPublisher,
	merchantID int32,
	documentType string,
	documentID, branchID int64,
	otp string,
) error {
	params, err := publisher.buildZatcaOutboxParams(
		merchantID,
		documentType,
		documentID,
		branchID,
		otp,
	)
	if err != nil {
		return err
	}
	if err := qtx.InsertZatcaEventOutbox(ctx, params); err != nil {
		return fmt.Errorf("enqueue ZATCA %s event: %w", documentType, err)
	}
	return nil
}

// EnqueueBill records a bill event in the caller's open transaction.
func (p *ZatcaPublisher) EnqueueBill(
	ctx context.Context,
	qtx *db.Queries,
	merchantID int32,
	billID, branchID int64,
) error {
	return enqueueZatcaEvent(ctx, qtx, p, merchantID, zatcaDocBill, billID, branchID, "")
}

// EnqueueCredit records a credit-note event in the caller's open transaction.
func (p *ZatcaPublisher) EnqueueCredit(
	ctx context.Context,
	qtx *db.Queries,
	merchantID int32,
	creditNoteID, branchID int64,
) error {
	return enqueueZatcaEvent(ctx, qtx, p, merchantID, zatcaDocCredit, creditNoteID, branchID, "")
}

// EnqueueDebit records a debit-note event in the caller's open transaction.
// The baseline has no debit-note HTTP handler, but this is the durable entry
// point for the existing SubmitDebit domain path.
func (p *ZatcaPublisher) EnqueueDebit(
	ctx context.Context,
	qtx *db.Queries,
	merchantID int32,
	debitNoteID, branchID int64,
) error {
	if qtx == nil {
		return fmt.Errorf("ZATCA debit enqueue queries are unavailable")
	}
	if debitNoteID <= 0 {
		return fmt.Errorf("ZATCA debit note id must be greater than zero")
	}
	if branchID <= 0 || uint64(branchID) > uint64(^uint32(0)) {
		return fmt.Errorf("ZATCA debit branch id must be greater than zero")
	}

	row, err := qtx.GetDebitNotePublicationContext(ctx, db.GetDebitNotePublicationContextParams{
		DebitNoteID: uint64(debitNoteID),
		MerchantID:  merchantID,
		BranchID:    uint32(branchID),
	})
	if err != nil {
		return fmt.Errorf("load debit-note publication context: %w", err)
	}
	if row.DocumentID == nil || strings.TrimSpace(*row.DocumentID) == "" ||
		row.StableDocumentKey == nil || strings.TrimSpace(*row.StableDocumentKey) == "" {
		return fmt.Errorf("debit note %d has no stable document identity", debitNoteID)
	}
	if strings.TrimSpace(row.EgsID) == "" {
		return fmt.Errorf("debit note %d has no EGS identity", debitNoteID)
	}

	params, err := p.buildZatcaOutboxParamsWithStableKey(
		merchantID,
		zatcaDocDebit,
		debitNoteID,
		branchID,
		"",
		strings.TrimSpace(*row.StableDocumentKey),
		row.EgsID,
		row.SequenceNumber,
	)
	if err != nil {
		return err
	}
	if err := qtx.InsertZatcaEventOutbox(ctx, params); err != nil {
		return fmt.Errorf("enqueue ZATCA debit event: %w", err)
	}
	return nil
}

// EnqueueOnboarding records an onboarding action in the caller's transaction.
func (p *ZatcaPublisher) EnqueueOnboarding(
	ctx context.Context,
	qtx *db.Queries,
	merchantID int32,
	branchID int64,
	otp string,
) error {
	return enqueueZatcaEvent(ctx, qtx, p, merchantID, zatcaDocOnboard, branchID, branchID, otp)
}

// ZatcaOutboxDrainer claims committed events, publishes them outside database
// locks, and records the result durably.
type ZatcaOutboxDrainer struct {
	db        *sql.DB
	queries   *db.Queries
	publisher *ZatcaPublisher
	batchSize int32
	wake      chan struct{}
}

func newZatcaOutboxDrainer(sqlDB *sql.DB, queries *db.Queries, publisher *ZatcaPublisher) *ZatcaOutboxDrainer {
	return &ZatcaOutboxDrainer{
		db:        sqlDB,
		queries:   queries,
		publisher: publisher,
		batchSize: defaultZatcaOutboxBatchSize,
		wake:      make(chan struct{}, 1),
	}
}

func (d *ZatcaOutboxDrainer) Wake() {
	if d == nil {
		return
	}
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *ZatcaOutboxDrainer) Run(ctx context.Context) {
	if d == nil {
		return
	}

	ticker := time.NewTicker(zatcaOutboxPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.drainOnce(ctx)
		case <-d.wake:
			d.drainOnce(ctx)
		}
	}
}

func (d *ZatcaOutboxDrainer) drainOnce(ctx context.Context) {
	if err := d.Drain(ctx); err != nil {
		log.Printf("[zatca-outbox] drain failed: %s", sanitizeForLog(err.Error()))
	}
}

// Drain performs one bounded claim/publish pass. It is intentionally exported
// for deterministic tests and for callers that want an immediate post-commit
// wake-up without starting a long-lived goroutine.
func (d *ZatcaOutboxDrainer) Drain(ctx context.Context) error {
	if d == nil || d.db == nil || d.queries == nil {
		return fmt.Errorf("ZATCA outbox drainer is unavailable")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ZATCA outbox claim: %w", err)
	}
	qtx := d.queries.WithTx(tx)
	rows, err := qtx.ListDueZatcaEventOutbox(ctx, d.batchSize)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("list due ZATCA outbox events: %w", err)
	}
	for _, row := range rows {
		if err := qtx.MarkZatcaEventOutboxProcessing(ctx, row.ID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("claim ZATCA outbox event %d: %w", row.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ZATCA outbox claims: %w", err)
	}

	var firstErr error
	for _, row := range rows {
		if err := d.publishAndRecord(ctx, row); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (d *ZatcaOutboxDrainer) publishAndRecord(ctx context.Context, row db.ZatcaEventOutbox) error {
	err := d.publisher.publishOutboxRow(row)
	if err == nil {
		if updateErr := d.queries.MarkZatcaEventOutboxPublished(ctx, row.ID); updateErr != nil {
			return fmt.Errorf("mark ZATCA outbox event %d published: %w", row.ID, updateErr)
		}
		if row.DocumentType == zatcaDocOnboard {
			if updateErr := d.queries.ClearZatcaOnboardingPublicationError(ctx, row.BranchID); updateErr != nil {
				return fmt.Errorf("clear onboarding publication error for branch %d: %w", row.BranchID, updateErr)
			}
		}
		return nil
	}

	nextAttempt := time.Now().Add(zatcaOutboxRetryDelay(row.AttemptCount))
	lastError := truncateZatcaOutboxError(err)
	if updateErr := d.queries.MarkZatcaEventOutboxFailed(ctx, db.MarkZatcaEventOutboxFailedParams{
		AvailableAt: nextAttempt,
		LastError:   &lastError,
		ID:          row.ID,
	}); updateErr != nil {
		return fmt.Errorf("publish ZATCA outbox event %d: %v; record failure: %w", row.ID, err, updateErr)
	}
	if row.DocumentType == zatcaDocOnboard {
		if updateErr := d.queries.MarkZatcaOnboardingPublicationFailure(ctx, db.MarkZatcaOnboardingPublicationFailureParams{
			LastError: &lastError,
			BranchID:  row.BranchID,
		}); updateErr != nil {
			return fmt.Errorf("publish ZATCA outbox event %d: %v; record onboarding failure: %w", row.ID, err, updateErr)
		}
	}
	return fmt.Errorf("publish ZATCA outbox event %d: %w", row.ID, err)
}

func zatcaOutboxRetryDelay(attemptCount uint32) time.Duration {
	delay := 5 * time.Second
	for i := uint32(0); i < attemptCount && delay < zatcaOutboxMaxRetryDelay; i++ {
		delay *= 2
	}
	if delay > zatcaOutboxMaxRetryDelay {
		return zatcaOutboxMaxRetryDelay
	}
	return delay
}

func truncateZatcaOutboxError(err error) string {
	if err == nil {
		return ""
	}
	message := sanitizeForLog(err.Error())
	if len(message) > 1024 {
		return message[:1024]
	}
	return message
}

// publishPayload is the only path used by the drainer. It reconstructs the
// existing subject from the stored payload and supplies the durable key as the
// JetStream message id so acknowledgement/update races remain idempotent.
func (p *ZatcaPublisher) publishPayload(payload []byte, idempotencyKey string) error {
	if p == nil || p.js == nil {
		return fmt.Errorf("publish ZATCA message: NATS JetStream is unavailable")
	}

	var message Message
	if err := json.Unmarshal(payload, &message); err != nil {
		return fmt.Errorf("decode ZATCA outbox payload: %w", err)
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		message.IdempotencyKey = strings.TrimSpace(idempotencyKey)
		message.StableDocumentKey = strings.TrimSpace(idempotencyKey)
	}
	if err := p.publishMessage(message); err != nil {
		return err
	}
	return nil
}

func (p *ZatcaPublisher) publishOutboxRow(row db.ZatcaEventOutbox) error {
	if p == nil || p.js == nil {
		return fmt.Errorf("publish ZATCA message: NATS JetStream is unavailable")
	}

	var message Message
	if err := json.Unmarshal(row.Payload, &message); err != nil {
		return fmt.Errorf("decode ZATCA outbox payload: %w", err)
	}
	message.EventID = row.EventID
	message.TenantID = row.TenantID
	message.DBName = row.TenantID
	message.BranchID = int64(row.BranchID)
	message.DocType = row.DocumentType
	message.DocumentType = row.DocumentType
	message.ID = int64(row.DocumentID)
	message.DocumentID = int64(row.DocumentID)
	message.SourceID = int64(row.DocumentID)
	message.OutboxID = row.ID
	message.IdempotencyKey = row.IdempotencyKey
	message.StableDocumentKey = row.IdempotencyKey
	message.AttemptCount = row.AttemptCount
	message.Attempt = row.AttemptCount + 1
	message.AttemptID = fmt.Sprintf("%d:%d", row.ID, message.Attempt)

	if err := p.publishMessage(message); err != nil {
		return err
	}
	return nil
}
