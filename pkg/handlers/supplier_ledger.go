package handlers

// ============================================================================
// Supplier General Ledger Handler — GET /api/v2/supplier/ledger
// ============================================================================
// Powers the frontend's combined supplier general ledger page
// (/dashboard/supplier-ledger): a chronological, running-balance view of
// every purchase bill (debit — increases what the merchant owes) and every
// approved/posted payment (credit — reduces what the merchant owes) for
// either one supplier or all of them.
//
// Query params:
//
//	supplier_id: a numeric supplier id, or "all" (default if omitted)
//	from:        start date (YYYY-MM-DD), optional — defaults to 90 days ago
//	to:          end date (YYYY-MM-DD), optional — defaults to today
//
// Response:
//
//	{
//	  "summary":  { bill_count, payment_count, total_debit, total_credit, closing_balance },
//	  "ledger":   [ { date, type, system_id, supplier_no, reference, description,
//	                 debit, credit, balance, link_url }, ... ],  // combined, chronological
//	  "suppliers": [ { supplier_id, supplier_name, debit, credit, balance }, ... ] // only when supplier_id=all
//	}
// ============================================================================

import (
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type supplierLedgerEntry struct {
	Date        string  `json:"date"`
	Type        string  `json:"type"`
	SystemID    uint64  `json:"system_id"`
	SupplierID  int32   `json:"supplier_id"`
	SupplierNo  string  `json:"supplier_no"`
	Reference   string  `json:"reference"`
	Description string  `json:"description"`
	Debit       float64 `json:"debit"`
	Credit      float64 `json:"credit"`
	Balance     float64 `json:"balance"`
	LinkURL     string  `json:"link_url"`
}

type supplierLedgerBreakdown struct {
	SupplierID   int32   `json:"supplier_id"`
	SupplierName string  `json:"supplier_name"`
	Debit        float64 `json:"debit"`
	Credit       float64 `json:"credit"`
	Balance      float64 `json:"balance"`
}

// GetSupplierLedger handles GET /api/v2/supplier/ledger.
func (h *handler) GetSupplierLedger(c *gin.Context) {
	merchantID := int32(getMerchantID(c))

	dateFrom, dateTo, ok := parseSupplierLedgerDateRange(c)
	if !ok {
		return
	}

	var supplierIDFilter *int32
	if raw := c.Query("supplier_id"); raw != "" && raw != "all" {
		id, err := parseInt32Param(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid supplier_id"})
			return
		}
		supplierIDFilter = &id
	}

	bills, err := h.queries.GetLedgerBillsForMerchant(c.Request.Context(), db.GetLedgerBillsForMerchantParams{
		MerchantID: merchantID, SupplierID: supplierIDFilter, DateFrom: &dateFrom, DateTo: &dateTo,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch ledger bills"})
		return
	}
	payments, err := h.queries.GetLedgerPaymentsForMerchant(c.Request.Context(), db.GetLedgerPaymentsForMerchantParams{
		MerchantID: merchantID, SupplierID: supplierIDFilter, DateFrom: &dateFrom, DateTo: &dateTo,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch ledger payments"})
		return
	}

	perSupplier := buildSupplierLedgerEntries(bills, payments)

	// Combined, running-balance ledger across every matched supplier
	// (meaningful for a single supplier; for "all" it's still a useful
	// chronological feed, ordered the same way each supplier's own feed is).
	combined := make([]supplierLedgerEntry, 0, len(bills)+len(payments))
	for _, entries := range perSupplier {
		combined = append(combined, entries...)
	}
	sort.SliceStable(combined, func(i, j int) bool { return combined[i].Date < combined[j].Date })
	combined = withRunningBalance(combined)

	summary := gin.H{
		"bill_count":    len(bills),
		"payment_count": len(payments),
	}
	var totalDebit, totalCredit decimal.Decimal
	for _, e := range combined {
		totalDebit = totalDebit.Add(decimal.NewFromFloat(e.Debit))
		totalCredit = totalCredit.Add(decimal.NewFromFloat(e.Credit))
	}
	summary["total_debit"] = totalDebit
	summary["total_credit"] = totalCredit
	summary["closing_balance"] = totalDebit.Sub(totalCredit)

	response := gin.H{"summary": summary, "ledger": combined}
	if supplierIDFilter == nil {
		response["suppliers"] = supplierLedgerBreakdowns(perSupplier)
	}
	c.JSON(http.StatusOK, response)
}

// parseSupplierLedgerDateRange reads from/to query params (default: last 90
// days), writing a 400 response and returning ok=false on a bad format.
func parseSupplierLedgerDateRange(c *gin.Context) (time.Time, time.Time, bool) {
	now := time.Now()
	dateFrom, dateTo := now.AddDate(0, 0, -90), now
	if raw := c.Query("from"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid 'from' date format, use YYYY-MM-DD"})
			return time.Time{}, time.Time{}, false
		}
		dateFrom = parsed
	}
	if raw := c.Query("to"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid 'to' date format, use YYYY-MM-DD"})
			return time.Time{}, time.Time{}, false
		}
		dateTo = parsed
	}
	return dateFrom, dateTo, true
}

// buildSupplierLedgerEntries groups bills+payments by supplier and computes
// each supplier's own chronological, running-balance ledger.
func buildSupplierLedgerEntries(bills []db.GetLedgerBillsForMerchantRow, payments []db.GetLedgerPaymentsForMerchantRow) map[int32][]supplierLedgerEntry {
	bySupplier := make(map[int32][]supplierLedgerEntry)
	names := make(map[int32]string)

	for _, bill := range bills {
		name := ""
		if bill.SupplierName != nil {
			name = *bill.SupplierName
		}
		names[bill.SupplierID] = name
		supplierNo := ""
		if bill.SupplierSequenceNumber != nil {
			supplierNo = fmt.Sprintf("%d", *bill.SupplierSequenceNumber)
		}
		total, _ := bill.Total.Float64()
		bySupplier[bill.SupplierID] = append(bySupplier[bill.SupplierID], supplierLedgerEntry{
			Date:        bill.EffectiveDate.Format("2006-01-02"),
			Type:        "bill",
			SystemID:    bill.ID,
			SupplierID:  bill.SupplierID,
			SupplierNo:  supplierNo,
			Reference:   fmt.Sprintf("PB-%d", bill.ID),
			Description: name,
			Debit:       total,
			LinkURL:     fmt.Sprintf("/dashboard/purchase-bills/%d", bill.ID),
		})
	}

	for _, payment := range payments {
		if payment.SupplierID == nil {
			continue
		}
		supplierID := *payment.SupplierID
		if _, known := names[supplierID]; !known {
			names[supplierID] = payment.SupplierName
		}
		amount, _ := payment.Amount.Float64()
		bySupplier[supplierID] = append(bySupplier[supplierID], supplierLedgerEntry{
			Date:        payment.EffectiveDate.Format("2006-01-02"),
			Type:        "payment",
			SystemID:    payment.ID,
			SupplierID:  supplierID,
			Reference:   fmt.Sprintf("CV-%d", payment.VoucherNumber),
			Description: payment.SupplierName,
			Credit:      amount,
			LinkURL:     fmt.Sprintf("/dashboard/cash-vouchers/%d", payment.ID),
		})
	}

	for supplierID, entries := range bySupplier {
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Date < entries[j].Date })
		bySupplier[supplierID] = withRunningBalance(entries)
	}
	return bySupplier
}

// withRunningBalance stamps each entry's Balance as the cumulative
// debit-minus-credit up to and including that entry, in the given order.
func withRunningBalance(entries []supplierLedgerEntry) []supplierLedgerEntry {
	balance := decimal.Zero
	for i := range entries {
		balance = balance.Add(decimal.NewFromFloat(entries[i].Debit)).Sub(decimal.NewFromFloat(entries[i].Credit))
		entries[i].Balance, _ = balance.Float64()
	}
	return entries
}

func supplierLedgerBreakdowns(bySupplier map[int32][]supplierLedgerEntry) []supplierLedgerBreakdown {
	breakdowns := make([]supplierLedgerBreakdown, 0, len(bySupplier))
	for supplierID, entries := range bySupplier {
		var debit, credit decimal.Decimal
		name := ""
		for _, e := range entries {
			debit = debit.Add(decimal.NewFromFloat(e.Debit))
			credit = credit.Add(decimal.NewFromFloat(e.Credit))
			if e.Description != "" && e.Type != "payment" {
				name = e.Description
			}
		}
		if name == "" {
			for _, e := range entries {
				if e.Type == "payment" && e.Description != "" {
					name = e.Description
					break
				}
			}
		}
		debitF, _ := debit.Float64()
		creditF, _ := credit.Float64()
		balanceF, _ := debit.Sub(credit).Float64()
		breakdowns = append(breakdowns, supplierLedgerBreakdown{
			SupplierID: supplierID, SupplierName: name,
			Debit: debitF, Credit: creditF, Balance: balanceF,
		})
	}
	sort.Slice(breakdowns, func(i, j int) bool { return breakdowns[i].SupplierName < breakdowns[j].SupplierName })
	return breakdowns
}
