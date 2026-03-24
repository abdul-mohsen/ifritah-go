package handlers

// ============================================================================
// Dashboard API — Complete Backend for Afrita Dashboard
// ============================================================================
// Copy to: pkg/handlers/handler_dashboard.go
//
// Endpoints:
//   GET  /api/v2/dashboard                → GetDashboard (main KPIs + charts)
//   GET  /api/v2/dashboard/analytics      → GetDashboardAnalytics (aging, P&L, cash flow)
//   GET  /api/v2/dashboard/compare        → GetDashboardCompare (period comparison)
//
// Query Parameters (all endpoints):
//   ?state=0|1|2|3         → Filter invoices by state (0=draft, 1=processing, 2=processed, 3=issued)
//   ?start_date=2026-01-01 → Filter from date (inclusive)
//   ?end_date=2026-03-31   → Filter to date (inclusive)
//   ?months=6              → Number of months for charts (default 6)
//
// Compare endpoint additional params:
//   ?a_start=2026-01-01&a_end=2026-01-31  → Period A
//   ?b_start=2025-01-01&b_end=2025-01-31  → Period B
//
// DATABASE TABLES/VIEWS USED:
//   - bill               (invoices — rows NOT in credit_note are regular invoices)
//   - bill_totals         (VIEW: bill + computed total/total_vat from bill_product)
//   - credit_note         (tracks which bills are credit notes via bill_id FK → bill.id)
//   - bill_product        (line items for bills, has name/part_name columns)
//   - purchase_bill       (purchase bills — has merchant_id, supplier_id)
//   - purchase_bill_totals (VIEW: purchase_bill + computed total/total_vat)
//   - product             (inventory: id, article_id, store_id, price, quantity, cost_price, min_stock)
//   - articles            (TecDoc catalog: legacyArticleId → articleNumber)
//   - client              (customers — NO company FK, shared across tenants)
//   - supplier            (vendors — has company_id)
//   - store               (warehouses — has company_id)
//   - branches            (branches — has company_id)
//
// SCHEMA NOTES:
//   - bill has merchant_id but NO bill_type column and NO total column
//   - Credit notes: a bill is a credit note if credit_note.bill_id = bill.id
//   - bill_totals VIEW provides: total_before_vat, total_vat, total (computed from bill_product)
//   - purchase_bill_totals VIEW provides same for purchase bills
//   - product has NO merchant_id — belongs to store via store_id, store has company_id
//   - product has NO name column — use articles.articleNumber via product.article_id = articles.legacyArticleId
//   - client has NO company_id — clients are not tenant-scoped
//   - supplier uses company_id (not merchant_id)
//   - store uses company_id (not merchant_id)
//   - No `order` table exists — client activity derived from bill.buyer_id
//
// Follows same patterns as handler_stock.go:
//   - (h *handler) method receiver
//   - h.DB for database access
//   - GetSessionInfo(c) for current user (session.companyID = company.id)
// ============================================================================

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ── GET /api/v2/dashboard ───────────────────────────────────────────────────

func (h *handler) GetDashboard(c *gin.Context) {
	companyID := h.getUserCompany(c)

	// Parse query parameters
	stateFilter := c.Query("state")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	monthsParam := c.DefaultQuery("months", "6")
	numMonths, _ := strconv.Atoi(monthsParam)
	if numMonths < 1 || numMonths > 24 {
		numMonths = 6
	}

	now := time.Now()

	// ── Build month labels ─────────────────────────────────────────
	monthLabels := make([]string, numMonths)
	for i := numMonths - 1; i >= 0; i-- {
		m := now.AddDate(0, -i, 0)
		monthLabels[numMonths-1-i] = m.Format("01/2006")
	}

	// ── Helper: product has no merchant_id, it links to store ──────
	// store.company_id = session.companyID
	storeSubquery := "SELECT id FROM store WHERE company_id = ?"

	// ── 1. COUNTS ──────────────────────────────────────────────────
	var totalProducts, totalClients, totalSuppliers, totalStores, totalBranches int

	h.DB.QueryRow(`
		SELECT COUNT(*) FROM product
		WHERE store_id IN (`+storeSubquery+`) AND is_deleted = 0
	`, companyID).Scan(&totalProducts)

	// client has no company_id — count all non-deleted
	h.DB.QueryRow("SELECT COUNT(*) FROM client WHERE is_deleted = 0").Scan(&totalClients)

	h.DB.QueryRow("SELECT COUNT(*) FROM supplier WHERE company_id = ? AND is_deleted = 0",
		companyID).Scan(&totalSuppliers)
	h.DB.QueryRow("SELECT COUNT(*) FROM store WHERE company_id = ?",
		companyID).Scan(&totalStores)
	h.DB.QueryRow("SELECT COUNT(*) FROM branches WHERE company_id = ?",
		companyID).Scan(&totalBranches)

	// ── 2. INVOICE AGGREGATES ──────────────────────────────────────
	// bill_totals VIEW gives us the total column; LEFT JOIN credit_note to exclude credit notes
	invoiceWhere := "WHERE bt.merchant_id = ?"
	invoiceArgs := []interface{}{companyID}

	if stateFilter != "" {
		if stateVal, err := strconv.Atoi(stateFilter); err == nil {
			invoiceWhere += " AND bt.state = ?"
			invoiceArgs = append(invoiceArgs, stateVal)
		}
	}
	if startDate != "" {
		invoiceWhere += " AND bt.effective_date >= ?"
		invoiceArgs = append(invoiceArgs, startDate)
	}
	if endDate != "" {
		invoiceWhere += " AND bt.effective_date <= ?"
		invoiceArgs = append(invoiceArgs, endDate+"T23:59:59Z")
	}

	// 2a. Status counts (invoices only, not credit notes)
	statusCounts := map[string]int{"draft": 0, "processing": 0, "processed": 0, "issued": 0}
	statusRows, err := h.DB.Query(`
		SELECT bt.state, COUNT(*) as cnt
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		`+invoiceWhere+` AND cn.id IS NULL
		GROUP BY bt.state
	`, invoiceArgs...)
	if err == nil {
		defer statusRows.Close()
		for statusRows.Next() {
			var state, cnt int
			if statusRows.Scan(&state, &cnt) == nil {
				switch state {
				case 0:
					statusCounts["draft"] = cnt
				case 1:
					statusCounts["processing"] = cnt
				case 2:
					statusCounts["processed"] = cnt
				case 3:
					statusCounts["issued"] = cnt
				}
			}
		}
	}

	totalInvoices := statusCounts["draft"] + statusCounts["processing"] +
		statusCounts["processed"] + statusCounts["issued"]

	// 2b. Revenue totals (invoices only)
	var totalRevenue, totalVAT, totalDiscount float64
	h.DB.QueryRow(`
		SELECT COALESCE(SUM(bt.total), 0),
		       COALESCE(SUM(bt.total_vat), 0),
		       COALESCE(SUM(bt.discount), 0)
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		`+invoiceWhere+` AND cn.id IS NULL
	`, invoiceArgs...).Scan(&totalRevenue, &totalVAT, &totalDiscount)

	// 2c. Pending amount (draft + processing, invoices only)
	var pendingAmount float64
	var pendingCount int
	pendingArgs := append([]interface{}{}, invoiceArgs...)
	h.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(bt.total), 0)
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		`+invoiceWhere+` AND bt.state IN (0, 1) AND cn.id IS NULL
	`, pendingArgs...).Scan(&pendingCount, &pendingAmount)

	// 2d. Credit notes count & total
	var creditNoteCount int
	var creditNoteTotal float64
	cnArgs := []interface{}{companyID}
	cnWhere := "WHERE bt.merchant_id = ?"
	if startDate != "" {
		cnWhere += " AND bt.effective_date >= ?"
		cnArgs = append(cnArgs, startDate)
	}
	if endDate != "" {
		cnWhere += " AND bt.effective_date <= ?"
		cnArgs = append(cnArgs, endDate+"T23:59:59Z")
	}
	h.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(bt.total), 0)
		FROM bill_totals bt
		INNER JOIN credit_note cn ON cn.bill_id = bt.id
		`+cnWhere,
		cnArgs...).Scan(&creditNoteCount, &creditNoteTotal)

	// ── 3. PURCHASE BILL AGGREGATES ────────────────────────────────
	pbWhere := "WHERE pbt.merchant_id = ?"
	pbArgs := []interface{}{companyID}
	if startDate != "" {
		pbWhere += " AND pbt.effective_date >= ?"
		pbArgs = append(pbArgs, startDate)
	}
	if endDate != "" {
		pbWhere += " AND pbt.effective_date <= ?"
		pbArgs = append(pbArgs, endDate+"T23:59:59Z")
	}

	var totalPurchases float64
	var totalPurchaseBills int
	h.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(pbt.total), 0)
		FROM purchase_bill_totals pbt `+pbWhere,
		pbArgs...).Scan(&totalPurchaseBills, &totalPurchases)

	grossProfit := totalRevenue - totalPurchases
	grossMargin := 0.0
	if totalRevenue > 0 {
		grossMargin = grossProfit * 100 / totalRevenue
	}

	// ── 4. MONTHLY REVENUE & PURCHASES (chart data) ────────────────
	type monthlyData struct {
		Revenue   float64
		Purchases float64
	}
	monthlyMap := make(map[string]*monthlyData)
	for _, label := range monthLabels {
		monthlyMap[label] = &monthlyData{}
	}

	// Revenue by month (invoices only, via bill_totals)
	revenueRows, err := h.DB.Query(`
		SELECT DATE_FORMAT(bt.effective_date, '%m/%Y') AS month_key,
		       COALESCE(SUM(bt.total), 0) AS revenue
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		WHERE bt.merchant_id = ? AND cn.id IS NULL
		  AND bt.effective_date >= DATE_SUB(NOW(), INTERVAL ? MONTH)
		GROUP BY month_key
		ORDER BY month_key
	`, companyID, numMonths)
	if err == nil {
		defer revenueRows.Close()
		for revenueRows.Next() {
			var key string
			var rev float64
			if revenueRows.Scan(&key, &rev) == nil {
				if md, ok := monthlyMap[key]; ok {
					md.Revenue = rev
				}
			}
		}
	}

	// Purchases by month (via purchase_bill_totals)
	purchaseRows, err := h.DB.Query(`
		SELECT DATE_FORMAT(pbt.effective_date, '%m/%Y') AS month_key,
		       COALESCE(SUM(pbt.total), 0) AS purchases
		FROM purchase_bill_totals pbt
		WHERE pbt.merchant_id = ?
		  AND pbt.effective_date >= DATE_SUB(NOW(), INTERVAL ? MONTH)
		GROUP BY month_key
		ORDER BY month_key
	`, companyID, numMonths)
	if err == nil {
		defer purchaseRows.Close()
		for purchaseRows.Next() {
			var key string
			var purch float64
			if purchaseRows.Scan(&key, &purch) == nil {
				if md, ok := monthlyMap[key]; ok {
					md.Purchases = purch
				}
			}
		}
	}

	monthlyRevenue := make([]string, numMonths)
	monthlyPurchases := make([]string, numMonths)
	monthlyProfit := make([]string, numMonths)
	for i, label := range monthLabels {
		md := monthlyMap[label]
		monthlyRevenue[i] = fmt.Sprintf("%.2f", md.Revenue)
		monthlyPurchases[i] = fmt.Sprintf("%.2f", md.Purchases)
		monthlyProfit[i] = fmt.Sprintf("%.2f", md.Revenue-md.Purchases)
	}

	// ── 5. RECENT INVOICES (last 10) ───────────────────────────────
	type recentInvoice struct {
		ID             int     `json:"id"`
		SequenceNumber int     `json:"sequence_number"`
		Total          string  `json:"total"`
		State          int     `json:"state"`
		StateLabel     string  `json:"state_label"`
		Date           string  `json:"date"`
		IsCreditNote   bool    `json:"is_credit_note"`
		UserPhone      *string `json:"user_phone_number"`
	}

	var recentInvoices []recentInvoice
	recentRows, err := h.DB.Query(`
		SELECT bt.id, bt.sequence_number, COALESCE(bt.total, 0), bt.state,
		       DATE_FORMAT(bt.effective_date, '%Y-%m-%d') AS date,
		       CASE WHEN cn.id IS NOT NULL THEN 1 ELSE 0 END AS is_credit,
		       bt.user_phone_number
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		WHERE bt.merchant_id = ?
		ORDER BY bt.effective_date DESC, bt.id DESC
		LIMIT 10
	`, companyID)
	if err == nil {
		defer recentRows.Close()
		for recentRows.Next() {
			var ri recentInvoice
			var total float64
			var isCN int
			if recentRows.Scan(&ri.ID, &ri.SequenceNumber, &total, &ri.State,
				&ri.Date, &isCN, &ri.UserPhone) == nil {
				ri.Total = fmt.Sprintf("%.2f", total)
				ri.IsCreditNote = isCN == 1
				switch ri.State {
				case 0:
					ri.StateLabel = "مسودة"
				case 1:
					ri.StateLabel = "قيد المعالجة"
				case 2:
					ri.StateLabel = "تمت المعالجة"
				case 3:
					ri.StateLabel = "صادرة"
				default:
					ri.StateLabel = "غير معروف"
				}
				recentInvoices = append(recentInvoices, ri)
			}
		}
	}
	if recentInvoices == nil {
		recentInvoices = []recentInvoice{}
	}

	// ── 6. LOW-STOCK PRODUCTS (quantity <= min_stock, top 10) ──────
	// product has no name — join articles via article_id = legacyArticleId
	type lowStockProduct struct {
		ID            int    `json:"id"`
		ArticleNumber string `json:"article_number"`
		Price         string `json:"price"`
		Quantity      string `json:"quantity"`
		CostPrice     string `json:"cost_price"`
		MinStock      int    `json:"min_stock"`
		StoreID       int    `json:"store_id"`
	}

	var lowStock []lowStockProduct
	lowRows, err := h.DB.Query(`
		SELECT p.id,
		       COALESCE(a.articleNumber, CAST(p.article_id AS CHAR)),
		       p.price, p.quantity, p.cost_price, p.min_stock, p.store_id
		FROM product p
		LEFT JOIN articles a ON a.legacyArticleId = p.article_id
		WHERE p.store_id IN (`+storeSubquery+`)
		  AND p.is_deleted = 0 AND p.quantity <= p.min_stock
		ORDER BY p.quantity ASC, p.id ASC
		LIMIT 10
	`, companyID)
	if err == nil {
		defer lowRows.Close()
		for lowRows.Next() {
			var ls lowStockProduct
			var price, qty, costPrice float64
			if lowRows.Scan(&ls.ID, &ls.ArticleNumber, &price, &qty,
				&costPrice, &ls.MinStock, &ls.StoreID) == nil {
				ls.Price = fmt.Sprintf("%.2f", price)
				ls.Quantity = fmt.Sprintf("%.3f", qty)
				ls.CostPrice = fmt.Sprintf("%.2f", costPrice)
				lowStock = append(lowStock, ls)
			}
		}
	}
	if lowStock == nil {
		lowStock = []lowStockProduct{}
	}

	var lowStockCount int
	h.DB.QueryRow(`
		SELECT COUNT(*) FROM product
		WHERE store_id IN (`+storeSubquery+`)
		  AND is_deleted = 0 AND quantity <= min_stock
	`, companyID).Scan(&lowStockCount)

	// ── 7. TOP PRODUCTS (by quantity in stock, top 8) ──────────────
	type topProduct struct {
		ID            int    `json:"id"`
		ArticleNumber string `json:"article_number"`
		Quantity      string `json:"quantity"`
		Price         string `json:"price"`
	}

	var topProducts []topProduct
	topProdRows, err := h.DB.Query(`
		SELECT p.id,
		       COALESCE(a.articleNumber, CAST(p.article_id AS CHAR)),
		       p.quantity, p.price
		FROM product p
		LEFT JOIN articles a ON a.legacyArticleId = p.article_id
		WHERE p.store_id IN (`+storeSubquery+`)
		  AND p.is_deleted = 0
		ORDER BY p.quantity DESC, p.id DESC
		LIMIT 8
	`, companyID)
	if err == nil {
		defer topProdRows.Close()
		for topProdRows.Next() {
			var tp topProduct
			var price, qty float64
			if topProdRows.Scan(&tp.ID, &tp.ArticleNumber, &qty, &price) == nil {
				tp.Quantity = fmt.Sprintf("%.3f", qty)
				tp.Price = fmt.Sprintf("%.2f", price)
				topProducts = append(topProducts, tp)
			}
		}
	}
	if topProducts == nil {
		topProducts = []topProduct{}
	}

	// ── 8. PRODUCT PRICE TIERS ─────────────────────────────────────
	type priceTier struct {
		Label    string `json:"label"`
		Count    int    `json:"count"`
		AvgPrice string `json:"avg_price"`
	}

	var tier1Count, tier2Count, tier3Count, tier4Count int
	var tier1Avg, tier2Avg, tier3Avg, tier4Avg float64

	tierBase := `SELECT COUNT(*), COALESCE(AVG(price),0) FROM product
		WHERE store_id IN (` + storeSubquery + `) AND is_deleted = 0`
	h.DB.QueryRow(tierBase+" AND price < 50", companyID).Scan(&tier1Count, &tier1Avg)
	h.DB.QueryRow(tierBase+" AND price >= 50 AND price < 200", companyID).Scan(&tier2Count, &tier2Avg)
	h.DB.QueryRow(tierBase+" AND price >= 200 AND price < 500", companyID).Scan(&tier3Count, &tier3Avg)
	h.DB.QueryRow(tierBase+" AND price >= 500", companyID).Scan(&tier4Count, &tier4Avg)

	marginTiers := []priceTier{
		{Label: "< 50 ر.س", Count: tier1Count, AvgPrice: fmt.Sprintf("%.2f", tier1Avg)},
		{Label: "50-200 ر.س", Count: tier2Count, AvgPrice: fmt.Sprintf("%.2f", tier2Avg)},
		{Label: "200-500 ر.س", Count: tier3Count, AvgPrice: fmt.Sprintf("%.2f", tier3Avg)},
		{Label: "500+ ر.س", Count: tier4Count, AvgPrice: fmt.Sprintf("%.2f", tier4Avg)},
	}

	// ── 9. SUPPLIER PERFORMANCE (top 5 by spend) ───────────────────
	type supplierPerf struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		BillCount  int    `json:"bill_count"`
		TotalSpent string `json:"total_spent"`
		AvgTotal   string `json:"avg_total"`
	}

	var supplierPerfs []supplierPerf
	suppRows, err := h.DB.Query(`
		SELECT s.id, s.name,
		       COUNT(pbt.id) AS bill_count,
		       COALESCE(SUM(pbt.total), 0) AS total_spent,
		       COALESCE(AVG(pbt.total), 0) AS avg_total
		FROM supplier s
		LEFT JOIN purchase_bill_totals pbt
		  ON pbt.supplier_id = s.id AND pbt.merchant_id = ?
		WHERE s.company_id = ? AND s.is_deleted = 0
		GROUP BY s.id, s.name
		ORDER BY total_spent DESC
		LIMIT 5
	`, companyID, companyID)
	if err == nil {
		defer suppRows.Close()
		for suppRows.Next() {
			var sp supplierPerf
			var totalSpent, avgTotal float64
			if suppRows.Scan(&sp.ID, &sp.Name, &sp.BillCount, &totalSpent, &avgTotal) == nil {
				sp.TotalSpent = fmt.Sprintf("%.2f", totalSpent)
				sp.AvgTotal = fmt.Sprintf("%.2f", avgTotal)
				supplierPerfs = append(supplierPerfs, sp)
			}
		}
	}
	if supplierPerfs == nil {
		supplierPerfs = []supplierPerf{}
	}

	// ── 10. FULFILLMENT & RETURN RATE ──────────────────────────────
	// No order table — fulfillment = issued / total invoices
	var issuedInvoices int
	h.DB.QueryRow(`
		SELECT COUNT(*) FROM bill b
		LEFT JOIN credit_note cn ON cn.bill_id = b.id
		WHERE b.merchant_id = ? AND b.state = 3 AND cn.id IS NULL
	`, companyID).Scan(&issuedInvoices)

	fulfillmentRate := 0.0
	if totalInvoices > 0 {
		fulfillmentRate = float64(issuedInvoices) * 100 / float64(totalInvoices)
	}

	returnRate := 0.0
	if totalInvoices > 0 {
		returnRate = float64(creditNoteCount) * 100 / float64(totalInvoices)
	}

	// Monthly return rates
	type monthlyReturn struct {
		Month       string `json:"month"`
		Invoices    int    `json:"invoices"`
		CreditNotes int    `json:"credit_notes"`
		Rate        string `json:"rate"`
	}

	var monthlyReturns []monthlyReturn
	returnRows, err := h.DB.Query(`
		SELECT DATE_FORMAT(b.effective_date, '%m/%Y') AS month_key,
		       SUM(CASE WHEN cn.id IS NULL THEN 1 ELSE 0 END) AS inv_count,
		       SUM(CASE WHEN cn.id IS NOT NULL THEN 1 ELSE 0 END) AS cn_count
		FROM bill b
		LEFT JOIN credit_note cn ON cn.bill_id = b.id
		WHERE b.merchant_id = ?
		  AND b.effective_date >= DATE_SUB(NOW(), INTERVAL ? MONTH)
		GROUP BY month_key
		ORDER BY month_key
	`, companyID, numMonths)
	if err == nil {
		defer returnRows.Close()
		for returnRows.Next() {
			var mr monthlyReturn
			if returnRows.Scan(&mr.Month, &mr.Invoices, &mr.CreditNotes) == nil {
				rate := 0.0
				if mr.Invoices > 0 {
					rate = float64(mr.CreditNotes) * 100 / float64(mr.Invoices)
				}
				mr.Rate = fmt.Sprintf("%.1f", rate)
				monthlyReturns = append(monthlyReturns, mr)
			}
		}
	}
	if monthlyReturns == nil {
		monthlyReturns = []monthlyReturn{}
	}

	// ── 11. INVENTORY TURNOVER ─────────────────────────────────────
	var totalInventoryValue float64
	h.DB.QueryRow(`
		SELECT COALESCE(SUM(price * quantity), 0)
		FROM product
		WHERE store_id IN (`+storeSubquery+`) AND is_deleted = 0
	`, companyID).Scan(&totalInventoryValue)

	invTurnover := 0.0
	if totalInventoryValue > 0 {
		invTurnover = totalPurchases / totalInventoryValue
	}

	// ── 12. WEEKDAY REVENUE (Sat=0 through Fri=6 for MENA) ────────
	type weekdayRev struct {
		Day     int    `json:"day"`
		DayName string `json:"day_name"`
		AvgRev  string `json:"avg_revenue"`
	}

	dayNames := []string{"السبت", "الأحد", "الإثنين", "الثلاثاء", "الأربعاء", "الخميس", "الجمعة"}
	weekdayRevenues := make([]weekdayRev, 7)
	for i := range weekdayRevenues {
		weekdayRevenues[i] = weekdayRev{Day: i, DayName: dayNames[i], AvgRev: "0.00"}
	}

	wdRows, err := h.DB.Query(`
		SELECT DAYOFWEEK(bt.effective_date) AS dow,
		       COALESCE(AVG(bt.total), 0) AS avg_rev,
		       COUNT(*) AS cnt
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		WHERE bt.merchant_id = ? AND cn.id IS NULL
		  AND bt.effective_date >= DATE_SUB(NOW(), INTERVAL ? MONTH)
		GROUP BY dow
	`, companyID, numMonths)
	if err == nil {
		defer wdRows.Close()
		for wdRows.Next() {
			var dow int
			var avgRev float64
			var cnt int
			if wdRows.Scan(&dow, &avgRev, &cnt) == nil {
				// MySQL DAYOFWEEK(1=Sun..7=Sat) → MENA index
				menaIdx := dow % 7
				if menaIdx >= 0 && menaIdx < 7 {
					weekdayRevenues[menaIdx].AvgRev = fmt.Sprintf("%.2f", avgRev)
				}
			}
		}
	}

	// ── 13. YoY REVENUE ────────────────────────────────────────────
	yoyRevenue := make([]string, numMonths)
	for i, label := range monthLabels {
		parts := strings.SplitN(label, "/", 2)
		if len(parts) != 2 {
			yoyRevenue[i] = "0.00"
			continue
		}
		month, _ := strconv.Atoi(parts[0])
		year, _ := strconv.Atoi(parts[1])
		prevYear := year - 1

		var rev float64
		h.DB.QueryRow(`
			SELECT COALESCE(SUM(bt.total), 0)
			FROM bill_totals bt
			LEFT JOIN credit_note cn ON cn.bill_id = bt.id
			WHERE bt.merchant_id = ? AND cn.id IS NULL
			  AND MONTH(bt.effective_date) = ? AND YEAR(bt.effective_date) = ?
		`, companyID, month, prevYear).Scan(&rev)
		yoyRevenue[i] = fmt.Sprintf("%.2f", rev)
	}

	// ── 14. TOP CLIENTS BY INVOICE VALUE ───────────────────────────
	// No order table — derive from bill.buyer_id → client.id
	type topClient struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		InvCount    int    `json:"invoice_count"`
		Total       string `json:"total"`
		LastInvoice string `json:"last_invoice"`
	}

	var topClients []topClient
	tcRows, err := h.DB.Query(`
		SELECT c.id, c.name,
		       COUNT(bt.id) AS inv_count,
		       COALESCE(SUM(bt.total), 0) AS total_value,
		       COALESCE(MAX(DATE_FORMAT(bt.effective_date, '%Y-%m-%d')), '-') AS last_inv
		FROM client c
		INNER JOIN bill_totals bt ON bt.buyer_id = c.id AND bt.merchant_id = ?
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		WHERE cn.id IS NULL AND c.is_deleted = 0
		GROUP BY c.id, c.name
		ORDER BY total_value DESC
		LIMIT 5
	`, companyID)
	if err == nil {
		defer tcRows.Close()
		for tcRows.Next() {
			var tc topClient
			var totalVal float64
			if tcRows.Scan(&tc.ID, &tc.Name, &tc.InvCount, &totalVal, &tc.LastInvoice) == nil {
				tc.Total = fmt.Sprintf("%.2f", totalVal)
				topClients = append(topClients, tc)
			}
		}
	}
	if topClients == nil {
		topClients = []topClient{}
	}

	// ── 15. CLIENT DISTRIBUTION ────────────────────────────────────
	// Active = has at least one invoice via buyer_id
	var activeClients int
	h.DB.QueryRow(`
		SELECT COUNT(DISTINCT bt.buyer_id)
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		WHERE bt.merchant_id = ? AND bt.buyer_id IS NOT NULL AND cn.id IS NULL
	`, companyID).Scan(&activeClients)
	inactiveClients := totalClients - activeClients
	if inactiveClients < 0 {
		inactiveClients = 0
	}

	// ── ASSEMBLE RESPONSE ──────────────────────────────────────────
	avgInvoiceValue := 0.0
	if totalInvoices > 0 {
		avgInvoiceValue = totalRevenue / float64(totalInvoices)
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{
			"total_invoices":       totalInvoices,
			"total_revenue":        fmt.Sprintf("%.2f", totalRevenue),
			"total_vat":            fmt.Sprintf("%.2f", totalVAT),
			"total_discount":       fmt.Sprintf("%.2f", totalDiscount),
			"total_products":       totalProducts,
			"total_clients":        totalClients,
			"total_suppliers":      totalSuppliers,
			"total_stores":         totalStores,
			"total_branches":       totalBranches,
			"pending_invoices":     pendingCount,
			"pending_amount":       fmt.Sprintf("%.2f", pendingAmount),
			"total_purchases":      fmt.Sprintf("%.2f", totalPurchases),
			"total_purchase_bills": totalPurchaseBills,
			"gross_profit":         fmt.Sprintf("%.2f", grossProfit),
			"gross_margin":         fmt.Sprintf("%.1f", grossMargin),
			"avg_invoice_value":    fmt.Sprintf("%.2f", avgInvoiceValue),
			"low_stock_count":      lowStockCount,
			"credit_note_count":    creditNoteCount,
			"credit_note_total":    fmt.Sprintf("%.2f", creditNoteTotal),
			"inv_turnover":         fmt.Sprintf("%.2f", invTurnover),
			"fulfillment_rate":     fmt.Sprintf("%.1f", fulfillmentRate),
			"return_rate":          fmt.Sprintf("%.1f", returnRate),
		},
		"status_counts": statusCounts,
		"charts": gin.H{
			"month_labels":      monthLabels,
			"monthly_revenue":   monthlyRevenue,
			"monthly_purchases": monthlyPurchases,
			"monthly_profit":    monthlyProfit,
			"yoy_revenue":       yoyRevenue,
			"weekday_revenue":   weekdayRevenues,
			"monthly_returns":   monthlyReturns,
		},
		"recent_invoices":    recentInvoices,
		"low_stock_products": lowStock,
		"top_products":       topProducts,
		"margin_tiers":       marginTiers,
		"supplier_perf":      supplierPerfs,
		"top_clients":        topClients,
		"client_distribution": gin.H{
			"active":   activeClients,
			"inactive": inactiveClients,
		},
		"filters": gin.H{
			"state":      stateFilter,
			"start_date": startDate,
			"end_date":   endDate,
			"months":     numMonths,
		},
	})
}

// ── GET /api/v2/dashboard/analytics ─────────────────────────────────────────
// AR/AP aging, cash flow, P&L statement, KPI trends.

func (h *handler) GetDashboardAnalytics(c *gin.Context) {
	companyID := h.getUserCompany(c)

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	monthsParam := c.DefaultQuery("months", "6")
	numMonths, _ := strconv.Atoi(monthsParam)
	if numMonths < 1 || numMonths > 24 {
		numMonths = 6
	}

	now := time.Now()
	monthLabels := make([]string, numMonths)
	for i := numMonths - 1; i >= 0; i-- {
		m := now.AddDate(0, -i, 0)
		monthLabels[numMonths-1-i] = m.Format("01/2006")
	}

	// ── 1. ACCOUNTS RECEIVABLE AGING (unpaid invoices) ─────────────
	type agingBucket struct {
		Label string `json:"label"`
		Count int    `json:"count"`
		Total string `json:"total"`
	}

	arBuckets := []agingBucket{
		{Label: "0-30 أيام (حالي)", Count: 0, Total: "0.00"},
		{Label: "31-60 أيام (متأخر)", Count: 0, Total: "0.00"},
		{Label: "61-90 أيام (متأخر جداً)", Count: 0, Total: "0.00"},
		{Label: "90+ أيام (حرج)", Count: 0, Total: "0.00"},
	}

	arRows, err := h.DB.Query(`
		SELECT
			CASE
				WHEN DATEDIFF(NOW(), bt.effective_date) <= 30 THEN 0
				WHEN DATEDIFF(NOW(), bt.effective_date) <= 60 THEN 1
				WHEN DATEDIFF(NOW(), bt.effective_date) <= 90 THEN 2
				ELSE 3
			END AS bucket,
			COUNT(*) AS cnt,
			COALESCE(SUM(bt.total), 0) AS total
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		WHERE bt.merchant_id = ? AND bt.state IN (0, 1) AND cn.id IS NULL
		GROUP BY bucket
		ORDER BY bucket
	`, companyID)
	if err == nil {
		defer arRows.Close()
		for arRows.Next() {
			var bucket, cnt int
			var total float64
			if arRows.Scan(&bucket, &cnt, &total) == nil && bucket >= 0 && bucket < 4 {
				arBuckets[bucket].Count = cnt
				arBuckets[bucket].Total = fmt.Sprintf("%.2f", total)
			}
		}
	}

	// ── 2. ACCOUNTS PAYABLE AGING (purchase bills) ─────────────────
	apBuckets := []agingBucket{
		{Label: "0-30 أيام (حالي)", Count: 0, Total: "0.00"},
		{Label: "31-60 أيام (متأخر)", Count: 0, Total: "0.00"},
		{Label: "61-90 أيام (متأخر جداً)", Count: 0, Total: "0.00"},
		{Label: "90+ أيام (حرج)", Count: 0, Total: "0.00"},
	}

	apRows, err := h.DB.Query(`
		SELECT
			CASE
				WHEN DATEDIFF(NOW(), pbt.effective_date) <= 30 THEN 0
				WHEN DATEDIFF(NOW(), pbt.effective_date) <= 60 THEN 1
				WHEN DATEDIFF(NOW(), pbt.effective_date) <= 90 THEN 2
				ELSE 3
			END AS bucket,
			COUNT(*) AS cnt,
			COALESCE(SUM(pbt.total), 0) AS total
		FROM purchase_bill_totals pbt
		WHERE pbt.merchant_id = ? AND pbt.state != 3
		GROUP BY bucket
		ORDER BY bucket
	`, companyID)
	if err == nil {
		defer apRows.Close()
		for apRows.Next() {
			var bucket, cnt int
			var total float64
			if apRows.Scan(&bucket, &cnt, &total) == nil && bucket >= 0 && bucket < 4 {
				apBuckets[bucket].Count = cnt
				apBuckets[bucket].Total = fmt.Sprintf("%.2f", total)
			}
		}
	}

	// ── 3. CASH FLOW (monthly inflow vs outflow) ───────────────────
	type cashFlowPoint struct {
		Month   string `json:"month"`
		Inflow  string `json:"inflow"`
		Outflow string `json:"outflow"`
		Net     string `json:"net"`
	}

	cfMap := make(map[string]*struct{ Inflow, Outflow float64 })
	for _, label := range monthLabels {
		cfMap[label] = &struct{ Inflow, Outflow float64 }{}
	}

	// Inflows = invoice revenue by month (via bill_totals)
	cfInRows, err := h.DB.Query(`
		SELECT DATE_FORMAT(bt.effective_date, '%m/%Y') AS mkey,
		       COALESCE(SUM(bt.total), 0)
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		WHERE bt.merchant_id = ? AND cn.id IS NULL
		  AND bt.effective_date >= DATE_SUB(NOW(), INTERVAL ? MONTH)
		GROUP BY mkey
	`, companyID, numMonths)
	if err == nil {
		defer cfInRows.Close()
		for cfInRows.Next() {
			var key string
			var val float64
			if cfInRows.Scan(&key, &val) == nil {
				if cf, ok := cfMap[key]; ok {
					cf.Inflow = val
				}
			}
		}
	}

	// Outflows = purchase bill totals by month
	cfOutRows, err := h.DB.Query(`
		SELECT DATE_FORMAT(pbt.effective_date, '%m/%Y') AS mkey,
		       COALESCE(SUM(pbt.total), 0)
		FROM purchase_bill_totals pbt
		WHERE pbt.merchant_id = ?
		  AND pbt.effective_date >= DATE_SUB(NOW(), INTERVAL ? MONTH)
		GROUP BY mkey
	`, companyID, numMonths)
	if err == nil {
		defer cfOutRows.Close()
		for cfOutRows.Next() {
			var key string
			var val float64
			if cfOutRows.Scan(&key, &val) == nil {
				if cf, ok := cfMap[key]; ok {
					cf.Outflow = val
				}
			}
		}
	}

	cashFlow := make([]cashFlowPoint, numMonths)
	for i, label := range monthLabels {
		cf := cfMap[label]
		cashFlow[i] = cashFlowPoint{
			Month:   label,
			Inflow:  fmt.Sprintf("%.2f", cf.Inflow),
			Outflow: fmt.Sprintf("%.2f", cf.Outflow),
			Net:     fmt.Sprintf("%.2f", cf.Inflow-cf.Outflow),
		}
	}

	// ── 4. P&L STATEMENT ───────────────────────────────────────────
	type pnlStatement struct {
		Revenue      string   `json:"revenue"`
		COGS         string   `json:"cogs"`
		GrossProfit  string   `json:"gross_profit"`
		GrossMargin  string   `json:"gross_margin"`
		MonthLabels  []string `json:"month_labels"`
		MonthRevenue []string `json:"month_revenue"`
		MonthCOGS    []string `json:"month_cogs"`
		MonthProfit  []string `json:"month_profit"`
	}

	var totalRev, totalCOGS float64
	monthRev := make([]string, numMonths)
	monthCOGS := make([]string, numMonths)
	monthProfit := make([]string, numMonths)

	for i, label := range monthLabels {
		cf := cfMap[label]
		totalRev += cf.Inflow
		totalCOGS += cf.Outflow
		monthRev[i] = fmt.Sprintf("%.2f", cf.Inflow)
		monthCOGS[i] = fmt.Sprintf("%.2f", cf.Outflow)
		monthProfit[i] = fmt.Sprintf("%.2f", cf.Inflow-cf.Outflow)
	}

	gp := totalRev - totalCOGS
	gm := 0.0
	if totalRev > 0 {
		gm = gp * 100 / totalRev
	}

	pnl := pnlStatement{
		Revenue:      fmt.Sprintf("%.2f", totalRev),
		COGS:         fmt.Sprintf("%.2f", totalCOGS),
		GrossProfit:  fmt.Sprintf("%.2f", gp),
		GrossMargin:  fmt.Sprintf("%.1f", gm),
		MonthLabels:  monthLabels,
		MonthRevenue: monthRev,
		MonthCOGS:    monthCOGS,
		MonthProfit:  monthProfit,
	}

	// ── 5. KPI TRENDS (current vs previous period) ─────────────────
	type kpiTrend struct {
		Direction string `json:"direction"`
		Percent   string `json:"percent"`
		Arrow     string `json:"arrow"`
	}

	var periodStart, periodEnd time.Time
	if startDate != "" && endDate != "" {
		ps, _ := time.Parse("2006-01-02", startDate)
		pe, _ := time.Parse("2006-01-02", endDate)
		if !ps.IsZero() && !pe.IsZero() {
			periodStart = ps
			periodEnd = pe.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
	}
	if periodStart.IsZero() || periodEnd.IsZero() {
		periodEnd = now
		periodStart = now.AddDate(0, 0, -7)
	}

	duration := periodEnd.Sub(periodStart)
	prevEnd := periodStart.Add(-time.Nanosecond)
	prevStart := prevEnd.Add(-duration)

	var curRev, prevRev, curPurch, prevPurch float64
	var curInvCount, prevInvCount int

	h.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(bt.total), 0)
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		WHERE bt.merchant_id = ? AND cn.id IS NULL
		  AND bt.effective_date >= ? AND bt.effective_date <= ?
	`, companyID,
		periodStart.Format(time.RFC3339),
		periodEnd.Format(time.RFC3339)).Scan(&curInvCount, &curRev)

	h.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(bt.total), 0)
		FROM bill_totals bt
		LEFT JOIN credit_note cn ON cn.bill_id = bt.id
		WHERE bt.merchant_id = ? AND cn.id IS NULL
		  AND bt.effective_date >= ? AND bt.effective_date <= ?
	`, companyID,
		prevStart.Format(time.RFC3339),
		prevEnd.Format(time.RFC3339)).Scan(&prevInvCount, &prevRev)

	h.DB.QueryRow(`
		SELECT COALESCE(SUM(pbt.total), 0)
		FROM purchase_bill_totals pbt
		WHERE pbt.merchant_id = ?
		  AND pbt.effective_date >= ? AND pbt.effective_date <= ?
	`, companyID,
		periodStart.Format(time.RFC3339),
		periodEnd.Format(time.RFC3339)).Scan(&curPurch)

	h.DB.QueryRow(`
		SELECT COALESCE(SUM(pbt.total), 0)
		FROM purchase_bill_totals pbt
		WHERE pbt.merchant_id = ?
		  AND pbt.effective_date >= ? AND pbt.effective_date <= ?
	`, companyID,
		prevStart.Format(time.RFC3339),
		prevEnd.Format(time.RFC3339)).Scan(&prevPurch)

	makeTrendFn := func(cur, prev float64) kpiTrend {
		if prev == 0 && cur == 0 {
			return kpiTrend{Direction: "flat", Percent: "0", Arrow: "—"}
		}
		if prev == 0 {
			return kpiTrend{Direction: "up", Percent: "100", Arrow: "↑"}
		}
		pct := ((cur - prev) / math.Abs(prev)) * 100
		pctStr := fmt.Sprintf("%.1f", math.Abs(pct))
		dir := "flat"
		arrow := "—"
		if pct > 0.5 {
			dir = "up"
			arrow = "↑"
		} else if pct < -0.5 {
			dir = "down"
			arrow = "↓"
		}
		return kpiTrend{Direction: dir, Percent: pctStr, Arrow: arrow}
	}

	curProfit := curRev - curPurch
	prevProfit := prevRev - prevPurch

	kpiTrends := gin.H{
		"invoices":        makeTrendFn(float64(curInvCount), float64(prevInvCount)),
		"revenue":         makeTrendFn(curRev, prevRev),
		"purchases_total": makeTrendFn(curPurch, prevPurch),
		"gross_profit":    makeTrendFn(curProfit, prevProfit),
	}

	c.JSON(http.StatusOK, gin.H{
		"ar_aging":   arBuckets,
		"ap_aging":   apBuckets,
		"cash_flow":  cashFlow,
		"pnl":        pnl,
		"kpi_trends": kpiTrends,
	})
}

// ── GET /api/v2/dashboard/compare ───────────────────────────────────────────
// Compares two arbitrary date periods side by side.

func (h *handler) GetDashboardCompare(c *gin.Context) {
	companyID := h.getUserCompany(c)

	aStart := c.Query("a_start")
	aEnd := c.Query("a_end")
	bStart := c.Query("b_start")
	bEnd := c.Query("b_end")

	if aStart == "" || aEnd == "" || bStart == "" || bEnd == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "يرجى تحديد فترتين للمقارنة"})
		return
	}

	type periodStats struct {
		Invoices   int    `json:"invoices"`
		Revenue    string `json:"revenue"`
		Purchases  string `json:"purchases"`
		Profit     string `json:"profit"`
		AvgInvoice string `json:"avg_invoice"`
		Pending    string `json:"pending"`
		Margin     string `json:"margin"`
		Issued     int    `json:"issued"`
		Draft      int    `json:"draft"`
	}

	computePeriod := func(start, end string) periodStats {
		var ps periodStats
		var revenue, purchases, pending float64
		var issued, draft int

		h.DB.QueryRow(`
			SELECT COUNT(*), COALESCE(SUM(bt.total), 0)
			FROM bill_totals bt
			LEFT JOIN credit_note cn ON cn.bill_id = bt.id
			WHERE bt.merchant_id = ? AND cn.id IS NULL
			  AND DATE(bt.effective_date) >= ? AND DATE(bt.effective_date) <= ?
		`, companyID, start, end).Scan(&ps.Invoices, &revenue)

		h.DB.QueryRow(`
			SELECT COALESCE(SUM(bt.total), 0) FROM bill_totals bt
			LEFT JOIN credit_note cn ON cn.bill_id = bt.id
			WHERE bt.merchant_id = ? AND cn.id IS NULL
			  AND bt.state IN (0, 1)
			  AND DATE(bt.effective_date) >= ? AND DATE(bt.effective_date) <= ?
		`, companyID, start, end).Scan(&pending)

		h.DB.QueryRow(`
			SELECT COUNT(*) FROM bill b
			LEFT JOIN credit_note cn ON cn.bill_id = b.id
			WHERE b.merchant_id = ? AND cn.id IS NULL AND b.state = 3
			  AND DATE(b.effective_date) >= ? AND DATE(b.effective_date) <= ?
		`, companyID, start, end).Scan(&issued)

		h.DB.QueryRow(`
			SELECT COUNT(*) FROM bill b
			LEFT JOIN credit_note cn ON cn.bill_id = b.id
			WHERE b.merchant_id = ? AND cn.id IS NULL AND b.state = 0
			  AND DATE(b.effective_date) >= ? AND DATE(b.effective_date) <= ?
		`, companyID, start, end).Scan(&draft)

		h.DB.QueryRow(`
			SELECT COALESCE(SUM(pbt.total), 0) FROM purchase_bill_totals pbt
			WHERE pbt.merchant_id = ?
			  AND DATE(pbt.effective_date) >= ? AND DATE(pbt.effective_date) <= ?
		`, companyID, start, end).Scan(&purchases)

		profit := revenue - purchases
		avg := 0.0
		if ps.Invoices > 0 {
			avg = revenue / float64(ps.Invoices)
		}
		margin := 0.0
		if revenue > 0 {
			margin = profit * 100 / revenue
		}

		ps.Revenue = fmt.Sprintf("%.2f", revenue)
		ps.Purchases = fmt.Sprintf("%.2f", purchases)
		ps.Profit = fmt.Sprintf("%.2f", profit)
		ps.AvgInvoice = fmt.Sprintf("%.2f", avg)
		ps.Pending = fmt.Sprintf("%.2f", pending)
		ps.Margin = fmt.Sprintf("%.1f", margin)
		ps.Issued = issued
		ps.Draft = draft

		return ps
	}

	c.JSON(http.StatusOK, gin.H{
		"period_a": computePeriod(aStart, aEnd),
		"period_b": computePeriod(bStart, bEnd),
	})
}
