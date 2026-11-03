package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/invoice"
	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/models"
	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/pdf"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/model"
)

// CreditBill credits part of a final bill. The note is verified against its bill before a number is booked. The note, its lines, the
// stock that it puts back and its ZATCA event are written in one transaction, and the answer is the note's id. A repeat of the same
// request gets the original note and writes nothing.
func (h *handler) CreditBill(c *gin.Context) {
	var request model.CreditNoteRequest
	if err := c.BindJSON(&request); err != nil {
		log.Printf("CreditBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err := verifyCreditRequest(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	fingerprint, err := creditFingerprint(request)
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
	userSession := GetSessionInfo(c)

	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()
	noteID, ok := h.writeCreditNote(c, tx, &request, ident, int32(userSession.id))
	if !ok {
		return
	}
	c.JSON(http.StatusCreated, noteID)
}

// writeCreditNote verifies a credit note against its bill, and writes it in tx. It returns the note's id, and on failure it has written the
// error response. The bill is locked first, so the reads that follow see every note that was recorded before this one.
func (h *handler) writeCreditNote(c *gin.Context, tx *sql.Tx, request *model.CreditNoteRequest, ident billIdentity, userID int32) (uint64, bool) {
	ctx := c.Request.Context()
	qtx := h.queries.WithTx(tx)
	bill, ok := lockNoteBill(c, qtx, request.BillID, request.BranchID)
	if !ok {
		return 0, false
	}
	rec, found, err := creditRecordOf(c, qtx, ident)
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
	lines, total, ok := verifyCreditAgainstBill(c, qtx, request, bill)
	if !ok {
		return 0, false
	}

	state := noteStateFinal
	res, err := qtx.InsertCreditNote(ctx, db.InsertCreditNoteParams{
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
			// A concurrent request recorded this request id after the check. The transaction is rolled back, and the record is
			// read again outside it.
			_ = tx.Rollback()
			rec, found, lookupErr := creditRecordOf(c, h.queries, ident)
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
		err := qtx.InsertCreditNoteLine(ctx, db.InsertCreditNoteLineParams{
			CreditNoteID:  noteID,
			BillProductID: line.billProductID,
			Name:          line.name,
			Quantity:      line.quantity,
			Price:         line.price,
			Discount:      line.discount,
			Vat:           line.vat,
		})
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return 0, false
		}
	}
	if err := h.restoreCreditStock(c, qtx, lines, bill.StoreID, noteID, userID); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	if err := h.queueNoteEvent(c, tx, identity.DocCredit, noteID, *bill.BranchID, ident); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	return noteID, true
}

// verifyCreditAgainstBill checks a credit note against the lines of its bill, the quantities that earlier notes credited, and the
// amounts they recorded. It returns the lines that the note writes and their total. It writes the error response and returns false when
// the note cannot be issued.
func verifyCreditAgainstBill(c *gin.Context, qtx *db.Queries, request *model.CreditNoteRequest, bill db.LockBillForNoteRow) ([]noteLine, decimal.Decimal, bool) {
	ctx := c.Request.Context()
	rows, err := qtx.ListBillLinesForNote(ctx, request.BillID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return nil, decimal.Zero, false
	}
	credits, err := qtx.ListCreditedLinesForBill(ctx, request.BillID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return nil, decimal.Zero, false
	}
	amounts, err := qtx.ListCreditAmountsForBill(ctx, request.BillID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return nil, decimal.Zero, false
	}

	billLines := make(map[uint64]db.ListBillLinesForNoteRow, len(rows))
	for _, row := range rows {
		billLines[uint64(row.ID)] = row
	}
	credited := make(map[uint64]decimal.Decimal, len(credits))
	for _, row := range credits {
		id := uint64(row.BillProductID)
		credited[id] = credited[id].Add(row.Quantity)
	}
	lines, err := creditLines(request.Lines, billLines, credited)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return nil, decimal.Zero, false
	}
	total, err := checkNoteTotal(request.Amount, lines)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return nil, decimal.Zero, false
	}
	remaining := creditRemaining(bill.Total, amounts)
	if total.GreaterThan(remaining) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "credit is more than the bill's remaining amount, " + remaining.StringFixed(2)})
		return nil, decimal.Zero, false
	}
	return lines, total, true
}

func (h *handler) GetCreditBillPDF(c *gin.Context) {

	// Validate id is a positive integer before composing a path with it.
	// Path-traversal hardening (Sonar gosecurity:S2083): the param feeds
	// filepath.Join below, so any value containing path separators or
	// `..` segments could escape the downloads directory.
	rawID := c.Param("id")
	creditID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || creditID == 0 {
		c.Status(http.StatusBadRequest)
		return
	}
	id := strconv.FormatUint(creditID, 10)

	filename := filepath.Join("/var", "www", "html", "downloads", id+".pdf")
	if true {
		bill, products := h.getBillDetail(c)
		for _, p := range products {
			if p.Name != model.MaintenanceCost {
				p.Name = "تكلفة الصيانة"
			}
		}

		var invoice invoice.Invoice
		if bill.Client == nil {
			invoice = b2cInvoice(true, models.PaperThermal, bill, products).
				WithType(models.InvoiceTypeB2CCredit).
				WithNoteReason(*bill.CreditNote).
				WithTitle("إشعار دائن").
				Build()
		} else {
			invoice = b2bInvoice(true, models.PaperA4, bill, products, *bill.Client).
				WithType(models.InvoiceTypeB2BCredit).
				WithNoteReason(*bill.CreditNote).
				WithTitle("إشعار دائن").
				Build()
		}

		fontDir := "fonts"
		pdfBytes, err := pdf.GenerateInvoiceBytes(invoice, fontDir)
		if err != nil {
			return
		}

		if err := os.WriteFile(filename, pdfBytes, 0644); err != nil {
			c.Header("X-Cache-Warning", err.Error())
		}

		c.Header("X-Cache", "MISS")
	}
	c.Header("X-Cache", "HIT")

	c.File(filename)

}
