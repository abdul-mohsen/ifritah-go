package handlers

import (
	"fmt"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/models"
	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/pdf"
	"github.com/gin-gonic/gin"
)

type BillCredit struct {
	BillId int    `json:"bill_id" binding:"required"`
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

	// Insert the credit note
	res, err := tx.Exec(
		"INSERT INTO credit_note (bill_id, state, note) VALUES (?, ?, ?)",
		request.BillId, 1, request.Note,
	)
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
		tx, int32(creditNoteID), int32(request.BillId), int32(userSession.id),
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

	// userSession := GetSessionInfo(c) // to allow users to use this feature

	var id string = c.Param("id")

	filename := filepath.Join("/var", "www", "html", "downloads", id+".pdf")
	// Check if the file exists
	// _, err := os.Stat(filename)
	if true {
		bill, xProducts := h.getBillDetail(c)
		var products []models.Product
		name := "تكلفة الصيانة"
		for _, p := range xProducts {
			if p.Name != model.MaintenanceCost {
				name = p.Name
			}
			product := models.Product{
				Name:      name,
				Quantity:  p.Quantity,
				UnitPrice: p.Price,
				Discount:  p.Discount,
				VATAmount: p.TotalVAT,
				Total:     p.Total,
			}
			products = append(products, product)

		}
		qr := ""

		if bill.QRCode != nil {
			qr = *bill.QRCode
		}

		invoice := models.Invoice{
			Title:                        "فاتورة دائن",
			InvoiceNumber:                fmt.Sprintf("INV%d", *bill.CreditID),
			StoreName:                    bill.StoreName,
			StoreAddress:                 bill.Address,
			Date:                         bill.EffectiveDate.Local().Format(time.DateTime),
			VATRegistrationNo:            bill.VatRegistration,
			QRCodeData:                   qr,
			TotalDiscount:                "0.0",
			TotalTaxableAmt:              bill.TotalBeforeVAT,
			TotalVAT:                     bill.TotalVAT,
			TotalWithVAT:                 bill.Total,
			VATPercentage:                "15",
			CommercialRegistrationNumber: bill.CommercialRegistrationNumber,
			CreditNote:                   bill.CreditNote,
			Labels: models.Labels{
				CommercialRegistrationNumber: "رقم السجل التجاري",
				InvoiceNumber:                "رقم الفاتورة:",
				Date:                         "تاريخ:",
				VATRegistration:              "رقم تسجيل ضريبة القيمة المضافة:",
				TotalTaxable:                 "اجمالي المبلغ الخاضع للضريبة",
				TotalWithVat:                 "المجموع مع الضريبة",
				ProductColumn:                "المنتجات",
				QuantityColumn:               "الكمية",
				UnitPriceColumn:              "سعر الوحدة",
				DiscountColumn:               "الخصم",
				VATAmountColumn:              "ضريبة القيمة المضافة",
				TotalColumn:                  "السعر شامل الضريبة",
				TotalDiscount:                "إجمالي الخصم",
				Footer:                       fmt.Sprintf(">>>>>>>>>>>>>>  إغلاق الفاتورة %d   <<<<<<<<<<<<<<< BILL REF %d |", *bill.CreditID, bill.SequenceNumber),
				Vat:                          "ضريبة القيمة المضافة (%51)",
				Reason:                       "ملاحظة:",
			},
			Language: "ar",
			Products: products,
			IsRTL:    true,
		}

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

	// Upload the file to specific dst.
	c.File(filename)

}
