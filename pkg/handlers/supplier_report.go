package handlers

// ============================================================================
// Supplier Report Handler — GET /api/v2/supplier/:id/report
// ============================================================================
// IMPACT: 🟢 High — enables per-supplier purchase analytics & كشف حساب
// RISK:   Low (read-only aggregation, no schema changes beyond indexes)
//
// PROBLEM:
//   POST /api/v2/purchase_bill/all returns no supplier_id in the list response.
//   To build a supplier report, the frontend must fetch every single bill's
//   detail (N+1 calls), which is slow and wastes bandwidth.
//
// REAL SCHEMA NOTES:
//   purchase_bill (11 columns):
//     id, effective_date, payment_due_date, state, discount (BIGINT),
//     supplier_id, sequence_number, supplier_sequence_number, vat_sequence_number,
//     store_id, merchant_id
//     ⚠ NO: pdf_link, created_at, updated_at, total, total_before_vat, total_vat
//
//   purchase_bill_product:
//     id, product_id, bill_id, vat DECIMAL(5,2) DEFAULT 15.00,
//     price DECIMAL(12,2), quantity DECIMAL(10,3), name,
//     GENERATED: total_before_vat, vat_total, total_including_vat, type
//
//   supplier (11 columns):
//     id (BIGINT), company_id, name, address, phone_number, number,
//     vat_number, is_deleted, bank_account, created_at, updated_at
//     ⚠ NO: credit_limit, payment_terms_days, email, commercial_registration
//
//   cash_voucher: merchant_id, recipient_type='supplier', recipient_id, amount, ...
//
//   Auth: GetSessionInfo(c), c.GetInt64("userId")
//   Backend: handler struct {DB *sql.DB, queries *db.Queries}
//
// ENDPOINTS:
//   1. GET  /api/v2/supplier/:id/report?from=&to=
//   2. PUT  /api/v2/purchase_bill/:id/received
//   3. DELETE /api/v2/purchase_bill/:id/received
// ============================================================================

import (
	db "ifritah/web-service-gin/pkg/db/gen"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ── GET /api/v2/supplier/:id/report ─────────────────────────────────────────
// Query params:
//
//	from: start date (YYYY-MM-DD), optional — defaults to 12 months ago
//	to:   end date (YYYY-MM-DD), optional — defaults to today
//
// Response:
//
//	{
//	  "supplier":          { id, company_id, name, address, phone_number,
//	                         number, vat_number, is_deleted, bank_account },
//	  "summary": {
//	    "bill_count":         42,
//	    "total_spent":        125000.00,   // SUM(purchase_bill_product.total_including_vat)
//	    "total_before_vat":   108695.65,   // SUM(purchase_bill_product.total_before_vat)
//	    "total_vat":          16304.35,    // SUM(purchase_bill_product.vat_total)
//	    "unpaid_total":       35000.00,    // bills where state=0
//	    "paid_total":         90000.00,    // bills where state>=1
//	    "received_count":     38,
//	    "avg_bill":           2976.19,
//	    "total_discount":     500,         // BIGINT from purchase_bill.discount
//	    "total_payments":     80000.00,    // SUM(cash_voucher.amount)
//	    "payment_count":      15,
//	    "closing_balance":    45000.00     // total_spent - total_payments
//	  },
//	  "bills":             [ ... ],        // with computed totals from GENERATED columns
//	  "payments":          [ ... ],        // cash_voucher disbursements to supplier
//	  "top_items":         [ ... ],
//	  "aging":             [ { bucket, bill_count, bucket_total } ],
//	  "monthly_spending":  [ { month, bill_count, total_spent } ],
//	  "payment_breakdown": { cash_total, bank_transfer_total }
//	}
func (h *handler) GetSupplierReport(c *gin.Context) {
	supplierID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid supplier ID"})
		return
	}

	// Tenant isolation: merchant_id on purchase_bill / cash_voucher.
	// Adjust based on how your auth middleware exposes it.
	merchantID := c.GetInt("merchant_id")

	// Parse date range (default: last 12 months)
	now := time.Now()
	fromStr := c.Query("from")
	toStr := c.Query("to")

	var dateFrom, dateTo time.Time
	if fromStr != "" {
		dateFrom, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid 'from' date format, use YYYY-MM-DD"})
			return
		}
	} else {
		dateFrom = now.AddDate(-1, 0, 0)
	}
	if toStr != "" {
		dateTo, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid 'to' date format, use YYYY-MM-DD"})
			return
		}
	} else {
		dateTo = now
	}

	// 1. Fetch supplier info
	// Uses existing GetSupplier query (same as GET /api/v2/supplier/:id)
	supplier, err := h.queries.GetSupplier(c, int64(supplierID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "supplier not found"})
		return
	}

	// 2. Fetch bill summary (totals from purchase_bill_product GENERATED columns)
	summary, err := h.queries.GetSupplierBillSummary(c, db.GetSupplierBillSummaryParams{
		SupplierID: int32(supplierID),
		MerchantID: int32(merchantID),
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to compute bill summary"})
		return
	}

	// 3. Fetch payment summary from cash_voucher
	supplierID32 := int32(supplierID)
	paymentSummary, err := h.queries.GetSupplierPaymentSummary(c, db.GetSupplierPaymentSummaryParams{
		SupplierID: &supplierID32,
		MerchantID: int32(merchantID),
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		// Non-fatal — default to zero
		paymentSummary = db.GetSupplierPaymentSummaryRow{}
	}

	// Closing balance = total spent - total payments
	closingBalance := summary.TotalSpent.Sub(paymentSummary.TotalPayments)

	// 4. Fetch bills (with totals from purchase_bill_product GENERATED columns)
	bills, err := h.queries.GetPurchaseBillsBySupplier(c, db.GetPurchaseBillsBySupplierParams{
		SupplierID: int32(supplierID),
		MerchantID: int32(merchantID),
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch bills"})
		return
	}

	// 5. Fetch payments (cash_voucher disbursements to supplier)
	payments, err := h.queries.GetSupplierPayments(c, db.GetSupplierPaymentsParams{
		SupplierID: &supplierID32,
		MerchantID: int32(merchantID),
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		payments = nil // non-fatal
	}

	// 6. Fetch top items (from purchase_bill_product, NOT bill_product)
	topItems, err := h.queries.GetTopPurchasedItems(c, db.GetTopPurchasedItemsParams{
		SupplierID: int32(supplierID),
		MerchantID: int32(merchantID),
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		topItems = nil // non-fatal
	}

	// 7. Fetch aging buckets (unpaid bills by overdue period)
	aging, err := h.queries.GetSupplierAgingBuckets(c, db.GetSupplierAgingBucketsParams{
		SupplierID: int32(supplierID),
		MerchantID: int32(merchantID),
	})
	if err != nil {
		aging = nil // non-fatal
	}

	// 8. Fetch monthly spending trend
	monthlySpending, err := h.queries.GetSupplierMonthlySpending(c, db.GetSupplierMonthlySpendingParams{
		SupplierID: int32(supplierID),
		MerchantID: int32(merchantID),
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		monthlySpending = nil // non-fatal
	}

	c.JSON(http.StatusOK, gin.H{
		"supplier": supplier,
		"summary": gin.H{
			"bill_count":       summary.BillCount,
			"total_spent":      summary.TotalSpent,
			"total_before_vat": summary.TotalBeforeVat,
			"total_vat":        summary.TotalVat,
			"unpaid_total":     summary.UnpaidTotal,
			"paid_total":       summary.PaidTotal,
			"received_count":   summary.ReceivedCount,
			"avg_bill":         summary.AvgBill,
			"total_discount":   summary.TotalDiscount,
			"total_payments":   paymentSummary.TotalPayments,
			"payment_count":    paymentSummary.PaymentCount,
			"closing_balance":  closingBalance,
		},
		"bills":            bills,
		"payments":         payments,
		"top_items":        topItems,
		"aging":            aging,
		"monthly_spending": monthlySpending,
		"payment_breakdown": gin.H{
			"cash_total":          paymentSummary.CashTotal,
			"bank_transfer_total": paymentSummary.BankTransferTotal,
		},
	})
}

// ── PUT /api/v2/purchase_bill/:id/received ──────────────────────────────────
// Marks a purchase bill as received (goods confirmed delivered).
// Body: {} (empty — uses server timestamp and current user)

func (h *handler) MarkBillReceived(c *gin.Context) {
	billID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid bill ID"})
		return
	}
	merchantID := c.GetInt("merchant_id")
	userID := c.GetInt64("userId") // from GetSessionInfo / JWTVerifyMiddleware
	userID32 := int32(userID)

	err = h.queries.MarkPurchaseBillReceived(c, db.MarkPurchaseBillReceivedParams{
		ID:         uint64(billID),
		MerchantID: int32(merchantID),
		ReceivedBy: &userID32, // received_by is INT FK to user.id (see migration)
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to mark as received"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

// ── DELETE /api/v2/purchase_bill/:id/received ───────────────────────────────
// Clears receipt confirmation from a purchase bill.

func (h *handler) UnmarkBillReceived(c *gin.Context) {
	billID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid bill ID"})
		return
	}
	merchantID := c.GetInt("merchant_id")

	err = h.queries.UnmarkPurchaseBillReceived(c, db.UnmarkPurchaseBillReceivedParams{
		ID:         uint64(billID),
		MerchantID: int32(merchantID),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to clear receipt status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}
