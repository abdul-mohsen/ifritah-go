package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/gin-gonic/gin"
)

type BillDebit struct {
	BillID uint64 `json:"bill_id" binding:"required"`
	Note   string `json:"note" binding:"required"`
}

// DebitBill creates the durable debit source, pending submission, and outbox
// event in one authenticated transaction. XML/provider processing belongs to
// the daemon and is intentionally not part of this handler.
func (h *handler) DebitBill(c *gin.Context) {
	var request BillDebit
	if err := c.BindJSON(&request); err != nil {
		log.Printf("DebitBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}
	request.Note = strings.TrimSpace(request.Note)
	if request.BillID == 0 || request.Note == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	if h == nil || h.DB == nil || h.queries == nil || h.pub == nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	session := GetSessionInfo(c)
	merchantID := int32(session.id)
	ctx := c.Request.Context()

	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	parent, err := qtx.GetDebitNoteSourceContext(ctx, db.GetDebitNoteSourceContextParams{
		TenantID:   h.pub.dbName,
		BillID:     request.BillID,
		MerchantID: merchantID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			c.Status(http.StatusBadRequest)
			return
		}
		log.Printf("DebitBill source lookup: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if parent.BranchID == nil || *parent.BranchID == 0 {
		c.Status(http.StatusBadRequest)
		return
	}
	egsID, ok := debitEGSID(parent.EgsID)
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}

	state := int32(1)
	result, err := qtx.InsertDebitNote(ctx, db.InsertDebitNoteParams{
		BillID:         parent.BillID,
		MerchantID:     merchantID,
		BranchID:       *parent.BranchID,
		SequenceNumber: parent.SequenceNumber,
		EgsID:          egsID,
		State:          state,
		Note:           request.Note,
	})
	if err != nil {
		if IsDuplicate(err) {
			c.Status(http.StatusConflict)
			return
		}
		log.Printf("DebitBill insert: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	debitID, err := result.LastInsertId()
	if err != nil || debitID <= 0 {
		if err == nil {
			err = fmt.Errorf("invalid debit note id %d", debitID)
		}
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	documentID := fmt.Sprintf("%s:%d", zatcaDocDebit, debitID)
	stableKey := zatcaOutboxIdempotencyKey(
		h.pub.dbName,
		zatcaDocDebit,
		debitID,
		int64(*parent.BranchID),
		"",
	)
	if err := qtx.SetDebitNoteIdentity(ctx, db.SetDebitNoteIdentityParams{
		DocumentID:        &documentID,
		StableDocumentKey: &stableKey,
		ID:                uint64(debitID),
		MerchantID:        merchantID,
	}); err != nil {
		log.Printf("DebitBill identity: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := qtx.RegisterDebitZatcaSubmission(ctx, db.RegisterDebitZatcaSubmissionParams{
		BillID:            parent.BillID,
		SourceID:          uint64(debitID),
		MerchantID:        merchantID,
		DocumentID:        documentID,
		StableDocumentKey: &stableKey,
		SequenceNumber:    parent.SequenceNumber,
		EgsID:             &egsID,
	}); err != nil {
		log.Printf("DebitBill submission: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := h.pub.EnqueueDebit(
		ctx,
		qtx,
		merchantID,
		debitID,
		int64(*parent.BranchID),
	); err != nil {
		log.Printf("DebitBill enqueue ZATCA event: %s", sanitizeForLog(err.Error()))
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	h.wakeZatcaOutbox()
	c.JSON(http.StatusCreated, debitID)
}

func debitEGSID(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		v = strings.TrimSpace(v)
		return v, v != ""
	case []byte:
		s := strings.TrimSpace(string(v))
		return s, s != ""
	default:
		return "", false
	}
}
