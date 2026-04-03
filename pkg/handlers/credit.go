package handlers

// ============================================================================
// COMPLETE FILE: pkg/handlers/credit.go (with stock tracking integrated)
// ============================================================================
// Copy this file to replace: pkg/handlers/credit.go
//
// Stock changes vs original:
//   1. CreditBill — wrapped in transaction, calls recordCreditNoteMovements()
//
// GetCreditBillPDF is unchanged from the original.
// ============================================================================

import (
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/models"
	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/pdf"
	"github.com/gin-gonic/gin"
)

type BillCredit struct {
	BillId int32  `json:"bill_id" binding:"required"`
	Note   string `json:"note" binding:"required"`
}

func (h *handler) CreditBill(c *gin.Context) {

	var request BillCredit

	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
		c.Status(http.StatusBadRequest)
		return
	}

	userSession := GetSessionInfo(c)

	// Use a transaction so credit note creation + stock restoration are atomic
	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	// Insert the credit note
	var one int32 = 1
	args := db.InsertCreditNoteParams{
		BillID: &request.BillId,
		State:  &one,
		Note:   &request.Note,
	}
	res, err := qtx.InsertCreditNote(c.Request.Context(), args)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	creditNoteID, err := res.LastInsertId()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	// ── Stock tracking: restore stock for all catalog items on the original bill ──
	if err := h.recordCreditNoteMovements(
		qtx, c, int32(creditNoteID), request.BillId, int32(userSession.id),
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"detail": err.Error(),
			"type":   "stock_error",
		})
		return
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	c.Status(http.StatusCreated)
}

func (h *handler) GetCreditBillPDF(c *gin.Context) {

	var id string = c.Param("id")

	filename := filepath.Join("/var", "www", "html", "downloads", id+".pdf")
	if true {
		bill, products := h.getBillDetail(c)
		for _, p := range products {
			if p.Name != model.MaintenanceCost {
				p.Name = "تكلفة الصيانة"
			}
		}

		invoice := b2cInvoice(true, models.PaperThermal, bill, products).
			WithType(models.InvoiceTypeB2CCredit).
			WithNoteReason(*bill.CreditNote).
			WithTitle("إشعار دائن").
			Build()

		fontDir := "fonts"
		pdfBytes, err := pdf.GenerateInvoiceBytes(invoice, fontDir)
		if err != nil {
			log.Panic(err)
		}

		if err := os.WriteFile(filename, pdfBytes, 0644); err != nil {
			c.Header("X-Cache-Warning", err.Error())
		}

		c.Header("X-Cache", "MISS")
	}
	c.Header("X-Cache", "HIT")

	c.File(filename)

}
