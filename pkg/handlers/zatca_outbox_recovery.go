package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/outbox"
)

type zatcaDeadLetterEventResponse struct {
	ID            uint64  `json:"id"`
	EventID       string  `json:"event_id"`
	DocumentKey   string  `json:"document_key"`
	Subject       string  `json:"subject"`
	Status        string  `json:"status"`
	AttemptCount  uint32  `json:"attempt_count"`
	NextAttemptAt string  `json:"next_attempt_at"`
	LastError     *string `json:"last_error,omitempty"`
	CreatedAt     string  `json:"created_at"`
	PublishedAt   *string `json:"published_at,omitempty"`
	UpdatedAt     string  `json:"updated_at"`
}

type zatcaOutboxRecoveryAuditResponse struct {
	ID                   uint64  `json:"id"`
	OutboxID             uint64  `json:"outbox_id"`
	EventID              string  `json:"event_id"`
	IdempotencyKey       string  `json:"idempotency_key"`
	OperatorUserID       *int32  `json:"operator_user_id,omitempty"`
	OperatorUsername     string  `json:"operator_username"`
	Reason               string  `json:"reason"`
	PreviousStatus       string  `json:"previous_status"`
	PreviousAttemptCount uint32  `json:"previous_attempt_count"`
	PreviousLastError    *string `json:"previous_last_error,omitempty"`
	RecoveredAt          string  `json:"recovered_at"`
}

type zatcaOutboxReplayRequest struct {
	Reason string `json:"reason"`
}

func (h *handler) ListZatcaDeadLetterOutbox(c *gin.Context) {
	if !requireZatcaOutboxAdmin(c) {
		return
	}

	limit := outbox.DefaultRecoveryListLimit
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid outbox limit"})
			return
		}
		limit = int32(value)
	}

	events, err := outbox.ListDeadLetterEvents(c.Request.Context(), h.DB, limit)
	if err != nil {
		writeZatcaOutboxRecoveryError(c, err)
		return
	}

	response := make([]zatcaDeadLetterEventResponse, 0, len(events))
	for _, event := range events {
		response = append(response, deadLetterEventResponse(event))
	}
	c.JSON(http.StatusOK, response)
}

func (h *handler) ListZatcaOutboxRecoveryAudit(c *gin.Context) {
	if !requireZatcaOutboxAdmin(c) {
		return
	}

	audits, err := outbox.ListRecoveryAudit(c.Request.Context(), h.DB, c.Param("event_id"))
	if err != nil {
		writeZatcaOutboxRecoveryError(c, err)
		return
	}

	response := make([]zatcaOutboxRecoveryAuditResponse, 0, len(audits))
	for _, audit := range audits {
		response = append(response, recoveryAuditResponse(audit))
	}
	c.JSON(http.StatusOK, response)
}

func (h *handler) ReplayZatcaOutbox(c *gin.Context) {
	if !requireZatcaOutboxAdmin(c) {
		return
	}

	var request zatcaOutboxReplayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": invalidRequestDetail(err)})
		return
	}

	userID, username, ok := outboxRecoveryOperator(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": ErrInvalidTokenClaims})
		return
	}

	result, err := outbox.RecoverDeadLetter(c.Request.Context(), h.DB, outbox.RecoveryRequest{
		EventID:          c.Param("event_id"),
		IdempotencyKey:   c.GetHeader("Idempotency-Key"),
		OperatorUserID:   userID,
		OperatorUsername: username,
		Reason:           request.Reason,
	})
	if err != nil {
		writeZatcaOutboxRecoveryError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"event":           deadLetterEventResponse(result.Event),
		"audit":           recoveryAuditResponse(result.Audit),
		"already_applied": result.AlreadyApplied,
	})
}

func requireZatcaOutboxAdmin(c *gin.Context) bool {
	if authenticatedRole(c) == RoleAdmin {
		return true
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"detail": "insufficient permissions",
		"error":  "forbidden",
	})
	return false
}

func outboxRecoveryOperator(c *gin.Context) (*int32, string, bool) {
	userID := c.GetInt64("user_id")
	username := strings.TrimSpace(c.GetString("username"))
	value, exists := c.Get("decoded_jwt")
	if claims, ok := value.(*model.Claims); exists && ok && claims != nil {
		if userID == 0 {
			userID = claims.Id
		}
		if username == "" {
			username = strings.TrimSpace(claims.Username)
		}
	}
	if userID <= 0 || userID > int64(^uint32(0)>>1) || username == "" {
		return nil, "", false
	}
	id := int32(userID)
	return &id, username, true
}

func writeZatcaOutboxRecoveryError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	detail := "outbox recovery failed"
	switch {
	case errors.Is(err, outbox.ErrRecoveryInvalidRequest):
		status = http.StatusBadRequest
		detail = err.Error()
	case errors.Is(err, outbox.ErrRecoveryEventNotFound):
		status = http.StatusNotFound
		detail = err.Error()
	case errors.Is(err, outbox.ErrRecoveryNotDeadLetter),
		errors.Is(err, outbox.ErrRecoveryIdempotencyConflict):
		status = http.StatusConflict
		detail = err.Error()
	default:
		log.Printf("ZATCA outbox recovery: %v", err)
	}
	c.JSON(status, gin.H{"detail": detail})
}

func deadLetterEventResponse(event db.Outbox) zatcaDeadLetterEventResponse {
	response := zatcaDeadLetterEventResponse{
		ID:            event.ID,
		EventID:       event.EventID,
		DocumentKey:   event.DocumentKey,
		Subject:       event.Subject,
		Status:        event.Status,
		AttemptCount:  event.AttemptCount,
		NextAttemptAt: event.NextAttemptAt.UTC().Format(time.RFC3339),
		LastError:     event.LastError,
		CreatedAt:     event.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     event.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if event.PublishedAt != nil {
		value := event.PublishedAt.UTC().Format(time.RFC3339)
		response.PublishedAt = &value
	}
	return response
}

func recoveryAuditResponse(audit db.OutboxRecoveryAudit) zatcaOutboxRecoveryAuditResponse {
	return zatcaOutboxRecoveryAuditResponse{
		ID:                   audit.ID,
		OutboxID:             audit.OutboxID,
		EventID:              audit.EventID,
		IdempotencyKey:       audit.IdempotencyKey,
		OperatorUserID:       audit.OperatorUserID,
		OperatorUsername:     audit.OperatorUsername,
		Reason:               audit.Reason,
		PreviousStatus:       audit.PreviousStatus,
		PreviousAttemptCount: audit.PreviousAttemptCount,
		PreviousLastError:    audit.PreviousLastError,
		RecoveredAt:          audit.RecoveredAt.UTC().Format(time.RFC3339),
	}
}
