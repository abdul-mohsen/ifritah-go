package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	generated "ifritah/web-service-gin/pkg/db/gen"
)

var (
	ErrEGSSequenceBusy                = errors.New("egs sequence reservation is busy")
	ErrEGSSequenceOwnership           = errors.New("egs sequence ownership mismatch")
	ErrEGSSequenceReservationConflict = errors.New("egs sequence reservation identity conflict")
	ErrEGSSequenceCommitNotApplied    = errors.New("egs sequence commit was not applied")
)

const (
	defaultEGSSequenceLease = 5 * time.Minute

	EGSDocumentQueued    = "queued"
	EGSDocumentSubmitted = "submitted"
	EGSDocumentAccepted  = "accepted"
	EGSDocumentWarning   = "warning"
	EGSDocumentRejected  = "rejected"
	EGSDocumentAmbiguous = "ambiguous"
)

// EGSSequenceIdentity is the canonical owner of one invoice hash chain.
// User, company, and POS identities are intentionally not part of this key.
type EGSSequenceIdentity struct {
	EGSSequenceID string
	TenantID      uint64
	BranchID      uint32
	VATNumber     string
	EGSID         string
}

type EGSSequenceReservationRequest struct {
	EGSSequenceIdentity
	ReservationKey   string
	ReservationToken string
	DocumentUUID     string
	OutboxEventID    string
	LeaseDuration    time.Duration
}

type EGSSequenceReservation struct {
	EGSSequenceIdentity
	ReservationKey   string
	ReservationToken string
	DocumentUUID     string
	OutboxEventID    string
	ICV              uint64
	PIH              *string
	LeaseExpiresAt   time.Time
}

// EGSSequenceCommitRequest contains the immutable document snapshot and its
// matching outbox event. Both records are inserted before the chain advances.
type EGSSequenceCommitRequest struct {
	Reservation EGSSequenceReservation

	DocumentType string
	SourceID     uint64
	State        string
	Snapshot     json.RawMessage

	AggregateType string
	AggregateID   uint64
	PosID         *string
	BillRequestID sql.NullString
	Payload       json.RawMessage

	PayloadFingerprint  sql.NullString
	ContentHash         sql.NullString
	PreviousInvoiceHash *string
}

type EGSSequenceAllocator struct {
	DB      *sql.DB
	Queries *generated.Queries
	Now     func() time.Time
}

func NewEGSSequenceAllocator(sqlDB *sql.DB, queries *generated.Queries) *EGSSequenceAllocator {
	return &EGSSequenceAllocator{
		DB:      sqlDB,
		Queries: queries,
		Now:     time.Now,
	}
}

func (a *EGSSequenceAllocator) Reserve(
	ctx context.Context,
	request EGSSequenceReservationRequest,
) (EGSSequenceReservation, error) {
	if err := validateReservationRequest(request); err != nil {
		return EGSSequenceReservation{}, err
	}
	if a == nil || a.DB == nil || a.Queries == nil {
		return EGSSequenceReservation{}, errors.New("egs sequence allocator is not configured")
	}

	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	leaseDuration := request.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = defaultEGSSequenceLease
	}
	leaseExpiresAt := now().Add(leaseDuration)

	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return EGSSequenceReservation{}, fmt.Errorf("begin EGS sequence reservation: %w", err)
	}
	defer tx.Rollback()

	qtx := a.Queries.WithTx(tx)
	identity := request.EGSSequenceIdentity
	if err := qtx.EnsureEGSSequence(ctx, generated.EnsureEGSSequenceParams{
		EgsSequenceID: identity.EGSSequenceID,
		TenantID:      identity.TenantID,
		BranchID:      identity.BranchID,
		VatNumber:     identity.VATNumber,
		EgsID:         identity.EGSID,
	}); err != nil {
		return EGSSequenceReservation{}, fmt.Errorf("ensure EGS sequence %s: %w", identity.EGSSequenceID, err)
	}

	state, err := qtx.GetEGSSequenceForUpdate(ctx, generated.GetEGSSequenceForUpdateParams{
		EgsSequenceID: identity.EGSSequenceID,
		TenantID:      identity.TenantID,
		BranchID:      identity.BranchID,
		VatNumber:     identity.VATNumber,
		EgsID:         identity.EGSID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return EGSSequenceReservation{}, fmt.Errorf("%w: %s", ErrEGSSequenceOwnership, identity.EGSSequenceID)
	}
	if err != nil {
		return EGSSequenceReservation{}, fmt.Errorf("lock EGS sequence %s: %w", identity.EGSSequenceID, err)
	}

	if state.ReservationKey != nil {
		if *state.ReservationKey == request.ReservationKey {
			if (state.ReservedDocumentUuid.Valid && state.ReservedDocumentUuid.String != request.DocumentUUID) ||
				(state.ReservedOutboxEventID.Valid && state.ReservedOutboxEventID.String != request.OutboxEventID) {
				return EGSSequenceReservation{}, fmt.Errorf("%w: %s", ErrEGSSequenceReservationConflict, identity.EGSSequenceID)
			}
			reservation, err := reservationFromState(state)
			if err != nil {
				return EGSSequenceReservation{}, err
			}
			if err := tx.Commit(); err != nil {
				return EGSSequenceReservation{}, fmt.Errorf("commit EGS sequence replay: %w", err)
			}
			return reservation, nil
		}
		if state.LeaseExpiresAt != nil && state.LeaseExpiresAt.After(now()) {
			return EGSSequenceReservation{}, fmt.Errorf("%w: %s", ErrEGSSequenceBusy, identity.EGSSequenceID)
		}
	}
	if state.LastIcv == ^uint64(0) {
		return EGSSequenceReservation{}, fmt.Errorf("EGS sequence %s exhausted", identity.EGSSequenceID)
	}

	result, err := qtx.ReserveEGSSequence(ctx, generated.ReserveEGSSequenceParams{
		ReservationKey:   stringPointer(request.ReservationKey),
		ReservationToken: stringPointer(request.ReservationToken),
		DocumentUuid:     sql.NullString{String: request.DocumentUUID, Valid: true},
		OutboxEventID:    sql.NullString{String: request.OutboxEventID, Valid: true},
		LeaseExpiresAt:   &leaseExpiresAt,
		EgsSequenceID:    identity.EGSSequenceID,
		TenantID:         identity.TenantID,
		BranchID:         identity.BranchID,
		VatNumber:        identity.VATNumber,
		EgsID:            identity.EGSID,
	})
	if err != nil {
		return EGSSequenceReservation{}, fmt.Errorf("reserve EGS sequence %s: %w", identity.EGSSequenceID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return EGSSequenceReservation{}, fmt.Errorf("inspect EGS sequence reservation %s: %w", identity.EGSSequenceID, err)
	}
	if affected != 1 {
		return EGSSequenceReservation{}, fmt.Errorf("%w: %s", ErrEGSSequenceBusy, identity.EGSSequenceID)
	}

	reservation := EGSSequenceReservation{
		EGSSequenceIdentity: identity,
		ReservationKey:      request.ReservationKey,
		ReservationToken:    request.ReservationToken,
		DocumentUUID:        request.DocumentUUID,
		OutboxEventID:       request.OutboxEventID,
		ICV:                 state.LastIcv + 1,
		PIH:                 state.PreviousInvoiceHash,
		LeaseExpiresAt:      leaseExpiresAt,
	}
	if err := tx.Commit(); err != nil {
		return EGSSequenceReservation{}, fmt.Errorf("commit EGS sequence reservation: %w", err)
	}
	return reservation, nil
}

func (a *EGSSequenceAllocator) Commit(
	ctx context.Context,
	request EGSSequenceCommitRequest,
) error {
	if err := validateCommitRequest(request); err != nil {
		return err
	}
	if a == nil || a.DB == nil || a.Queries == nil {
		return errors.New("egs sequence allocator is not configured")
	}

	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin EGS sequence commit: %w", err)
	}
	defer tx.Rollback()

	qtx := a.Queries.WithTx(tx)
	reservation := request.Reservation
	pih := reservation.PIH
	if request.PreviousInvoiceHash == nil {
		request.PreviousInvoiceHash = stringPointer("")
	}
	snapshot := request.Snapshot
	if len(snapshot) == 0 {
		snapshot = request.Payload
	}
	payload := request.Payload
	if len(payload) == 0 {
		payload = snapshot
	}
	payloadFingerprint := request.PayloadFingerprint
	contentHash := request.ContentHash
	if !payloadFingerprint.Valid || !contentHash.Valid {
		hash := sha256Hex(payload)
		if !payloadFingerprint.Valid {
			payloadFingerprint = sql.NullString{String: hash, Valid: true}
		}
		if !contentHash.Valid {
			contentHash = sql.NullString{String: hash, Valid: true}
		}
	}
	state := request.State
	if state == "" {
		state = EGSDocumentQueued
	}
	aggregateType := request.AggregateType
	if aggregateType == "" {
		aggregateType = request.DocumentType
	}
	aggregateID := request.AggregateID
	if aggregateID == 0 {
		aggregateID = request.SourceID
	}
	sequenceID := reservation.EGSSequenceID
	icv := reservation.ICV

	if _, err := qtx.InsertZatcaEGSDocument(ctx, generated.InsertZatcaEGSDocumentParams{
		TenantID:         reservation.TenantID,
		EgsSequenceID:    sequenceID,
		BranchID:         reservation.BranchID,
		VatNumber:        reservation.VATNumber,
		EgsID:            reservation.EGSID,
		DocumentUuid:     reservation.DocumentUUID,
		DocumentType:     request.DocumentType,
		SourceID:         request.SourceID,
		OutboxEventID:    reservation.OutboxEventID,
		Icv:              icv,
		Pih:              pih,
		DocumentSnapshot: snapshot,
		State:            state,
	}); err != nil {
		return fmt.Errorf("insert immutable EGS document %s: %w", reservation.DocumentUUID, err)
	}
	if _, err := qtx.InsertSaleOutbox(ctx, generated.InsertSaleOutboxParams{
		TenantID:           reservation.TenantID,
		EventID:            reservation.OutboxEventID,
		AggregateType:      aggregateType,
		AggregateID:        aggregateID,
		PosID:              request.PosID,
		BillRequestID:      request.BillRequestID,
		InvoiceUuid:        sql.NullString{String: reservation.DocumentUUID, Valid: true},
		EgsSequenceID:      stringPointer(sequenceID),
		Icv:                &icv,
		Pih:                pih,
		Payload:            payload,
		PayloadFingerprint: payloadFingerprint,
		ContentHash:        contentHash,
	}); err != nil {
		return fmt.Errorf("insert EGS outbox event %s: %w", reservation.OutboxEventID, err)
	}

	result, err := qtx.CommitEGSSequence(ctx, generated.CommitEGSSequenceParams{
		PreviousInvoiceHash: request.PreviousInvoiceHash,
		EgsSequenceID:       sequenceID,
		TenantID:            reservation.TenantID,
		BranchID:            reservation.BranchID,
		VatNumber:           reservation.VATNumber,
		EgsID:               reservation.EGSID,
		ReservationKey:      stringPointer(reservation.ReservationKey),
		ReservationToken:    stringPointer(reservation.ReservationToken),
		ReservedIcv:         &icv,
		DocumentUuid:        sql.NullString{String: reservation.DocumentUUID, Valid: true},
		OutboxEventID:       sql.NullString{String: reservation.OutboxEventID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("commit EGS sequence %s: %w", sequenceID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect EGS sequence commit %s: %w", sequenceID, err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: %s", ErrEGSSequenceCommitNotApplied, sequenceID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit EGS document and sequence %s: %w", sequenceID, err)
	}
	return nil
}

func (a *EGSSequenceAllocator) Release(
	ctx context.Context,
	reservation EGSSequenceReservation,
) error {
	if err := validateReservation(reservation); err != nil {
		return err
	}
	if a == nil || a.DB == nil || a.Queries == nil {
		return errors.New("egs sequence allocator is not configured")
	}

	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin EGS sequence release: %w", err)
	}
	defer tx.Rollback()

	result, err := a.Queries.WithTx(tx).ReleaseEGSSequence(ctx, generated.ReleaseEGSSequenceParams{
		EgsSequenceID:    reservation.EGSSequenceID,
		TenantID:         reservation.TenantID,
		BranchID:         reservation.BranchID,
		VatNumber:        reservation.VATNumber,
		EgsID:            reservation.EGSID,
		ReservationKey:   stringPointer(reservation.ReservationKey),
		ReservationToken: stringPointer(reservation.ReservationToken),
	})
	if err != nil {
		return fmt.Errorf("release EGS sequence %s: %w", reservation.EGSSequenceID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect EGS sequence release %s: %w", reservation.EGSSequenceID, err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: %s", ErrEGSSequenceOwnership, reservation.EGSSequenceID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit EGS sequence release %s: %w", reservation.EGSSequenceID, err)
	}
	return nil
}

func reservationFromState(state generated.EgsSequence) (EGSSequenceReservation, error) {
	if state.ReservedIcv == nil ||
		state.ReservationKey == nil ||
		state.ReservationToken == nil ||
		!state.ReservedDocumentUuid.Valid ||
		!state.ReservedOutboxEventID.Valid ||
		state.LeaseExpiresAt == nil {
		return EGSSequenceReservation{}, fmt.Errorf("%w: incomplete reservation for %s", ErrEGSSequenceOwnership, state.EgsSequenceID)
	}
	return EGSSequenceReservation{
		EGSSequenceIdentity: EGSSequenceIdentity{
			EGSSequenceID: state.EgsSequenceID,
			TenantID:      state.TenantID,
			BranchID:      state.BranchID,
			VATNumber:     state.VatNumber,
			EGSID:         state.EgsID,
		},
		ReservationKey:   *state.ReservationKey,
		ReservationToken: *state.ReservationToken,
		DocumentUUID:     state.ReservedDocumentUuid.String,
		OutboxEventID:    state.ReservedOutboxEventID.String,
		ICV:              *state.ReservedIcv,
		PIH:              state.ReservedPreviousInvoiceHash,
		LeaseExpiresAt:   *state.LeaseExpiresAt,
	}, nil
}

func validateReservationRequest(request EGSSequenceReservationRequest) error {
	if err := validateIdentity(request.EGSSequenceIdentity); err != nil {
		return err
	}
	if strings.TrimSpace(request.ReservationKey) == "" ||
		strings.TrimSpace(request.ReservationToken) == "" ||
		strings.TrimSpace(request.DocumentUUID) == "" ||
		strings.TrimSpace(request.OutboxEventID) == "" {
		return errors.New("reservation key, token, document UUID, and outbox event ID are required")
	}
	return nil
}

func validateReservation(reservation EGSSequenceReservation) error {
	if err := validateIdentity(reservation.EGSSequenceIdentity); err != nil {
		return err
	}
	if strings.TrimSpace(reservation.ReservationKey) == "" ||
		strings.TrimSpace(reservation.ReservationToken) == "" ||
		strings.TrimSpace(reservation.DocumentUUID) == "" ||
		strings.TrimSpace(reservation.OutboxEventID) == "" ||
		reservation.ICV == 0 {
		return errors.New("complete EGS sequence reservation is required")
	}
	return nil
}

func validateCommitRequest(request EGSSequenceCommitRequest) error {
	if err := validateReservation(request.Reservation); err != nil {
		return err
	}
	if strings.TrimSpace(request.DocumentType) == "" {
		return errors.New("document type is required")
	}
	if request.State != "" {
		switch request.State {
		case EGSDocumentQueued, EGSDocumentSubmitted, EGSDocumentAccepted,
			EGSDocumentWarning, EGSDocumentRejected, EGSDocumentAmbiguous:
		default:
			return fmt.Errorf("invalid EGS document state %q", request.State)
		}
	}
	if len(request.Snapshot) == 0 && len(request.Payload) == 0 {
		return errors.New("immutable document snapshot is required")
	}
	return nil
}

func validateIdentity(identity EGSSequenceIdentity) error {
	if strings.TrimSpace(identity.EGSSequenceID) == "" ||
		strings.TrimSpace(identity.VATNumber) == "" ||
		strings.TrimSpace(identity.EGSID) == "" {
		return errors.New("EGS sequence ID, VAT number, and EGS ID are required")
	}
	if identity.TenantID == 0 || identity.BranchID == 0 {
		return errors.New("tenant ID and branch ID are required")
	}
	return nil
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func stringPointer(value string) *string {
	return &value
}
