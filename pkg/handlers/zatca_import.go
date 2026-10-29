package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/gin-gonic/gin"
)

const (
	maxZatcaImportBytes        = 5 << 20
	maxZatcaImportRequestBytes = 8 << 20
)

var (
	errZatcaImportConflict = errors.New("ZATCA import workflow conflict")
	errZatcaImportInvalid  = errors.New("invalid ZATCA signed-UBL import")
)

var zatcaImportFailureClasses = map[string]struct{}{
	"":                  {},
	"malformed":         {},
	"unsafe":            {},
	"oversized":         {},
	"required_fields":   {},
	"signature":         {},
	"trust_unavailable": {},
	"schema":            {},
	"unsupported":       {},
}

// CreateZatcaImport persists a T030-shaped import result. The request is an
// adapter contract only: parsing and cryptographic verification remain owned by
// the separate T030 service.
func (h *handler) CreateZatcaImport(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxZatcaImportRequestBytes)

	var request model.ZatcaImportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid signed-UBL import request"})
		return
	}
	normalized, originalBytes, resultData, err := normalizeZatcaImportRequest(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	if h == nil || h.DB == nil || h.queries == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "ZATCA import persistence is unavailable"})
		return
	}

	merchantID := int32(GetSessionInfo(c).id)
	if merchantID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "authenticated merchant is required"})
		return
	}
	actorID := merchantID
	ctx := c.Request.Context()

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to start signed-UBL import transaction"})
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	idempotencyKey := optionalString(normalized.IdempotencyKey)
	result, err := qtx.InsertZatcaImportJob(ctx, db.InsertZatcaImportJobParams{
		MerchantID:        merchantID,
		BranchID:          normalized.BranchID,
		SourceContentHash: normalized.Source.ContentHash,
		IdempotencyKey:    idempotencyKey,
		CreatedBy:         actorID,
	})
	if err != nil {
		if IsDuplicate(err) {
			_ = tx.Rollback()
			existingID, existingStatus, existingVersion, existingHash, found := h.findExistingZatcaImport(ctx, merchantID, normalized.Source.ContentHash, idempotencyKey)
			if found {
				c.JSON(http.StatusOK, zatcaImportJobResponse(existingID, existingStatus, existingVersion, existingHash, true))
				return
			}
			c.JSON(http.StatusConflict, gin.H{"detail": "signed-UBL import already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to create signed-UBL import job"})
		return
	}
	jobID, err := result.LastInsertId()
	if err != nil || jobID <= 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "signed-UBL import job id was not returned"})
		return
	}

	original := sql.NullString{}
	if len(originalBytes) > 0 {
		original = sql.NullString{String: string(originalBytes), Valid: true}
	}
	if err := qtx.InsertZatcaImportEvidence(ctx, db.InsertZatcaImportEvidenceParams{
		ImportJobID:        uint64(jobID),
		MerchantID:         merchantID,
		SourceReference:    normalized.Source.ContentReference,
		SourceOriginalName: optionalString(normalized.Source.OriginalName),
		SourceMimeType:     normalized.Source.MIMEType,
		SourceProvenance:   normalized.Source.Provenance,
		SourceContentHash:  normalized.Source.ContentHash,
		SourceSize:         uint64(normalized.Source.Size),
		OriginalBytes:      original,
		CanonicalData:      normalized.Canonical,
		ParsedData:         normalized.Parsed,
		SignatureData:      mustJSON(normalized.Signature),
		ValidationData:     mustJSON(normalized.Validation),
		ResultData:         resultData,
		FailureClass:       optionalString(normalized.FailureClass),
		FailureMessage:     optionalString(normalized.FailureMessage),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to persist signed-UBL evidence"})
		return
	}

	for _, issue := range normalized.Validation.Issues {
		severity := strings.ToLower(strings.TrimSpace(issue.Severity))
		if err := qtx.InsertZatcaImportValidationError(ctx, db.InsertZatcaImportValidationErrorParams{
			ImportJobID: uint64(jobID),
			MerchantID:  merchantID,
			Severity:    severity,
			ErrorCode:   strings.TrimSpace(issue.Code),
			FieldKey:    optionalString(issue.Field),
			Message:     strings.TrimSpace(issue.Message),
			Details:     issue.Details,
			CreatedBy:   actorID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to persist signed-UBL validation evidence"})
			return
		}
	}

	reason := "signed-UBL import received; review required"
	if normalized.FailureClass != "" {
		reason = "signed-UBL import requires review: " + normalized.FailureClass
	}
	if err := qtx.InsertZatcaImportStatusHistory(ctx, db.InsertZatcaImportStatusHistoryParams{
		ImportJobID:     uint64(jobID),
		MerchantID:      merchantID,
		FromStatus:      nil,
		ToStatus:        model.ZatcaImportStatusReview,
		TransitionKey:   "create:" + strconv.FormatInt(jobID, 10),
		WorkflowVersion: 1,
		Reason:          &reason,
		CreatedBy:       actorID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to persist signed-UBL import status"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to commit signed-UBL import"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":               uint64(jobID),
		"status":           model.ZatcaImportStatusReview,
		"workflow_version": uint64(1),
		"review_required":  true,
		"replayed":         false,
	})
}

// ListZatcaImports returns tenant-scoped import jobs without loading the
// potentially large immutable evidence blob for every row.
func (h *handler) ListZatcaImports(c *gin.Context) {
	merchantID := int32(GetSessionInfo(c).id)
	limit := int64(50)
	if raw := c.DefaultQuery("limit", "50"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 32); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	status := strings.TrimSpace(c.Query("status"))
	if !validZatcaImportStatusFilter(status) {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid signed-UBL import status"})
		return
	}
	rows, err := h.queries.ListZatcaImportJobs(c.Request.Context(), db.ListZatcaImportJobsParams{
		MerchantID:   merchantID,
		StatusFilter: status,
		Limit:        int32(limit),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load signed-UBL imports"})
		return
	}
	out := make([]model.ZatcaImportJob, 0, len(rows))
	for _, row := range rows {
		out = append(out, zatcaImportJobFromListRow(row))
	}
	c.JSON(http.StatusOK, out)
}

// GetZatcaImport returns all durable evidence and lifecycle records for a
// tenant-owned import job.
func (h *handler) GetZatcaImport(c *gin.Context) {
	id, ok := parseZatcaImportID(c)
	if !ok {
		return
	}
	merchantID := int32(GetSessionInfo(c).id)
	job, err := h.queries.GetZatcaImportJobForTenant(c.Request.Context(), db.GetZatcaImportJobForTenantParams{
		ID: id, MerchantID: merchantID,
	})
	if err != nil {
		zatcaImportReadError(c, err)
		return
	}
	response, err := h.loadZatcaImportResponse(c, job, merchantID)
	if err != nil {
		log.Printf("GetZatcaImport: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load signed-UBL import evidence"})
		return
	}
	c.JSON(http.StatusOK, response)
}

// ReviewZatcaImport appends a review decision and optionally approves the
// import. It never mutates outbound submission state.
func (h *handler) ReviewZatcaImport(c *gin.Context) {
	var request model.ZatcaImportReviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid signed-UBL review request"})
		return
	}
	decision := normalizeZatcaReviewDecision(request.Decision)
	if decision == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "decision must be approve, review, or request_changes"})
		return
	}
	id, ok := parseZatcaImportID(c)
	if !ok {
		return
	}
	merchantID := int32(GetSessionInfo(c).id)
	actorID := merchantID
	ctx := c.Request.Context()

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to start signed-UBL review transaction"})
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)
	job, err := qtx.GetZatcaImportJobForUpdate(ctx, db.GetZatcaImportJobForUpdateParams{ID: id, MerchantID: merchantID})
	if err != nil {
		zatcaImportTransitionError(c, err)
		return
	}
	expected := job.WorkflowVersion
	if request.ExpectedWorkflowVersion != nil {
		expected = *request.ExpectedWorkflowVersion
	}
	if expected != job.WorkflowVersion {
		c.JSON(http.StatusConflict, gin.H{"detail": errZatcaImportConflict.Error()})
		return
	}
	if (decision == "approve" && job.Status != model.ZatcaImportStatusReview) ||
		(decision != "approve" && job.Status != model.ZatcaImportStatusReview && job.Status != model.ZatcaImportStatusApproved) {
		c.JSON(http.StatusConflict, gin.H{"detail": "signed-UBL import is not reviewable in its current state"})
		return
	}

	reason := strings.TrimSpace(request.Reason)
	if len(reason) > 1024 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "review reason is too long"})
		return
	}
	observed := job.WorkflowVersion
	if err := qtx.InsertZatcaImportReview(ctx, db.InsertZatcaImportReviewParams{
		ImportJobID:             id,
		MerchantID:              merchantID,
		Decision:                decision,
		Reason:                  optionalString(reason),
		ExpectedWorkflowVersion: request.ExpectedWorkflowVersion,
		ObservedWorkflowVersion: observed,
		CreatedBy:               actorID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to persist signed-UBL review"})
		return
	}

	var updateResult sql.Result
	if decision == "approve" {
		updateResult, err = qtx.TransitionZatcaImportToApproved(ctx, db.TransitionZatcaImportToApprovedParams{
			ReviewedBy:              optionalInt32(actorID),
			ID:                      id,
			MerchantID:              merchantID,
			ExpectedWorkflowVersion: expected,
		})
	} else {
		updateResult, err = qtx.TouchZatcaImportReview(ctx, db.TouchZatcaImportReviewParams{
			ReviewedBy:              optionalInt32(actorID),
			ID:                      id,
			MerchantID:              merchantID,
			ExpectedWorkflowVersion: expected,
		})
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update signed-UBL review state"})
		return
	}
	if changed, rowsErr := updateResult.RowsAffected(); rowsErr != nil || changed != 1 {
		c.JSON(http.StatusConflict, gin.H{"detail": errZatcaImportConflict.Error()})
		return
	}
	nextStatus := job.Status
	if decision == "approve" {
		nextStatus = model.ZatcaImportStatusApproved
	}
	reviewReason := reason
	if reviewReason == "" {
		reviewReason = "signed-UBL import review recorded"
	}
	if err := qtx.InsertZatcaImportStatusHistory(ctx, db.InsertZatcaImportStatusHistoryParams{
		ImportJobID:     id,
		MerchantID:      merchantID,
		FromStatus:      &job.Status,
		ToStatus:        nextStatus,
		TransitionKey:   fmt.Sprintf("review:%d:%s:%d", id, decision, expected),
		WorkflowVersion: expected + 1,
		Reason:          &reviewReason,
		CreatedBy:       actorID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to persist signed-UBL review status"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to commit signed-UBL review"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "status": nextStatus, "workflow_version": expected + 1})
}

// PostZatcaImport marks an approved import as posted in the import lane only.
func (h *handler) PostZatcaImport(c *gin.Context) {
	var request model.ZatcaImportPostRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid signed-UBL post request"})
		return
	}
	if len(strings.TrimSpace(request.PostReference)) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "post reference is too long"})
		return
	}
	id, ok := parseZatcaImportID(c)
	if !ok {
		return
	}
	merchantID := int32(GetSessionInfo(c).id)
	actorID := merchantID
	ctx := c.Request.Context()
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to start signed-UBL post transaction"})
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)
	job, err := qtx.GetZatcaImportJobForUpdate(ctx, db.GetZatcaImportJobForUpdateParams{ID: id, MerchantID: merchantID})
	if err != nil {
		zatcaImportTransitionError(c, err)
		return
	}
	expected := job.WorkflowVersion
	if request.ExpectedWorkflowVersion != nil {
		expected = *request.ExpectedWorkflowVersion
	}
	if expected != job.WorkflowVersion || job.Status != model.ZatcaImportStatusApproved {
		c.JSON(http.StatusConflict, gin.H{"detail": "signed-UBL import must be approved and current before posting"})
		return
	}
	updateResult, err := qtx.TransitionZatcaImportToPosted(ctx, db.TransitionZatcaImportToPostedParams{
		PostedBy:                optionalInt32(actorID),
		PostReference:           optionalString(strings.TrimSpace(request.PostReference)),
		ID:                      id,
		MerchantID:              merchantID,
		ExpectedWorkflowVersion: expected,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update signed-UBL post state"})
		return
	}
	if changed, rowsErr := updateResult.RowsAffected(); rowsErr != nil || changed != 1 {
		c.JSON(http.StatusConflict, gin.H{"detail": errZatcaImportConflict.Error()})
		return
	}
	reason := "signed-UBL import posted"
	if err := qtx.InsertZatcaImportStatusHistory(ctx, db.InsertZatcaImportStatusHistoryParams{
		ImportJobID:     id,
		MerchantID:      merchantID,
		FromStatus:      &job.Status,
		ToStatus:        model.ZatcaImportStatusPosted,
		TransitionKey:   fmt.Sprintf("post:%d:%d", id, expected),
		WorkflowVersion: expected + 1,
		Reason:          &reason,
		CreatedBy:       actorID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to persist signed-UBL post status"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to commit signed-UBL post"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "status": model.ZatcaImportStatusPosted, "workflow_version": expected + 1})
}

// RejectZatcaImport records a review rejection without touching outbound
// submission state.
func (h *handler) RejectZatcaImport(c *gin.Context) {
	var request model.ZatcaImportRejectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid signed-UBL reject request"})
		return
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" || len(reason) > 1024 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "rejection reason is required and must be at most 1024 characters"})
		return
	}
	id, ok := parseZatcaImportID(c)
	if !ok {
		return
	}
	merchantID := int32(GetSessionInfo(c).id)
	actorID := merchantID
	ctx := c.Request.Context()
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to start signed-UBL reject transaction"})
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)
	job, err := qtx.GetZatcaImportJobForUpdate(ctx, db.GetZatcaImportJobForUpdateParams{ID: id, MerchantID: merchantID})
	if err != nil {
		zatcaImportTransitionError(c, err)
		return
	}
	expected := job.WorkflowVersion
	if request.ExpectedWorkflowVersion != nil {
		expected = *request.ExpectedWorkflowVersion
	}
	if expected != job.WorkflowVersion || (job.Status != model.ZatcaImportStatusReview && job.Status != model.ZatcaImportStatusApproved) {
		c.JSON(http.StatusConflict, gin.H{"detail": "signed-UBL import is not rejectable in its current state"})
		return
	}
	updateResult, err := qtx.TransitionZatcaImportToRejected(ctx, db.TransitionZatcaImportToRejectedParams{
		RejectedBy:              optionalInt32(actorID),
		RejectedReason:          &reason,
		ID:                      id,
		MerchantID:              merchantID,
		ExpectedWorkflowVersion: expected,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update signed-UBL rejection state"})
		return
	}
	if changed, rowsErr := updateResult.RowsAffected(); rowsErr != nil || changed != 1 {
		c.JSON(http.StatusConflict, gin.H{"detail": errZatcaImportConflict.Error()})
		return
	}
	if err := qtx.InsertZatcaImportStatusHistory(ctx, db.InsertZatcaImportStatusHistoryParams{
		ImportJobID:     id,
		MerchantID:      merchantID,
		FromStatus:      &job.Status,
		ToStatus:        model.ZatcaImportStatusRejected,
		TransitionKey:   fmt.Sprintf("reject:%d:%d", id, expected),
		WorkflowVersion: expected + 1,
		Reason:          &reason,
		CreatedBy:       actorID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to persist signed-UBL rejection status"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to commit signed-UBL rejection"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "status": model.ZatcaImportStatusRejected, "workflow_version": expected + 1})
}

func normalizeZatcaImportRequest(request model.ZatcaImportRequest) (model.ZatcaImportRequest, []byte, json.RawMessage, error) {
	originalBytes := request.OriginalBytes
	if len(originalBytes) == 0 {
		originalBytes = request.Bytes
	}
	if len(originalBytes) == 0 && request.XMLContent != "" {
		originalBytes = []byte(request.XMLContent)
	}
	if len(originalBytes) > maxZatcaImportBytes {
		return request, nil, nil, fmt.Errorf("signed-UBL source exceeds %d bytes", maxZatcaImportBytes)
	}
	if len(request.OriginalBytes) > 0 && len(request.Bytes) > 0 && !bytes.Equal(request.OriginalBytes, request.Bytes) {
		return request, nil, nil, fmt.Errorf("%w: bytes and original_bytes differ", errZatcaImportInvalid)
	}
	hash := sha256.Sum256(originalBytes)
	computedHash := hex.EncodeToString(hash[:])
	sourceHash := strings.ToLower(strings.TrimSpace(request.Source.ContentHash))
	if sourceHash == "" && len(originalBytes) > 0 {
		sourceHash = computedHash
	}
	if sourceHash == "" || len(sourceHash) != sha256.Size*2 {
		return request, nil, nil, fmt.Errorf("%w: source content hash is required", errZatcaImportInvalid)
	}
	if _, err := hex.DecodeString(sourceHash); err != nil {
		return request, nil, nil, fmt.Errorf("%w: source content hash is not hexadecimal", errZatcaImportInvalid)
	}
	if len(originalBytes) > 0 && sourceHash != computedHash {
		return request, nil, nil, fmt.Errorf("%w: source content hash does not match original bytes", errZatcaImportInvalid)
	}
	if request.Source.Size < 0 {
		return request, nil, nil, fmt.Errorf("%w: source size must not be negative", errZatcaImportInvalid)
	}
	if request.Source.Size == 0 && len(originalBytes) > 0 {
		request.Source.Size = int64(len(originalBytes))
	}
	if request.Source.Size <= 0 {
		return request, nil, nil, fmt.Errorf("%w: source size is required", errZatcaImportInvalid)
	}
	if len(originalBytes) > 0 && request.Source.Size != int64(len(originalBytes)) {
		return request, nil, nil, fmt.Errorf("%w: source size does not match original bytes", errZatcaImportInvalid)
	}
	if request.Source.Size > maxZatcaImportBytes {
		return request, nil, nil, fmt.Errorf("signed-UBL source exceeds %d bytes", maxZatcaImportBytes)
	}
	request.Source.ContentHash = sourceHash
	request.Source.ContentReference = strings.TrimSpace(request.Source.ContentReference)
	if request.Source.ContentReference == "" {
		request.Source.ContentReference = "sha256:" + sourceHash
	}
	if len(request.Source.ContentReference) > 255 {
		return request, nil, nil, fmt.Errorf("%w: source reference is too long", errZatcaImportInvalid)
	}
	request.Source.MIMEType = strings.TrimSpace(request.Source.MIMEType)
	if request.Source.MIMEType == "" {
		request.Source.MIMEType = "application/xml"
	}
	if len(request.Source.MIMEType) > 100 {
		return request, nil, nil, fmt.Errorf("%w: source MIME type is too long", errZatcaImportInvalid)
	}
	request.Source.Provenance = strings.TrimSpace(request.Source.Provenance)
	if request.Source.Provenance == "" {
		request.Source.Provenance = "supplier_xml"
	}
	if len(request.Source.Provenance) > 128 {
		return request, nil, nil, fmt.Errorf("%w: source provenance is too long", errZatcaImportInvalid)
	}
	request.Source.OriginalName = strings.TrimSpace(request.Source.OriginalName)
	if len(request.Source.OriginalName) > 255 {
		return request, nil, nil, fmt.Errorf("%w: source original name is too long", errZatcaImportInvalid)
	}
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if len(request.IdempotencyKey) > 128 {
		return request, nil, nil, fmt.Errorf("%w: idempotency key is too long", errZatcaImportInvalid)
	}
	if len(request.Canonical) == 0 {
		request.Canonical = request.Parsed
	}
	if len(request.Parsed) == 0 {
		request.Parsed = request.Canonical
	}
	if len(request.Canonical) > 0 && !json.Valid(request.Canonical) {
		return request, nil, nil, fmt.Errorf("%w: canonical data is invalid JSON", errZatcaImportInvalid)
	}
	if len(request.Parsed) > 0 && !json.Valid(request.Parsed) {
		return request, nil, nil, fmt.Errorf("%w: parsed data is invalid JSON", errZatcaImportInvalid)
	}
	if request.Signature.Status == "" {
		request.Signature.Status = request.Validation.SignatureStatus
	}
	if request.Validation.SignatureStatus == "" {
		request.Validation.SignatureStatus = request.Signature.Status
	}
	if request.Validation.SignatureStatus == "" {
		request.Validation.SignatureStatus = "unsigned"
	}
	if !validZatcaSignatureStatus(request.Validation.SignatureStatus) {
		return request, nil, nil, fmt.Errorf("%w: unsupported signature status", errZatcaImportInvalid)
	}
	if request.Signature.Status != "" && !validZatcaSignatureStatus(request.Signature.Status) {
		return request, nil, nil, fmt.Errorf("%w: unsupported signature evidence status", errZatcaImportInvalid)
	}
	if request.Signature.Status != "" && request.Signature.Status != request.Validation.SignatureStatus {
		return request, nil, nil, fmt.Errorf("%w: signature evidence status does not match validation status", errZatcaImportInvalid)
	}
	if request.Validation.SignatureStatus == "verified" &&
		(!request.Signature.Present || request.Signature.Status != "verified") {
		return request, nil, nil, fmt.Errorf("%w: verified validation requires verified signature evidence", errZatcaImportInvalid)
	}
	if request.Validation.SchemaStatus != "" &&
		request.Validation.SchemaStatus != "schema_valid" &&
		request.Validation.SchemaStatus != "schema_invalid" &&
		request.Validation.SchemaStatus != "schema_unavailable" {
		return request, nil, nil, fmt.Errorf("%w: unsupported schema status", errZatcaImportInvalid)
	}
	if request.FailureClass != "" {
		if _, ok := zatcaImportFailureClasses[strings.TrimSpace(request.FailureClass)]; !ok {
			return request, nil, nil, fmt.Errorf("%w: unsupported failure class", errZatcaImportInvalid)
		}
		request.FailureClass = strings.TrimSpace(request.FailureClass)
	}
	if len(request.FailureMessage) > 1024 {
		return request, nil, nil, fmt.Errorf("%w: failure message is too long", errZatcaImportInvalid)
	}
	for i := range request.Validation.Issues {
		issue := &request.Validation.Issues[i]
		issue.Code = strings.TrimSpace(issue.Code)
		issue.Field = strings.TrimSpace(issue.Field)
		issue.Message = strings.TrimSpace(issue.Message)
		issue.Severity = strings.ToLower(strings.TrimSpace(issue.Severity))
		if issue.Code == "" || len(issue.Code) > 64 || issue.Message == "" || len(issue.Message) > 1024 {
			return request, nil, nil, fmt.Errorf("%w: invalid validation issue", errZatcaImportInvalid)
		}
		if issue.Severity != "warning" && issue.Severity != "error" {
			return request, nil, nil, fmt.Errorf("%w: validation issue severity must be warning or error", errZatcaImportInvalid)
		}
		if len(issue.Field) > 128 || (len(issue.Details) > 0 && !json.Valid(issue.Details)) {
			return request, nil, nil, fmt.Errorf("%w: invalid validation issue details", errZatcaImportInvalid)
		}
	}
	resultData, err := json.Marshal(request)
	if err != nil {
		return request, nil, nil, fmt.Errorf("marshal signed-UBL import result: %w", err)
	}
	return request, originalBytes, resultData, nil
}

func (h *handler) findExistingZatcaImport(ctx context.Context, merchantID int32, hash string, idempotencyKey *string) (uint64, string, uint64, string, bool) {
	if idempotencyKey != nil {
		row, err := h.queries.GetZatcaImportJobByIdempotencyKey(ctx, db.GetZatcaImportJobByIdempotencyKeyParams{
			MerchantID: merchantID, IdempotencyKey: idempotencyKey,
		})
		if err == nil && row.SourceContentHash == hash {
			return row.ID, row.Status, row.WorkflowVersion, row.SourceContentHash, true
		}
	}
	row, err := h.queries.GetZatcaImportJobByContentHash(ctx, db.GetZatcaImportJobByContentHashParams{
		MerchantID: merchantID, SourceContentHash: hash,
	})
	if err != nil {
		return 0, "", 0, "", false
	}
	return row.ID, row.Status, row.WorkflowVersion, row.SourceContentHash, true
}

func (h *handler) loadZatcaImportResponse(c *gin.Context, job db.GetZatcaImportJobForTenantRow, merchantID int32) (model.ZatcaImportJob, error) {
	response := zatcaImportJobFromTenantRow(job)
	ctx := c.Request.Context()
	evidence, err := h.queries.GetZatcaImportEvidence(ctx, db.GetZatcaImportEvidenceParams{ImportJobID: job.ID, MerchantID: merchantID})
	if err != nil {
		return response, err
	}
	response.Evidence = zatcaImportEvidenceModel(evidence)
	errorsRows, err := h.queries.ListZatcaImportValidationErrors(ctx, db.ListZatcaImportValidationErrorsParams{ImportJobID: job.ID, MerchantID: merchantID})
	if err != nil {
		return response, err
	}
	for _, row := range errorsRows {
		response.ValidationErrors = append(response.ValidationErrors, zatcaImportValidationErrorModel(row))
	}
	reviews, err := h.queries.ListZatcaImportReviews(ctx, db.ListZatcaImportReviewsParams{ImportJobID: job.ID, MerchantID: merchantID})
	if err != nil {
		return response, err
	}
	for _, row := range reviews {
		response.Reviews = append(response.Reviews, zatcaImportReviewModel(row))
	}
	history, err := h.queries.ListZatcaImportStatusHistory(ctx, db.ListZatcaImportStatusHistoryParams{ImportJobID: job.ID, MerchantID: merchantID})
	if err != nil {
		return response, err
	}
	for _, row := range history {
		response.StatusHistory = append(response.StatusHistory, zatcaImportHistoryModel(row))
	}
	return response, nil
}

func zatcaImportEvidenceModel(row db.ZatcaImportEvidence) *model.ZatcaImportEvidence {
	var original []byte
	if row.OriginalBytes.Valid {
		original = []byte(row.OriginalBytes.String)
	}
	return &model.ZatcaImportEvidence{
		ID:                 row.ID,
		ImportJobID:        row.ImportJobID,
		MerchantID:         row.MerchantID,
		SourceReference:    row.SourceReference,
		SourceOriginalName: row.SourceOriginalName,
		SourceMIMEType:     row.SourceMimeType,
		SourceProvenance:   row.SourceProvenance,
		SourceContentHash:  row.SourceContentHash,
		SourceSize:         row.SourceSize,
		OriginalBytes:      original,
		CanonicalData:      row.CanonicalData,
		ParsedData:         row.ParsedData,
		SignatureData:      row.SignatureData,
		ValidationData:     row.ValidationData,
		ResultData:         row.ResultData,
		FailureClass:       row.FailureClass,
		FailureMessage:     row.FailureMessage,
		CreatedAt:          row.CreatedAt,
	}
}

func zatcaImportJobResponse(id uint64, status string, version uint64, hash string, replayed bool) gin.H {
	return gin.H{
		"id":                  id,
		"status":              status,
		"workflow_version":    version,
		"source_content_hash": hash,
		"replayed":            replayed,
	}
}

func zatcaImportJobFromTenantRow(row db.GetZatcaImportJobForTenantRow) model.ZatcaImportJob {
	return model.ZatcaImportJob{
		ID: row.ID, MerchantID: row.MerchantID, BranchID: row.BranchID, Status: row.Status,
		WorkflowVersion: row.WorkflowVersion, SourceContentHash: row.SourceContentHash,
		ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, PostedBy: row.PostedBy,
		PostedAt: row.PostedAt, PostReference: row.PostReference, RejectedBy: row.RejectedBy,
		RejectedAt: row.RejectedAt, RejectedReason: row.RejectedReason, CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func zatcaImportJobFromListRow(row db.ListZatcaImportJobsRow) model.ZatcaImportJob {
	return model.ZatcaImportJob{
		ID: row.ID, MerchantID: row.MerchantID, BranchID: row.BranchID, Status: row.Status,
		WorkflowVersion: row.WorkflowVersion, SourceContentHash: row.SourceContentHash,
		ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, PostedBy: row.PostedBy,
		PostedAt: row.PostedAt, PostReference: row.PostReference, RejectedBy: row.RejectedBy,
		RejectedAt: row.RejectedAt, RejectedReason: row.RejectedReason, CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func zatcaImportValidationErrorModel(row db.ZatcaImportValidationError) model.ZatcaImportValidationError {
	return model.ZatcaImportValidationError{
		ID: row.ID, ImportJobID: row.ImportJobID, MerchantID: row.MerchantID, Severity: row.Severity,
		ErrorCode: row.ErrorCode, FieldKey: row.FieldKey, Message: row.Message, Details: row.Details,
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
	}
}

func zatcaImportReviewModel(row db.ZatcaImportReview) model.ZatcaImportReview {
	return model.ZatcaImportReview{
		ID: row.ID, ImportJobID: row.ImportJobID, MerchantID: row.MerchantID, Decision: row.Decision,
		Reason: row.Reason, ExpectedWorkflowVersion: row.ExpectedWorkflowVersion,
		ObservedWorkflowVersion: row.ObservedWorkflowVersion, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
	}
}

func zatcaImportHistoryModel(row db.ZatcaImportStatusHistory) model.ZatcaImportStatusHistory {
	return model.ZatcaImportStatusHistory{
		ID: row.ID, ImportJobID: row.ImportJobID, MerchantID: row.MerchantID, FromStatus: row.FromStatus,
		ToStatus: row.ToStatus, TransitionKey: row.TransitionKey, WorkflowVersion: row.WorkflowVersion,
		Reason: row.Reason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
	}
}

func parseZatcaImportID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid signed-UBL import id"})
		return 0, false
	}
	return id, true
}

func zatcaImportReadError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"detail": "signed-UBL import not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load signed-UBL import"})
}

func zatcaImportTransitionError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"detail": "signed-UBL import not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load signed-UBL import workflow"})
}

func validZatcaImportStatusFilter(status string) bool {
	switch status {
	case "", model.ZatcaImportStatusReview, model.ZatcaImportStatusApproved,
		model.ZatcaImportStatusPosted, model.ZatcaImportStatusRejected:
		return true
	default:
		return false
	}
}

func normalizeZatcaReviewDecision(decision string) string {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "approve", "approved", "accept", "accepted":
		return "approve"
	case "review":
		return "review"
	case "request_changes", "request-change", "changes":
		return "request_changes"
	default:
		return ""
	}
}

func validZatcaSignatureStatus(status string) bool {
	switch status {
	case "verified", "unverified", "unsigned", "invalid":
		return true
	default:
		return false
	}
}

func optionalInt32(value int32) *int32 {
	if value <= 0 {
		return nil
	}
	return &value
}

func mustJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}
