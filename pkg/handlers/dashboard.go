// Package handlers — drop-in replacement for pkg/handlers/dashboard.go.
//
// Single-tenant: each database hosts exactly one company, so no merchant_id
// or company_id parameters are passed to any query.
//
// Architecture:
//   • All SQL lives in pkg/db/queries/dashboard.sql (sqlc-generated).
//   • All response shapes live in pkg/model/dashboard.go (typed structs).
//   • Independent aggregates run in parallel under a single 10s context.
//
// To wire up:
//   1. Drop queries/dashboard.sql into pkg/db/queries/.
//   2. Drop model/dashboard.go into pkg/model/.
//   3. Run `sqlc generate`.
//   4. Replace pkg/handlers/dashboard.go with this file.

package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/gin-gonic/gin"
)

// ── GET /api/v2/dashboard ───────────────────────────────────────────────────

func (h *handler) GetDashboard(c *gin.Context) {
	stateFilter := c.Query("state")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	monthsParam := c.DefaultQuery("months", "6")
	numMonths, _ := strconv.Atoi(monthsParam)
	if numMonths < 1 || numMonths > 24 {
		numMonths = 6
	}

	now := time.Now()
	monthLabels := buildMonthLabels(now, numMonths)
	months := int32(numMonths)

	stateNarg := optInt32(stateFilter)
	startNarg := optDate(startDate, false)
	endNarg := optDate(endDate, true)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	q := h.queries

	var (
		counts         db.GetDashboardCountsRow
		sales          db.GetDashboardSalesKPIsRow
		credit         db.GetDashboardCreditNoteStatsRow
		purchases      db.GetDashboardPurchaseKPIsRow
		monthlyRev     []db.GetMonthlyRevenueRow
		monthlyPur     []db.GetMonthlyPurchasesRow
		recent         []db.GetDashboardRecentInvoicesRow
		lowStock       []db.GetDashboardLowStockProductsRow
		topStock       []db.GetDashboardTopStockProductsRow
		tiersRows      []db.GetDashboardPriceTiersRow
		supplierRows   []db.GetDashboardSupplierPerformanceRow
		monthlyRetRows []db.GetDashboardMonthlyReturnsRow
		weekdayRows    []db.GetDashboardWeekdayRevenueRow
		yoyRows        []db.GetDashboardYoYRevenueRow
		topClientRows  []db.GetDashboardTopClientsRow
		orderStats     db.GetDashboardOrderStatsRow
		avgProcessing  any
		clvRows        []db.GetDashboardOrdersCLVRow
	)

	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() { defer wg.Done(); fn() }()
	}

	run(func() { counts, _ = q.GetDashboardCounts(ctx) })
	run(func() {
		sales, _ = q.GetDashboardSalesKPIs(ctx, db.GetDashboardSalesKPIsParams{
			State: stateNarg, StartDate: startNarg, EndDate: endNarg,
		})
	})
	run(func() {
		credit, _ = q.GetDashboardCreditNoteStats(ctx, db.GetDashboardCreditNoteStatsParams{
			StartDate: startNarg, EndDate: endNarg,
		})
	})
	run(func() {
		purchases, _ = q.GetDashboardPurchaseKPIs(ctx, db.GetDashboardPurchaseKPIsParams{
			StartDate: startNarg, EndDate: endNarg,
		})
	})
	run(func() { monthlyRev, _ = q.GetMonthlyRevenue(ctx, months) })
	run(func() { monthlyPur, _ = q.GetMonthlyPurchases(ctx, months) })
	run(func() { recent, _ = q.GetDashboardRecentInvoices(ctx) })
	run(func() { lowStock, _ = q.GetDashboardLowStockProducts(ctx) })
	run(func() { topStock, _ = q.GetDashboardTopStockProducts(ctx) })
	run(func() { tiersRows, _ = q.GetDashboardPriceTiers(ctx) })
	run(func() { supplierRows, _ = q.GetDashboardSupplierPerformance(ctx) })
	run(func() { monthlyRetRows, _ = q.GetDashboardMonthlyReturns(ctx, months) })
	run(func() { weekdayRows, _ = q.GetDashboardWeekdayRevenue(ctx, months) })
	run(func() {
		yoyRows, _ = q.GetDashboardYoYRevenue(ctx, db.GetDashboardYoYRevenueParams{
			Anchor:          now,
			MonthsBackStart: int32(12 + numMonths - 1),
			MonthsBackEnd:   int32(12 - 1),
		})
	})
	run(func() { topClientRows, _ = q.GetDashboardTopClients(ctx) })
	run(func() { orderStats, _ = q.GetDashboardOrderStats(ctx) })
	run(func() { avgProcessing, _ = q.GetDashboardAvgOrderProcessing(ctx) })
	run(func() { clvRows, _ = q.GetDashboardOrdersCLV(ctx) })

	wg.Wait()

	// ── Assemble ───────────────────────────────────────────────────
	statusCounts := map[string]int64{
		"draft":      toInt64(sales.DraftCount),
		"processing": toInt64(sales.ProcessingCount),
		"processed":  toInt64(sales.ProcessedCount),
		"issued":     toInt64(sales.IssuedCount),
	}
	totalInvoices := statusCounts["draft"] + statusCounts["processing"] +
		statusCounts["processed"] + statusCounts["issued"]

	totalRevenue := toFloat(sales.TotalRevenue)
	totalVAT := toFloat(sales.TotalVat)
	totalDiscount := toFloat(sales.TotalDiscount)
	pendingAmount := toFloat(sales.PendingAmount)
	totalPurchases := toFloat(purchases.PurchasesTotal)
	totalPurchaseVAT := toFloat(purchases.PurchasesVat)
	creditNoteTotal := toFloat(credit.CreditNoteTotal)
	totalInventoryValue := toFloat(counts.InventoryValue)

	grossProfit := totalRevenue - totalPurchases
	grossMargin := pct(grossProfit, totalRevenue)
	avgInvoiceValue := 0.0
	if totalInvoices > 0 {
		avgInvoiceValue = totalRevenue / float64(totalInvoices)
	}
	invTurnover := 0.0
	if totalInventoryValue > 0 {
		invTurnover = totalPurchases / totalInventoryValue
	}
	fulfillmentRate, returnRate := 0.0, 0.0
	if totalInvoices > 0 {
		fulfillmentRate = float64(toInt64(counts.IssuedInvoices)) * 100 / float64(totalInvoices)
		returnRate = float64(toInt64(credit.CreditNoteCount)) * 100 / float64(totalInvoices)
	}

	// monthly chart
	revMap := map[string]float64{}
	for _, r := range monthlyRev {
		revMap[r.MonthKey] = toFloat(r.Revenue)
	}
	purMap := map[string]float64{}
	for _, r := range monthlyPur {
		purMap[r.MonthKey] = toFloat(r.Purchases)
	}
	monthlyRevenueOut := make([]string, numMonths)
	monthlyPurchasesOut := make([]string, numMonths)
	monthlyProfitOut := make([]string, numMonths)
	for i, label := range monthLabels {
		r, p := revMap[label], purMap[label]
		monthlyRevenueOut[i] = fmt.Sprintf("%.2f", r)
		monthlyPurchasesOut[i] = fmt.Sprintf("%.2f", p)
		monthlyProfitOut[i] = fmt.Sprintf("%.2f", r-p)
	}

	// recent invoices
	recentInvoices := make([]model.DashboardInvoice, 0, len(recent))
	for _, r := range recent {
		recentInvoices = append(recentInvoices, model.DashboardInvoice{
			ID:              toInt(r.ID),
			SequenceNumber:  toInt64(r.SequenceNumber),
			Total:           fmt.Sprintf("%.2f", toFloat(r.Total)),
			State:           toInt(r.State),
			StateLabel:      stateLabel(toInt(r.State)),
			Date:            toString(r.EffectiveDate),
			IsCreditNote:    toBool(r.IsCreditNote),
			UserPhoneNumber: derefStr(r.UserPhoneNumber),
		})
	}

	// low stock + top stock
	lowStockOut := make([]model.DashboardLowStock, 0, len(lowStock))
	for _, l := range lowStock {
		lowStockOut = append(lowStockOut, model.DashboardLowStock{
			ID:            toInt(l.ID),
			ArticleNumber: l.ArticleNumber,
			Price:         fmt.Sprintf("%.2f", toFloat(l.Price)),
			Quantity:      fmt.Sprintf("%.3f", toFloat(l.Quantity)),
			CostPrice:     fmt.Sprintf("%.2f", toFloat(l.CostPrice)),
			MinStock:      toInt(l.MinStock),
			StoreID:       toInt(l.StoreID),
		})
	}
	topProductsOut := make([]model.DashboardTopProduct, 0, len(topStock))
	for _, t := range topStock {
		topProductsOut = append(topProductsOut, model.DashboardTopProduct{
			ID:            toInt(t.ID),
			ArticleNumber: t.ArticleNumber,
			Quantity:      fmt.Sprintf("%.3f", toFloat(t.Quantity)),
			Price:         fmt.Sprintf("%.2f", toFloat(t.Price)),
		})
	}

	// price tiers
	tierLabels := []string{"< 50 ر.س", "50-200 ر.س", "200-500 ر.س", "500+ ر.س"}
	marginTiers := make([]model.DashboardTier, 4)
	for i, l := range tierLabels {
		marginTiers[i] = model.DashboardTier{Label: l, Count: 0, AvgPrice: "0.00"}
	}
	for _, t := range tiersRows {
		idx := toInt(t.Tier)
		if idx >= 0 && idx < 4 {
			marginTiers[idx].Count = toInt(t.ProductCount)
			marginTiers[idx].AvgPrice = fmt.Sprintf("%.2f", toFloat(t.AvgPrice))
		}
	}

	// supplier performance
	supplierPerfsOut := make([]model.DashboardSupplier, 0, len(supplierRows))
	for _, s := range supplierRows {
		supplierPerfsOut = append(supplierPerfsOut, model.DashboardSupplier{
			ID:         toInt(s.ID),
			Name:       s.Name,
			BillCount:  toInt(s.BillCount),
			TotalSpent: fmt.Sprintf("%.2f", toFloat(s.TotalSpent)),
			AvgTotal:   fmt.Sprintf("%.2f", toFloat(s.AvgTotal)),
		})
	}

	// monthly returns
	monthlyReturnsOut := make([]model.DashboardMonthlyReturn, 0, len(monthlyRetRows))
	for _, r := range monthlyRetRows {
		invCount := toInt(r.InvoiceCount)
		cnCount := toInt(r.CreditNoteCount)
		rate := 0.0
		if invCount > 0 {
			rate = float64(cnCount) * 100 / float64(invCount)
		}
		monthlyReturnsOut = append(monthlyReturnsOut, model.DashboardMonthlyReturn{
			Month:       r.MonthKey,
			Invoices:    invCount,
			CreditNotes: cnCount,
			Rate:        fmt.Sprintf("%.1f", rate),
		})
	}

	// weekday revenue
	dayNames := []string{"السبت", "الأحد", "الإثنين", "الثلاثاء", "الأربعاء", "الخميس", "الجمعة"}
	weekdayRevenuesOut := make([]model.DashboardWeekdayRevenue, 7)
	for i := range weekdayRevenuesOut {
		weekdayRevenuesOut[i] = model.DashboardWeekdayRevenue{Day: i, DayName: dayNames[i], AvgRevenue: "0.00"}
	}
	for _, w := range weekdayRows {
		// MySQL DAYOFWEEK: 1=Sunday … 7=Saturday → MENA Sat=0..Fri=6 = dow % 7
		menaIdx := toInt(w.Dow) % 7
		if menaIdx >= 0 && menaIdx < 7 {
			weekdayRevenuesOut[menaIdx].AvgRevenue = fmt.Sprintf("%.2f", toFloat(w.AvgRevenue))
		}
	}

	// YoY revenue: align last-year keys with this-year monthLabels.
	yoyMap := map[string]float64{}
	for _, y := range yoyRows {
		yoyMap[y.MonthKey] = toFloat(y.Revenue)
	}
	yoyRevenue := make([]string, numMonths)
	for i, label := range monthLabels {
		parts := strings.SplitN(label, "/", 2)
		if len(parts) != 2 {
			yoyRevenue[i] = "0.00"
			continue
		}
		mo, _ := strconv.Atoi(parts[0])
		yr, _ := strconv.Atoi(parts[1])
		yoyRevenue[i] = fmt.Sprintf("%.2f", yoyMap[fmt.Sprintf("%02d/%04d", mo, yr-1)])
	}

	// top clients
	topClientsOut := make([]model.DashboardTopClient, 0, len(topClientRows))
	for _, t := range topClientRows {
		topClientsOut = append(topClientsOut, model.DashboardTopClient{
			ID:           toInt(t.ID),
			Name:         t.Name,
			InvoiceCount: toInt(t.InvoiceCount),
			Total:        fmt.Sprintf("%.2f", toFloat(t.TotalValue)),
			LastInvoice:  toString(t.LastInvoiceDate),
		})
	}

	// CLV — top clients by total order value
	topCLVOut := make([]model.DashboardCLVClient, 0, len(clvRows))
	clvValues := make([]float64, 0, len(clvRows))
	for _, r := range clvRows {
		v := toFloat(r.TotalValue)
		clvValues = append(clvValues, v)
		topCLVOut = append(topCLVOut, model.DashboardCLVClient{
			ClientID:   toInt(r.ClientID),
			Name:       toString(r.ClientName),
			OrderCount: toInt(r.OrderCount),
			Value:      fmt.Sprintf("%.2f", v),
		})
	}
	clientConcentration := concentrationRisk(clvValues, 3)

	totalClients := toInt64(counts.TotalClients)
	activeClients := toInt64(counts.ActiveClients)
	inactiveClients := totalClients - activeClients
	if inactiveClients < 0 {
		inactiveClients = 0
	}

	c.JSON(http.StatusOK, model.DashboardResponse{
		Stats: model.DashboardStats{
			TotalInvoices:       totalInvoices,
			TotalRevenue:        fmt.Sprintf("%.2f", totalRevenue),
			TotalVAT:            fmt.Sprintf("%.2f", totalVAT),
			TotalDiscount:       fmt.Sprintf("%.2f", totalDiscount),
			TotalProducts:       toInt64(counts.TotalProducts),
			TotalClients:        totalClients,
			TotalSuppliers:      toInt64(counts.TotalSuppliers),
			TotalStores:         toInt64(counts.TotalStores),
			TotalBranches:       toInt64(counts.TotalBranches),
			PendingInvoices:     statusCounts["draft"] + statusCounts["processing"],
			PendingAmount:       fmt.Sprintf("%.2f", pendingAmount),
			TotalPurchases:      fmt.Sprintf("%.2f", totalPurchases),
			TotalPurchaseBills:  toInt64(purchases.PurchaseCount),
			TotalPurchaseVAT:    fmt.Sprintf("%.2f", totalPurchaseVAT),
			GrossProfit:         fmt.Sprintf("%.2f", grossProfit),
			GrossMargin:         fmt.Sprintf("%.1f", grossMargin),
			AvgInvoiceValue:     fmt.Sprintf("%.2f", avgInvoiceValue),
			LowStockCount:       toInt64(counts.LowStockCount),
			CreditNoteCount:     toInt64(credit.CreditNoteCount),
			CreditNoteTotal:     fmt.Sprintf("%.2f", creditNoteTotal),
			InvTurnover:         fmt.Sprintf("%.2f", invTurnover),
			FulfillmentRate:     fmt.Sprintf("%.1f", fulfillmentRate),
			ReturnRate:          fmt.Sprintf("%.1f", returnRate),
			TotalOrders:         toInt64(orderStats.TotalOrders),
			PendingOrders:       toInt64(orderStats.PendingOrders),
			CompletedOrders:     toInt64(orderStats.CompletedOrders),
			CancelledOrders:     toInt64(orderStats.CancelledOrders),
			TotalOrdersAmount:   fmt.Sprintf("%.2f", toFloat(orderStats.TotalOrdersAmount)),
			AvgProcessingDays:   fmt.Sprintf("%.1f", toFloat(avgProcessing)),
			ClientConcentration: fmt.Sprintf("%.1f", clientConcentration),
		},
		StatusCounts: statusCounts,
		Charts: model.DashboardCharts{
			MonthLabels:      monthLabels,
			MonthlyRevenue:   monthlyRevenueOut,
			MonthlyPurchases: monthlyPurchasesOut,
			MonthlyProfit:    monthlyProfitOut,
			YoYRevenue:       yoyRevenue,
			WeekdayRevenue:   weekdayRevenuesOut,
			MonthlyReturns:   monthlyReturnsOut,
		},
		RecentInvoices:   recentInvoices,
		LowStockProducts: lowStockOut,
		TopProducts:      topProductsOut,
		MarginTiers:      marginTiers,
		SupplierPerf:     supplierPerfsOut,
		TopClients:       topClientsOut,
		TopCLV:           topCLVOut,
		ClientDistribution: model.DashboardClientDist{
			Active:   activeClients,
			Inactive: inactiveClients,
		},
		Filters: model.DashboardFilters{
			State:     stateFilter,
			StartDate: startDate,
			EndDate:   endDate,
			Months:    numMonths,
		},
	})
}

// ── GET /api/v2/dashboard/analytics ─────────────────────────────────────────

func (h *handler) GetDashboardAnalytics(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	monthsParam := c.DefaultQuery("months", "6")
	numMonths, _ := strconv.Atoi(monthsParam)
	if numMonths < 1 || numMonths > 24 {
		numMonths = 6
	}

	now := time.Now()
	monthLabels := buildMonthLabels(now, numMonths)
	months := int32(numMonths)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	q := h.queries

	// KPI-trend period (default = trailing 7d)
	periodStart, periodEnd := now.AddDate(0, 0, -7), now
	if startDate != "" && endDate != "" {
		if ps, err := time.Parse("2006-01-02", startDate); err == nil {
			if pe, err := time.Parse("2006-01-02", endDate); err == nil {
				periodStart = ps
				periodEnd = pe.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			}
		}
	}
	duration := periodEnd.Sub(periodStart)
	prevEnd := periodStart.Add(-time.Nanosecond)
	prevStart := prevEnd.Add(-duration)

	var (
		arRows     []db.GetDashboardARAgingRow
		apRows     []db.GetDashboardAPAgingRow
		monthlyRev []db.GetMonthlyRevenueRow
		monthlyPur []db.GetMonthlyPurchasesRow
		curInv     db.GetDashboardPeriodInvoicesRow
		prevInv    db.GetDashboardPeriodInvoicesRow
		curPurch   float64
		prevPurch  float64
		vatQRows   []db.GetDashboardVATQuarterlyRow
		bsRows     []db.GetDashboardBalanceSheetRow
		opexSum    db.GetDashboardOpExSummaryRow
		opexCats   []db.GetDashboardOpExByCategoryRow
		zatca      db.GetDashboardZATCAStatsRow
		payTrack   db.GetDashboardPaymentTrackingRow
	)

	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() { defer wg.Done(); fn() }()
	}

	run(func() { arRows, _ = q.GetDashboardARAging(ctx) })
	run(func() { apRows, _ = q.GetDashboardAPAging(ctx) })
	run(func() { monthlyRev, _ = q.GetMonthlyRevenue(ctx, months) })
	run(func() { monthlyPur, _ = q.GetMonthlyPurchases(ctx, months) })
	run(func() {
		curInv, _ = q.GetDashboardPeriodInvoices(ctx, db.GetDashboardPeriodInvoicesParams{
			StartDate: periodStart,
			EndDate:   periodEnd,
		})
	})
	run(func() {
		prevInv, _ = q.GetDashboardPeriodInvoices(ctx, db.GetDashboardPeriodInvoicesParams{
			StartDate: prevStart,
			EndDate:   prevEnd,
		})
	})
	run(func() {
		v, _ := q.GetDashboardPeriodPurchases(ctx, db.GetDashboardPeriodPurchasesParams{
			StartDate: periodStart,
			EndDate:   periodEnd,
		})
		curPurch = toFloat(v)
	})
	run(func() {
		v, _ := q.GetDashboardPeriodPurchases(ctx, db.GetDashboardPeriodPurchasesParams{
			StartDate: prevStart,
			EndDate:   prevEnd,
		})
		prevPurch = toFloat(v)
	})
	run(func() { vatQRows, _ = q.GetDashboardVATQuarterly(ctx) })
	run(func() { bsRows, _ = q.GetDashboardBalanceSheet(ctx, periodEnd) })
	run(func() {
		opexSum, _ = q.GetDashboardOpExSummary(ctx, db.GetDashboardOpExSummaryParams{
			StartDate: periodStart,
			EndDate:   periodEnd,
		})
	})
	run(func() {
		opexCats, _ = q.GetDashboardOpExByCategory(ctx, db.GetDashboardOpExByCategoryParams{
			StartDate: periodStart,
			EndDate:   periodEnd,
		})
	})
	run(func() { zatca, _ = q.GetDashboardZATCAStats(ctx) })
	run(func() {
		payTrack, _ = q.GetDashboardPaymentTracking(ctx, db.GetDashboardPaymentTrackingParams{
			StartDate: periodStart,
			EndDate:   periodEnd,
		})
	})

	wg.Wait()

	// AR / AP buckets
	bucketLabels := []string{
		"0-30 أيام (حالي)", "31-60 أيام (متأخر)",
		"61-90 أيام (متأخر جداً)", "90+ أيام (حرج)",
	}
	makeBuckets := func() []model.DashboardAgingBucket {
		out := make([]model.DashboardAgingBucket, 4)
		for i, l := range bucketLabels {
			out[i] = model.DashboardAgingBucket{Label: l, Count: 0, Total: "0.00"}
		}
		return out
	}
	arBuckets := makeBuckets()
	for _, r := range arRows {
		idx := toInt(r.Bucket)
		if idx >= 0 && idx < 4 {
			arBuckets[idx].Count = toInt(r.BillCount)
			arBuckets[idx].Total = fmt.Sprintf("%.2f", toFloat(r.Total))
		}
	}
	apBuckets := makeBuckets()
	for _, r := range apRows {
		idx := toInt(r.Bucket)
		if idx >= 0 && idx < 4 {
			apBuckets[idx].Count = toInt(r.BillCount)
			apBuckets[idx].Total = fmt.Sprintf("%.2f", toFloat(r.Total))
		}
	}

	// Cash flow + P&L
	revMap := map[string]float64{}
	for _, r := range monthlyRev {
		revMap[r.MonthKey] = toFloat(r.Revenue)
	}
	purMap := map[string]float64{}
	for _, r := range monthlyPur {
		purMap[r.MonthKey] = toFloat(r.Purchases)
	}

	cashFlow := make([]model.DashboardCashFlowMonth, numMonths)
	monthRev := make([]string, numMonths)
	monthCOGS := make([]string, numMonths)
	monthProfit := make([]string, numMonths)
	var totalRev, totalCOGS float64
	for i, label := range monthLabels {
		inflow, outflow := revMap[label], purMap[label]
		net := inflow - outflow
		cashFlow[i] = model.DashboardCashFlowMonth{
			Month:   label,
			Inflow:  fmt.Sprintf("%.2f", inflow),
			Outflow: fmt.Sprintf("%.2f", outflow),
			Net:     fmt.Sprintf("%.2f", net),
		}
		totalRev += inflow
		totalCOGS += outflow
		monthRev[i] = fmt.Sprintf("%.2f", inflow)
		monthCOGS[i] = fmt.Sprintf("%.2f", outflow)
		monthProfit[i] = fmt.Sprintf("%.2f", net)
	}

	gp := totalRev - totalCOGS
	pnl := model.DashboardPnL{
		Revenue:      fmt.Sprintf("%.2f", totalRev),
		COGS:         fmt.Sprintf("%.2f", totalCOGS),
		GrossProfit:  fmt.Sprintf("%.2f", gp),
		GrossMargin:  fmt.Sprintf("%.1f", pct(gp, totalRev)),
		MonthLabels:  monthLabels,
		MonthRevenue: monthRev,
		MonthCOGS:    monthCOGS,
		MonthProfit:  monthProfit,
	}

	curRev := toFloat(curInv.Revenue)
	prevRev := toFloat(prevInv.Revenue)
	curProfit := curRev - curPurch
	prevProfit := prevRev - prevPurch

	// VAT quarterly
	vatQuarterly := make([]model.DashboardVATQuarter, 0, len(vatQRows))
	for _, r := range vatQRows {
		out := toFloat(r.OutputVat)
		in := toFloat(r.InputVat)
		vatQuarterly = append(vatQuarterly, model.DashboardVATQuarter{
			Quarter:   fmt.Sprintf("Q%d/%d", toInt(r.Quarter), toInt(r.Year)),
			OutputVAT: fmt.Sprintf("%.2f", out),
			InputVAT:  fmt.Sprintf("%.2f", in),
			NetVAT:    fmt.Sprintf("%.2f", out-in),
		})
	}

	// ── Balance Sheet (account.type → grouped subtype amounts) ────
	var totalAssets, totalLiab, totalEquity float64
	subtypeMaps := map[string]map[string]float64{
		"asset":     {},
		"liability": {},
		"equity":    {},
	}
	for _, r := range bsRows {
		t := toString(r.AcctType)
		st := toString(r.AcctSubtype)
		amt := toFloat(r.NetBalance)
		// Liabilities & Equity carry credit balances → flip sign so they're positive.
		switch t {
		case "asset":
			totalAssets += amt
		case "liability":
			amt = -amt
			totalLiab += amt
		case "equity":
			amt = -amt
			totalEquity += amt
		default:
			continue
		}
		subtypeMaps[t][st] += amt
	}
	groupOf := func(m map[string]float64) []model.DashboardAccountGroup {
		out := make([]model.DashboardAccountGroup, 0, len(m))
		for k, v := range m {
			out = append(out, model.DashboardAccountGroup{
				Subtype: k,
				Amount:  fmt.Sprintf("%.2f", v),
			})
		}
		return out
	}
	balanceSheet := model.DashboardBalanceSheet{
		AsOf:             periodEnd.Format("2006-01-02"),
		TotalAssets:      fmt.Sprintf("%.2f", totalAssets),
		TotalLiabilities: fmt.Sprintf("%.2f", totalLiab),
		TotalEquity:      fmt.Sprintf("%.2f", totalEquity),
		NetWorth:         fmt.Sprintf("%.2f", totalAssets-totalLiab),
		Assets:           groupOf(subtypeMaps["asset"]),
		Liabilities:      groupOf(subtypeMaps["liability"]),
		Equity:           groupOf(subtypeMaps["equity"]),
	}

	// ── Operating Expenses ────────────────────────────────────────
	totalOpEx := toFloat(opexSum.TotalOpex)
	netIncome := curRev - curPurch - totalOpEx
	opexRatio := pct(totalOpEx, curRev)
	opexCatsOut := make([]model.DashboardExpenseCategory, 0, len(opexCats))
	for _, r := range opexCats {
		opexCatsOut = append(opexCatsOut, model.DashboardExpenseCategory{
			CategoryID:   toInt(r.CategoryID),
			Code:         r.CategoryCode,
			Name:         r.CategoryName,
			TotalAmount:  fmt.Sprintf("%.2f", toFloat(r.TotalAmount)),
			ExpenseCount: toInt64(r.ExpenseCount),
		})
	}
	opex := model.DashboardOpEx{
		StartDate:    periodStart.Format("2006-01-02"),
		EndDate:      periodEnd.Format("2006-01-02"),
		TotalOpEx:    fmt.Sprintf("%.2f", totalOpEx),
		OpExVAT:      fmt.Sprintf("%.2f", toFloat(opexSum.OpexVat)),
		ExpenseCount: toInt64(opexSum.ExpenseCount),
		NetIncome:    fmt.Sprintf("%.2f", netIncome),
		OpExRatio:    fmt.Sprintf("%.1f", opexRatio),
		ByCategory:   opexCatsOut,
	}

	// ── ZATCA ─────────────────────────────────────────────────────
	totalSubs := toInt64(zatca.TotalSubmissions)
	acceptanceRate := 0.0
	if totalSubs > 0 {
		acceptanceRate = float64(toInt64(zatca.AcceptedCount)) * 100 / float64(totalSubs)
	}
	zatcaOut := model.DashboardZATCAStats{
		TotalSubmissions:    totalSubs,
		PendingCount:        toInt64(zatca.PendingCount),
		SubmittedCount:      toInt64(zatca.SubmittedCount),
		AcceptedCount:       toInt64(zatca.AcceptedCount),
		RejectedCount:       toInt64(zatca.RejectedCount),
		WarningCount:        toInt64(zatca.WarningCount),
		AcceptanceRate:      fmt.Sprintf("%.1f", acceptanceRate),
		AvgRetries:          fmt.Sprintf("%.2f", toFloat(zatca.AvgRetries)),
		AvgClearanceSeconds: fmt.Sprintf("%.1f", toFloat(zatca.AvgClearanceSeconds)),
	}

	// ── Payment Tracking ──────────────────────────────────────────
	arOut := toFloat(payTrack.ArOutstandingTotal)
	apOut := toFloat(payTrack.ApOutstandingTotal)
	payRecv := toFloat(payTrack.PaymentsReceived)
	payMade := toFloat(payTrack.PaymentsMade)
	paymentTracking := model.DashboardPaymentTracking{
		AROutstandingCount: toInt64(payTrack.ArOutstandingCount),
		AROutstandingTotal: fmt.Sprintf("%.2f", arOut),
		APOutstandingCount: toInt64(payTrack.ApOutstandingCount),
		APOutstandingTotal: fmt.Sprintf("%.2f", apOut),
		PaymentsReceived:   fmt.Sprintf("%.2f", payRecv),
		PaymentsMade:       fmt.Sprintf("%.2f", payMade),
		NetCashPosition:    fmt.Sprintf("%.2f", arOut-apOut),
	}

	// ── Liquidity (derived from balance sheet subtypes) ──────────
	curAssets := subtypeMaps["asset"]["current_asset"] +
		subtypeMaps["asset"]["cash"] +
		subtypeMaps["asset"]["bank"] +
		subtypeMaps["asset"]["accounts_receivable"]
	inventory := subtypeMaps["asset"]["inventory"]
	if curAssets == 0 {
		// fallback: anything not fixed_asset is treated as current
		for k, v := range subtypeMaps["asset"] {
			if k != "fixed_asset" {
				curAssets += v
			}
		}
	}
	curLiab := subtypeMaps["liability"]["current_liability"] +
		subtypeMaps["liability"]["accounts_payable"] +
		subtypeMaps["liability"]["short_term_debt"]
	if curLiab == 0 {
		for k, v := range subtypeMaps["liability"] {
			if k != "long_term_debt" {
				curLiab += v
			}
		}
	}
	curRatio, quickRatio, debtToEq := 0.0, 0.0, 0.0
	if curLiab > 0 {
		curRatio = curAssets / curLiab
		quickRatio = (curAssets - inventory) / curLiab
	}
	if totalEquity > 0 {
		debtToEq = totalLiab / totalEquity
	}
	liquidity := model.DashboardLiquidity{
		CurrentAssets:      fmt.Sprintf("%.2f", curAssets),
		CurrentLiabilities: fmt.Sprintf("%.2f", curLiab),
		Inventory:          fmt.Sprintf("%.2f", inventory),
		CurrentRatio:       fmt.Sprintf("%.2f", curRatio),
		QuickRatio:         fmt.Sprintf("%.2f", quickRatio),
		DebtToEquity:       fmt.Sprintf("%.2f", debtToEq),
	}

	c.JSON(http.StatusOK, model.DashboardAnalyticsResponse{
		ARAging:  arBuckets,
		APAging:  apBuckets,
		CashFlow: cashFlow,
		PnL:      pnl,
		KPITrends: model.DashboardKPITrends{
			Invoices:       makeTrend(float64(toInt64(curInv.InvoiceCount)), float64(toInt64(prevInv.InvoiceCount))),
			Revenue:        makeTrend(curRev, prevRev),
			PurchasesTotal: makeTrend(curPurch, prevPurch),
			GrossProfit:    makeTrend(curProfit, prevProfit),
		},
		VATQuarterly:    vatQuarterly,
		BalanceSheet:    balanceSheet,
		OpEx:            opex,
		ZATCA:           zatcaOut,
		PaymentTracking: paymentTracking,
		Liquidity:       liquidity,
	})
}

// ── GET /api/v2/dashboard/compare ───────────────────────────────────────────

func (h *handler) GetDashboardCompare(c *gin.Context) {
	aStart := c.Query("a_start")
	aEnd := c.Query("a_end")
	bStart := c.Query("b_start")
	bEnd := c.Query("b_end")
	if aStart == "" || aEnd == "" || bStart == "" || bEnd == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "يرجى تحديد فترتين للمقارنة"})
		return
	}

	aStartT, errAS := time.Parse("2006-01-02", aStart)
	aEndT, errAE := time.Parse("2006-01-02", aEnd)
	bStartT, errBS := time.Parse("2006-01-02", bStart)
	bEndT, errBE := time.Parse("2006-01-02", bEnd)
	if errAS != nil || errAE != nil || errBS != nil || errBE != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "تواريخ غير صحيحة (YYYY-MM-DD)"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	q := h.queries

	type periodOut struct {
		Stats model.DashboardComparePeriod
		Err   error
	}
	compute := func(start, end time.Time) periodOut {
		invRow, errA := q.GetDashboardPeriodInvoices(ctx, db.GetDashboardPeriodInvoicesParams{
			StartDate: start, EndDate: end,
		})
		purSum, errB := q.GetDashboardPeriodPurchases(ctx, db.GetDashboardPeriodPurchasesParams{
			StartDate: start, EndDate: end,
		})
		if errA != nil && !errors.Is(errA, sql.ErrNoRows) {
			return periodOut{Err: errA}
		}
		if errB != nil && !errors.Is(errB, sql.ErrNoRows) {
			return periodOut{Err: errB}
		}

		revenue := toFloat(invRow.Revenue)
		purchases := toFloat(purSum)
		pending := toFloat(invRow.PendingAmount)
		profit := revenue - purchases
		invCount := toInt64(invRow.InvoiceCount)
		avg := 0.0
		if invCount > 0 {
			avg = revenue / float64(invCount)
		}
		return periodOut{Stats: model.DashboardComparePeriod{
			Invoices:   invCount,
			Revenue:    fmt.Sprintf("%.2f", revenue),
			Purchases:  fmt.Sprintf("%.2f", purchases),
			Profit:     fmt.Sprintf("%.2f", profit),
			AvgInvoice: fmt.Sprintf("%.2f", avg),
			Pending:    fmt.Sprintf("%.2f", pending),
			Margin:     fmt.Sprintf("%.1f", pct(profit, revenue)),
			Issued:     toInt64(invRow.IssuedCount),
			Draft:      toInt64(invRow.DraftCount),
		}}
	}

	var (
		wg   sync.WaitGroup
		a, b periodOut
	)
	wg.Add(2)
	go func() { defer wg.Done(); a = compute(aStartT, aEndT) }()
	go func() { defer wg.Done(); b = compute(bStartT, bEndT) }()
	wg.Wait()

	if a.Err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": a.Err.Error()})
		return
	}
	if b.Err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": b.Err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.DashboardCompareResponse{
		PeriodA: a.Stats,
		PeriodB: b.Stats,
	})
}

// ── helpers ────────────────────────────────────────────────────────────────

func buildMonthLabels(now time.Time, n int) []string {
	out := make([]string, n)
	for i := n - 1; i >= 0; i-- {
		out[n-1-i] = now.AddDate(0, -i, 0).Format("01/2006")
	}
	return out
}

// optInt32 returns nil for empty / invalid input so sqlc's pointer-typed
// narg params (e.g. *int32) can be passed directly.
func optInt32(s string) *int32 {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	n := int32(v)
	return &n
}

// optDate parses "YYYY-MM-DD" and returns a *time.Time anchored at start or
// end of day. Returns nil for empty / unparsable input.
func optDate(s string, endOfDay bool) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	if endOfDay {
		t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}
	return &t
}

// toFloat handles whatever sqlc generates for DECIMAL / SUM / AVG columns:
// float64, string, []byte, nil, or any int. SUM(DECIMAL) is usually `string`.
func toFloat(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return x
	case float32:
		return float64(x)
	case int64:
		return float64(x)
	case int32:
		return float64(x)
	case int:
		return float64(x)
	case uint64:
		return float64(x)
	case uint32:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case []byte:
		f, _ := strconv.ParseFloat(string(x), 64)
		return f
	case *float64:
		if x == nil {
			return 0
		}
		return *x
	case *int64:
		if x == nil {
			return 0
		}
		return float64(*x)
	case *int32:
		if x == nil {
			return 0
		}
		return float64(*x)
	case *string:
		if x == nil {
			return 0
		}
		f, _ := strconv.ParseFloat(*x, 64)
		return f
	}
	return 0
}

// toInt64 / toInt accept whatever sqlc generates for COUNT, plain INTs, etc.
func toInt64(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	case uint64:
		return int64(x)
	case uint32:
		return int64(x)
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	case []byte:
		n, _ := strconv.ParseInt(string(x), 10, 64)
		return n
	case *int64:
		if x == nil {
			return 0
		}
		return *x
	case *int32:
		if x == nil {
			return 0
		}
		return int64(*x)
	}
	return 0
}

func toInt(v any) int { return int(toInt64(v)) }
func toBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int64:
		return x != 0
	case int32:
		return x != 0
	case int:
		return x != 0
	case []byte:
		return len(x) > 0 && x[0] != '0'
	case string:
		return x != "" && x != "0"
	}
	return false
}

func toString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case time.Time:
		if x.IsZero() {
			return ""
		}
		return x.Format("2006-01-02")
	case *time.Time:
		if x == nil || x.IsZero() {
			return ""
		}
		return x.Format("2006-01-02")
	}
	return fmt.Sprintf("%v", v)
}

// derefStr returns the dereferenced *string or nil for JSON encoding.
func derefStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func pct(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return (a / b) * 100
}

func stateLabel(s int) string {
	switch s {
	case 0:
		return "مسودة"
	case 1:
		return "قيد المعالجة"
	case 2:
		return "تمت المعالجة"
	case 3:
		return "صادرة"
	default:
		return "غير معروف"
	}
}

func makeTrend(cur, prev float64) model.DashboardTrend {
	if prev == 0 && cur == 0 {
		return model.DashboardTrend{Direction: "flat", Percent: "0", Arrow: "—"}
	}
	if prev == 0 {
		return model.DashboardTrend{Direction: "up", Percent: "100", Arrow: "↑"}
	}
	p := ((cur - prev) / math.Abs(prev)) * 100
	dir, arrow := "flat", "—"
	if p > 0.5 {
		dir, arrow = "up", "↑"
	} else if p < -0.5 {
		dir, arrow = "down", "↓"
	}
	return model.DashboardTrend{Direction: dir, Percent: fmt.Sprintf("%.1f", math.Abs(p)), Arrow: arrow}
}

// concentrationRisk returns the % of total value held by the topN entries.
// Input slice MUST already be sorted descending (the CLV query orders by
// total_value DESC, so we just sum the first topN entries against the total).
func concentrationRisk(values []float64, topN int) float64 {
	if len(values) == 0 || topN <= 0 {
		return 0
	}
	var total, top float64
	for i, v := range values {
		total += v
		if i < topN {
			top += v
		}
	}
	if total == 0 {
		return 0
	}
	return (top / total) * 100
}
