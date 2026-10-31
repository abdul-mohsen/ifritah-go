package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/gin-gonic/gin"
)

const (
	zatcaCredentialTypeCompliance = "compliance"
	zatcaCredentialTypeProduction = "production"
	zatcaCredentialLifecycleLimit = 200
)

type zatcaCredentialIdentityRequest struct {
	TenantID       uint64 `json:"tenant_id" binding:"required"`
	CredentialType string `json:"credential_type"`
}

type createZatcaCredentialRequest struct {
	TenantID       uint64 `json:"tenant_id" binding:"required"`
	EGSSequenceID  string `json:"egs_sequence_id" binding:"required"`
	CredentialType string `json:"credential_type"`
}

type issueZatcaCredentialRequest struct {
	TenantID             uint64     `json:"tenant_id" binding:"required"`
	CredentialType       string     `json:"credential_type"`
	CertificateSerial    string     `json:"certificate_serial" binding:"required"`
	CertificateExpiresAt *time.Time `json:"certificate_expires_at" binding:"required"`
}

type failZatcaCredentialRenewalRequest struct {
	TenantID       uint64 `json:"tenant_id" binding:"required"`
	CredentialType string `json:"credential_type"`
	LastError      string `json:"last_error" binding:"required"`
}

type revokeZatcaCredentialRequest struct {
	TenantID       uint64 `json:"tenant_id" binding:"required"`
	CredentialType string `json:"credential_type"`
	Reason         string `json:"reason"`
}

func (h *handler) ListZatcaCredentials(c *gin.Context) {
	branchID, err := parseUint32Param(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return
	}
	tenantID, err := parseTenantID(c.Query("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "tenant_id is required"})
		return
	}

	rows, err := h.queries.ListZatcaEGSCredentialsByBranch(c.Request.Context(), db.ListZatcaEGSCredentialsByBranchParams{
		TenantID: tenantID,
		BranchID: branchID,
	})
	if err != nil {
		log.Printf("ListZatcaCredentials: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load ZATCA credentials"})
		return
	}
	if rows == nil {
		rows = []db.ListZatcaEGSCredentialsByBranchRow{}
	}
	c.JSON(http.StatusOK, rows)
}

func (h *handler) ListZatcaCredentialLifecycle(c *gin.Context) {
	tenantID, err := parseTenantID(c.Query("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "tenant_id is required"})
		return
	}

	limit := 50
	if value := c.Query("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > zatcaCredentialLifecycleLimit {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "limit must be between 1 and 200"})
			return
		}
	}

	expiresBefore := time.Now().Add(24 * time.Hour)
	if value := c.Query("expires_before"); value != "" {
		expiresBefore, err = time.Parse(time.RFC3339, value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "expires_before must be RFC3339"})
			return
		}
	}

	rows, err := h.queries.ListZatcaEGSCredentialsForLifecycle(c.Request.Context(), db.ListZatcaEGSCredentialsForLifecycleParams{
		TenantID:      tenantID,
		ExpiresBefore: timePointer(expiresBefore),
		Limit:         int32(limit),
	})
	if err != nil {
		log.Printf("ListZatcaCredentialLifecycle: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load ZATCA credential lifecycle"})
		return
	}
	if rows == nil {
		rows = []db.ListZatcaEGSCredentialsForLifecycleRow{}
	}
	c.JSON(http.StatusOK, rows)
}

func (h *handler) GetZatcaCredential(c *gin.Context) {
	branchID, egsSequenceID, ok := parseCredentialPath(c)
	if !ok {
		return
	}
	tenantID, err := parseTenantID(c.Query("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "tenant_id is required"})
		return
	}

	credential, err := h.queries.GetZatcaEGSCredential(c.Request.Context(), db.GetZatcaEGSCredentialParams{
		TenantID:       tenantID,
		BranchID:       branchID,
		EgsSequenceID:  egsSequenceID,
		CredentialType: normalizeCredentialType(c.Query("credential_type")),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"detail": "ZATCA credential not found"})
			return
		}
		log.Printf("GetZatcaCredential: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load ZATCA credential"})
		return
	}
	c.JSON(http.StatusOK, credential)
}

func (h *handler) CreateZatcaCredential(c *gin.Context) {
	branchID, err := parseUint32Param(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return
	}

	var req createZatcaCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}
	req.EGSSequenceID = strings.TrimSpace(req.EGSSequenceID)
	if req.TenantID == 0 || req.EGSSequenceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "tenant_id and egs_sequence_id are required"})
		return
	}
	req.CredentialType = normalizeCredentialType(req.CredentialType)
	if !validCredentialType(req.CredentialType) {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid credential_type"})
		return
	}

	result, err := h.queries.CreateZatcaEGSCredential(c.Request.Context(), db.CreateZatcaEGSCredentialParams{
		TenantID:       req.TenantID,
		BranchID:       branchID,
		EgsSequenceID:  req.EGSSequenceID,
		CredentialType: req.CredentialType,
	})
	if err != nil {
		if IsDuplicate(err) {
			c.JSON(http.StatusConflict, gin.H{"detail": "ZATCA credential already exists"})
			return
		}
		log.Printf("CreateZatcaCredential: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to create ZATCA credential"})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("CreateZatcaCredential last insert ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to create ZATCA credential"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":               id,
		"tenant_id":        req.TenantID,
		"branch_id":        branchID,
		"egs_sequence_id":  req.EGSSequenceID,
		"credential_type":  req.CredentialType,
		"status":           "pending",
		"renewal_state":    "none",
		"revocation_state": "none",
	})
}

func (h *handler) RequestZatcaCredentialRenewal(c *gin.Context) {
	branchID, egsSequenceID, ok := parseCredentialPath(c)
	if !ok {
		return
	}
	var req zatcaCredentialIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}
	credentialType, valid := credentialTypeFromRequest(req.CredentialType)
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid credential_type"})
		return
	}

	result, err := h.queries.RequestZatcaEGSCredentialRenewal(c.Request.Context(), db.RequestZatcaEGSCredentialRenewalParams{
		RequestedAt:    timePointer(time.Now()),
		TenantID:       req.TenantID,
		BranchID:       branchID,
		EgsSequenceID:  egsSequenceID,
		CredentialType: credentialType,
	})
	if err != nil {
		log.Printf("RequestZatcaCredentialRenewal: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to request ZATCA credential renewal"})
		return
	}
	if !requireLifecycleUpdate(c, result) {
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "pending", "renewal_state": "pending"})
}

func (h *handler) RequestZatcaCredentialRevocation(c *gin.Context) {
	branchID, egsSequenceID, ok := parseCredentialPath(c)
	if !ok {
		return
	}
	var req zatcaCredentialIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}
	credentialType, valid := credentialTypeFromRequest(req.CredentialType)
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid credential_type"})
		return
	}

	result, err := h.queries.RequestZatcaEGSCredentialRevocation(c.Request.Context(), db.RequestZatcaEGSCredentialRevocationParams{
		RequestedAt:    timePointer(time.Now()),
		TenantID:       req.TenantID,
		BranchID:       branchID,
		EgsSequenceID:  egsSequenceID,
		CredentialType: credentialType,
	})
	if err != nil {
		log.Printf("RequestZatcaCredentialRevocation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to request ZATCA credential revocation"})
		return
	}
	if !requireLifecycleUpdate(c, result) {
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "pending", "revocation_state": "pending"})
}

func (h *handler) RecordZatcaCredentialIssued(c *gin.Context) {
	branchID, egsSequenceID, ok := parseCredentialPath(c)
	if !ok {
		return
	}

	var req issueZatcaCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}
	credentialType, valid := credentialTypeFromRequest(req.CredentialType)
	if !valid || strings.TrimSpace(req.CertificateSerial) == "" || req.CertificateExpiresAt == nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "certificate serial, expiry, and credential_type are required"})
		return
	}
	if !req.CertificateExpiresAt.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "certificate expiry must be in the future"})
		return
	}

	result, err := h.queries.RecordZatcaEGSCredentialIssued(c.Request.Context(), db.RecordZatcaEGSCredentialIssuedParams{
		CertificateSerial:    stringPointer(strings.TrimSpace(req.CertificateSerial)),
		CertificateExpiresAt: req.CertificateExpiresAt,
		IssuedAt:             timePointer(time.Now()),
		TenantID:             req.TenantID,
		BranchID:             branchID,
		EgsSequenceID:        egsSequenceID,
		CredentialType:       credentialType,
	})
	if err != nil {
		log.Printf("RecordZatcaCredentialIssued: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to activate ZATCA credential"})
		return
	}
	if !requireLifecycleUpdate(c, result) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "active", "renewal_state": "completed", "revocation_state": "none"})
}

func (h *handler) MarkZatcaCredentialRenewalFailed(c *gin.Context) {
	branchID, egsSequenceID, ok := parseCredentialPath(c)
	if !ok {
		return
	}

	var req failZatcaCredentialRenewalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}
	credentialType, valid := credentialTypeFromRequest(req.CredentialType)
	if !valid || strings.TrimSpace(req.LastError) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "last_error and credential_type are required"})
		return
	}

	result, err := h.queries.MarkZatcaEGSCredentialRenewalFailed(c.Request.Context(), db.MarkZatcaEGSCredentialRenewalFailedParams{
		LastError:      stringPointer(strings.TrimSpace(req.LastError)),
		TenantID:       req.TenantID,
		BranchID:       branchID,
		EgsSequenceID:  egsSequenceID,
		CredentialType: credentialType,
	})
	if err != nil {
		log.Printf("MarkZatcaCredentialRenewalFailed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to record ZATCA credential renewal failure"})
		return
	}
	if !requireLifecycleUpdate(c, result) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "pending", "renewal_state": "failed"})
}

func (h *handler) MarkZatcaCredentialExpired(c *gin.Context) {
	branchID, egsSequenceID, ok := parseCredentialPath(c)
	if !ok {
		return
	}

	var req zatcaCredentialIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}
	credentialType, valid := credentialTypeFromRequest(req.CredentialType)
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid credential_type"})
		return
	}

	result, err := h.queries.MarkZatcaEGSCredentialExpired(c.Request.Context(), db.MarkZatcaEGSCredentialExpiredParams{
		TenantID:       req.TenantID,
		BranchID:       branchID,
		EgsSequenceID:  egsSequenceID,
		CredentialType: credentialType,
		AsOf:           timePointer(time.Now()),
	})
	if err != nil {
		log.Printf("MarkZatcaCredentialExpired: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to expire ZATCA credential"})
		return
	}
	if !requireLifecycleUpdate(c, result) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "expired"})
}

func (h *handler) MarkZatcaCredentialRevoked(c *gin.Context) {
	branchID, egsSequenceID, ok := parseCredentialPath(c)
	if !ok {
		return
	}

	var req revokeZatcaCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}
	credentialType, valid := credentialTypeFromRequest(req.CredentialType)
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid credential_type"})
		return
	}
	reason := strings.TrimSpace(req.Reason)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	result, err := h.queries.MarkZatcaEGSCredentialRevoked(c.Request.Context(), db.MarkZatcaEGSCredentialRevokedParams{
		RevokedAt:      timePointer(time.Now()),
		Reason:         reasonPtr,
		TenantID:       req.TenantID,
		BranchID:       branchID,
		EgsSequenceID:  egsSequenceID,
		CredentialType: credentialType,
	})
	if err != nil {
		log.Printf("MarkZatcaCredentialRevoked: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to revoke ZATCA credential"})
		return
	}
	if !requireLifecycleUpdate(c, result) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked", "revocation_state": "revoked"})
}

func parseCredentialPath(c *gin.Context) (uint32, string, bool) {
	branchID, err := parseUint32Param(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return 0, "", false
	}
	egsSequenceID := strings.TrimSpace(c.Param("egs_sequence_id"))
	if egsSequenceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "egs_sequence_id is required"})
		return 0, "", false
	}
	return branchID, egsSequenceID, true
}

func parseTenantID(value string) (uint64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, strconv.ErrSyntax
	}
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

func normalizeCredentialType(value string) string {
	if strings.TrimSpace(value) == "" {
		return zatcaCredentialTypeProduction
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func credentialTypeFromRequest(value string) (string, bool) {
	value = normalizeCredentialType(value)
	return value, validCredentialType(value)
}

func validCredentialType(value string) bool {
	return value == zatcaCredentialTypeCompliance || value == zatcaCredentialTypeProduction
}

func stringPointer(value string) *string {
	return &value
}

func requireLifecycleUpdate(c *gin.Context, result sql.Result) bool {
	affected, err := result.RowsAffected()
	if err != nil {
		log.Printf("ZATCA credential lifecycle rows affected: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update ZATCA credential"})
		return false
	}
	if affected != 1 {
		c.JSON(http.StatusConflict, gin.H{"detail": "ZATCA credential lifecycle transition is not allowed"})
		return false
	}
	return true
}

func timePointer(value time.Time) *time.Time {
	return &value
}
