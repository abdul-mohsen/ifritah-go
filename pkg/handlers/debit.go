package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/model"
)

// DebitBill adds an extra charge to a final bill. It is written like a credit note: the note is verified against its bill before a number
// is booked, and the note, its lines and its ZATCA event are written in one transaction. The answer is the note's id, and a repeat of the
// same request gets the original note.
func (h *handler) DebitBill(c *gin.Context) {
	var request model.DebitNoteRequest
	if err := c.BindJSON(&request); err != nil {
		log.Printf("DebitBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err := verifyDebitRequest(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	fingerprint, err := debitFingerprint(request)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	ident, err := noteIdentity(request.PosID, request.BillRequestID, fingerprint)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if !h.checkNoteStoreAccess(c, request.BillID) {
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()
	noteID, ok := h.writeDebitNote(c, tx, &request, ident)
	if !ok {
		return
	}
	c.JSON(http.StatusCreated, noteID)
}

// writeDebitNote verifies a debit note against its bill, and writes it in tx. It returns the note's id, and on failure it has written the
// error response.
func (h *handler) writeDebitNote(c *gin.Context, tx *sql.Tx, request *model.DebitNoteRequest, ident billIdentity) (uint64, bool) {
	ctx := c.Request.Context()
	qtx := h.queries.WithTx(tx)
	bill, ok := lockNoteBill(c, qtx, request.BillID, request.BranchID)
	if !ok {
		return 0, false
	}
	rec, found, err := debitRecordOf(c, qtx, ident)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	if found {
		return answerRecordedNote(c, rec, request.BillID, ident)
	}
	taken, err := billRequestTaken(c, qtx, ident)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	if taken {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": requestConflictCode, "message": noteConflictMessage})
		return 0, false
	}
	lines := debitLines(request.Lines)
	total, err := checkNoteTotal(request.Amount, lines)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return 0, false
	}

	state := noteStateFinal
	res, err := qtx.InsertDebitNote(ctx, db.InsertDebitNoteParams{
		BillID:                    request.BillID,
		State:                     &state,
		Note:                      &request.Reason,
		Amount:                    request.Amount,
		LinesTotal:                total,
		InvoiceUuid:               nullableString(ident.InvoiceUUID),
		PosID:                     nilIfEmpty(ident.PosID),
		BillRequestID:             nullableString(ident.BillRequestID),
		RequestFingerprint:        nullableString(ident.Fingerprint),
		RequestFingerprintVersion: identity.FingerprintVersion,
	})
	if err != nil {
		if IsDuplicate(err) {
			_ = tx.Rollback()
			rec, found, lookupErr := debitRecordOf(c, h.queries, ident)
			return answerLostNoteRace(c, rec, found, lookupErr, request.BillID, ident)
		}
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	insertedID, err := res.LastInsertId()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	noteID := uint64(insertedID)
	for _, line := range lines {
		err := qtx.InsertDebitNoteLine(ctx, db.InsertDebitNoteLineParams{
			DebitNoteID: noteID,
			Name:        *line.name,
			Quantity:    line.quantity,
			Price:       line.price,
			Vat:         line.vat,
		})
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return 0, false
		}
	}
	if err := h.queueNoteEvent(c, tx, identity.DocDebit, noteID, *bill.BranchID, ident); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	return noteID, true
}
