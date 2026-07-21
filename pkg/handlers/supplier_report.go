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
	"context"
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"net/http"
	"strconv"
	"strings"
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
	supplierID, err := parseInt32Param(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid supplier ID"})
		return
	}

	merchantID := int32(getMerchantID(c))
	dateFrom, dateTo, err := parseSupplierReportDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	report, err := h.buildSupplierReport(c.Request.Context(), supplierID, merchantID, dateFrom, dateTo)
	if err != nil {
		c.JSON(supplierReportErrorStatus(err), gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// ── GET /api/v2/supplier/report/multi ───────────────────────────────────────
// Combined ledger statement for two or more suppliers in a single request.
// Query params:
//
//	ids:  comma-separated supplier IDs, e.g. "12,34,56" (required, max 50)
//	from: start date (YYYY-MM-DD), optional — defaults to 12 months ago
//	to:   end date (YYYY-MM-DD), optional — defaults to today
//
// Introduced so the frontend's multi-supplier ledger statement page doesn't
// have to issue one HTTP round trip per selected supplier - it sends one
// request here and the backend loops server-side (same per-supplier query
// set as GetSupplierReport, just without the network round trip per
// supplier). This does not batch the underlying SQL into a single IN(...)
// query per metric - that would require reworking every query in this file
// to group-by supplier_id and is a larger, separate change; this instead
// eliminates the N-network-round-trips scalability problem the frontend
// would otherwise have, which is the one actually visible to callers.
//
// Response: { "suppliers": [ <same shape as GetSupplierReport's body>, ... ] }
// Suppliers that fail to resolve (not found, wrong tenant) are omitted from
// the response rather than failing the whole request.
func (h *handler) GetMultiSupplierReport(c *gin.Context) {
	idsParam := c.Query("ids")
	if idsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "ids query parameter is required"})
		return
	}

	supplierIDs, err := parseSupplierIDList(idsParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	if len(supplierIDs) > maxMultiSupplierReportIDs {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "too many supplier ids (max 50)"})
		return
	}

	merchantID := int32(getMerchantID(c))
	dateFrom, dateTo, err := parseSupplierReportDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	reports := make([]gin.H, 0, len(supplierIDs))
	for _, supplierID := range supplierIDs {
		report, err := h.buildSupplierReport(c.Request.Context(), supplierID, merchantID, dateFrom, dateTo)
		if err != nil {
			// Skip suppliers that don't resolve (not found / wrong tenant)
			// rather than failing the whole combined statement.
			continue
		}
		reports = append(reports, report)
	}

	c.JSON(http.StatusOK, gin.H{"suppliers": reports})
}

// maxMultiSupplierReportIDs caps how many suppliers can be combined into one
// GetMultiSupplierReport call, so a single request can't be used to force
// the backend to build an unbounded number of per-supplier reports.
const maxMultiSupplierReportIDs = 50

// parseSupplierIDList parses a comma-separated list of supplier IDs,
// de-duplicating and rejecting anything that isn't a positive integer.
func parseSupplierIDList(raw string) ([]int32, error) {
	parts := strings.Split(raw, ",")
	ids := make([]int32, 0, len(parts))
	seen := make(map[int32]bool, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 32)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid supplier id %q in ids list", p)
		}
		id := int32(n)
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids list must contain at least one supplier id")
	}
	return ids, nil
}

// parseSupplierReportDateRange parses the shared from/to query params used by
// both the single- and multi-supplier report endpoints (default: last 12
// months).
func parseSupplierReportDateRange(c *gin.Context) (time.Time, time.Time, error) {
	now := time.Now()
	fromStr := c.Query("from")
	toStr := c.Query("to")

	dateFrom := now.AddDate(-1, 0, 0)
	dateTo := now

	if fromStr != "" {
		parsed, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid 'from' date format, use YYYY-MM-DD")
		}
		dateFrom = parsed
	}
	if toStr != "" {
		parsed, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid 'to' date format, use YYYY-MM-DD")
		}
		dateTo = parsed
	}
	return dateFrom, dateTo, nil
}

// supplierNotFoundError distinguishes a missing/wrong-tenant supplier (404)
// from a real query failure (500) so buildSupplierReport's single error
// return can still map to the right HTTP status for the single-supplier
// endpoint, while the multi-supplier endpoint just skips either case.
type supplierNotFoundError struct{}

func (supplierNotFoundError) Error() string { return "supplier not found" }

func supplierReportErrorStatus(err error) int {
	if _, ok := err.(supplierNotFoundError); ok {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

// buildSupplierReport runs the full set of per-supplier report queries
// (summary, bills, payments, top items, aging, monthly spending) and returns
// the same response shape GetSupplierReport has always returned. Shared by
// both GetSupplierReport (one supplier) and GetMultiSupplierReport (a
// combined statement over several suppliers), so the two endpoints can never
// drift into returning different data for the same supplier.
func (h *handler) buildSupplierReport(ctx context.Context, supplierID, merchantID int32, dateFrom, dateTo time.Time) (gin.H, error) {
	// 1. Fetch supplier info
	// Uses existing GetSupplier query (same as GET /api/v2/supplier/:id)
	supplier, err := h.queries.GetSupplier(ctx, int64(supplierID))
	if err != nil {
		return nil, supplierNotFoundError{}
	}

	// 2. Fetch bill summary (totals from purchase_bill_product GENERATED columns)
	summary, err := h.queries.GetSupplierBillSummary(ctx, db.GetSupplierBillSummaryParams{
		SupplierID: supplierID,
		MerchantID: merchantID,
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to compute bill summary: %w", err)
	}

	// 3. Fetch payment summary from cash_voucher
	supplierID32 := supplierID
	paymentSummary, err := h.queries.GetSupplierPaymentSummary(ctx, db.GetSupplierPaymentSummaryParams{
		SupplierID: &supplierID32,
		MerchantID: merchantID,
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
	bills, err := h.queries.GetPurchaseBillsBySupplier(ctx, db.GetPurchaseBillsBySupplierParams{
		SupplierID: supplierID,
		MerchantID: merchantID,
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bills: %w", err)
	}

	// 5. Fetch payments (cash_voucher disbursements to supplier)
	payments, err := h.queries.GetSupplierPayments(ctx, db.GetSupplierPaymentsParams{
		SupplierID: &supplierID32,
		MerchantID: merchantID,
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		payments = nil // non-fatal
	}

	// 6. Fetch top items (from purchase_bill_product, NOT bill_product)
	topItems, err := h.queries.GetTopPurchasedItems(ctx, db.GetTopPurchasedItemsParams{
		SupplierID: supplierID,
		MerchantID: merchantID,
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		topItems = nil // non-fatal
	}

	// 7. Fetch aging buckets (unpaid bills by overdue period)
	aging, err := h.queries.GetSupplierAgingBuckets(ctx, db.GetSupplierAgingBucketsParams{
		SupplierID: supplierID,
		MerchantID: merchantID,
	})
	if err != nil {
		aging = nil // non-fatal
	}

	// 8. Fetch monthly spending trend
	monthlySpending, err := h.queries.GetSupplierMonthlySpending(ctx, db.GetSupplierMonthlySpendingParams{
		SupplierID: supplierID,
		MerchantID: merchantID,
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
	})
	if err != nil {
		monthlySpending = nil // non-fatal
	}

	return gin.H{
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
	}, nil
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
	session := GetSessionInfo(c)
	merchantID := int32(session.id)
	userID32 := int32(session.id)

	err = h.queries.MarkPurchaseBillReceived(c.Request.Context(), db.MarkPurchaseBillReceivedParams{
		ID:         uint64(billID),
		MerchantID: merchantID,
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
	merchantID := int32(getMerchantID(c))

	err = h.queries.UnmarkPurchaseBillReceived(c.Request.Context(), db.UnmarkPurchaseBillReceivedParams{
		ID:         uint64(billID),
		MerchantID: merchantID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to clear receipt status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}
