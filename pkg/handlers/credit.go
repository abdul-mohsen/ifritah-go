package handlers

// ============================================================================
// COMPLETE FILE: pkg/handlers/credit.go (with stock tracking integrated)
// ============================================================================
// Copy this file to replace: pkg/handlers/credit.go
//
// Stock changes vs original:
//   1. CreditBill — disabled until ZATCA submission of credit notes moves to the outbox (B6)
//
// GetCreditBillPDF is unchanged from the original.
// ============================================================================

import (
	"ifritah/web-service-gin/pkg/model"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/invoice"
	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/models"
	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/pdf"
	"github.com/gin-gonic/gin"
)

type BillCredit struct {
	BillId uint64 `json:"bill_id" binding:"required"`
	Note   string `json:"note" binding:"required"`
}

// CreditBill is disabled until ZATCA submission of credit notes uses the outbox contract (B6). The old flow
// sent credit notes with the legacy envelope, which the clean daemon rejects, so the request is refused
// before anything is written.
func (h *handler) CreditBill(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
		"message": "credit notes are unavailable until their ZATCA submission moves to the outbox (B6)",
	})
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
