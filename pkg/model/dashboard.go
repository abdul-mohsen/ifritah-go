// Package model — typed response structs for the dashboard endpoints.
//
// File destination in backend repo: pkg/model/dashboard.go
//
// The JSON tags here match exactly the JSON keys used by the original
// pkg/handlers/dashboard.go in ifritah-go (dev branch), so the frontend
// keeps working unchanged.
//
// All currency / percentage fields are emitted as STRINGS pre-formatted
// (e.g. "1234.56", "12.5") to match the existing wire format.

package model

// ── /api/v2/dashboard ───────────────────────────────────────────────────────

type DashboardResponse struct {
	Stats              DashboardStats        `json:"stats"`
	StatusCounts       map[string]int64      `json:"status_counts"`
	Charts             DashboardCharts       `json:"charts"`
	RecentInvoices     []DashboardInvoice    `json:"recent_invoices"`
	LowStockProducts   []DashboardLowStock   `json:"low_stock_products"`
	TopProducts        []DashboardTopProduct `json:"top_products"`
	MarginTiers        []DashboardTier       `json:"margin_tiers"`
	SupplierPerf       []DashboardSupplier   `json:"supplier_perf"`
	TopClients         []DashboardTopClient  `json:"top_clients"`
	TopCLV             []DashboardCLVClient  `json:"top_clv"`
	ClientDistribution DashboardClientDist   `json:"client_distribution"`
	Filters            DashboardFilters      `json:"filters"`
}

type DashboardCLVClient struct {
	ClientID   int    `json:"client_id"`
	Name       string `json:"name"`
	OrderCount int    `json:"order_count"`
	Value      string `json:"value"`
}

type DashboardStats struct {
	TotalInvoices       int64  `json:"total_invoices"`
	TotalRevenue        string `json:"total_revenue"`
	TotalVAT            string `json:"total_vat"`
	TotalDiscount       string `json:"total_discount"`
	TotalProducts       int64  `json:"total_products"`
	TotalClients        int64  `json:"total_clients"`
	TotalSuppliers      int64  `json:"total_suppliers"`
	TotalStores         int64  `json:"total_stores"`
	TotalBranches       int64  `json:"total_branches"`
	PendingInvoices     int64  `json:"pending_invoices"`
	PendingAmount       string `json:"pending_amount"`
	TotalPurchases      string `json:"total_purchases"`
	TotalPurchaseBills  int64  `json:"total_purchase_bills"`
	TotalPurchaseVAT    string `json:"total_purchase_vat"`
	GrossProfit         string `json:"gross_profit"`
	GrossMargin         string `json:"gross_margin"`
	AvgInvoiceValue     string `json:"avg_invoice_value"`
	LowStockCount       int64  `json:"low_stock_count"`
	CreditNoteCount     int64  `json:"credit_note_count"`
	CreditNoteTotal     string `json:"credit_note_total"`
	InvTurnover         string `json:"inv_turnover"`
	FulfillmentRate     string `json:"fulfillment_rate"`
	ReturnRate          string `json:"return_rate"`
	TotalOrders         int64  `json:"total_orders"`
	PendingOrders       int64  `json:"pending_orders"`
	CompletedOrders     int64  `json:"completed_orders"`
	CancelledOrders     int64  `json:"cancelled_orders"`
	TotalOrdersAmount   string `json:"total_orders_amount"`
	AvgProcessingDays   string `json:"avg_processing_days"`
	ClientConcentration string `json:"client_concentration"`
}

type DashboardCharts struct {
	MonthLabels      []string                  `json:"month_labels"`
	MonthlyRevenue   []string                  `json:"monthly_revenue"`
	MonthlyPurchases []string                  `json:"monthly_purchases"`
	MonthlyProfit    []string                  `json:"monthly_profit"`
	YoYRevenue       []string                  `json:"yoy_revenue"`
	WeekdayRevenue   []DashboardWeekdayRevenue `json:"weekday_revenue"`
	MonthlyReturns   []DashboardMonthlyReturn  `json:"monthly_returns"`
}

type DashboardInvoice struct {
	ID              int    `json:"id"`
	SequenceNumber  int64  `json:"sequence_number"`
	Total           string `json:"total"`
	State           int    `json:"state"`
	StateLabel      string `json:"state_label"`
	Date            string `json:"date"`
	IsCreditNote    bool   `json:"is_credit_note"`
	UserPhoneNumber any    `json:"user_phone_number"`
}

type DashboardLowStock struct {
	ID            int    `json:"id"`
	ArticleNumber string `json:"article_number"`
	Price         string `json:"price"`
	Quantity      string `json:"quantity"`
	CostPrice     string `json:"cost_price"`
	MinStock      int    `json:"min_stock"`
	StoreID       int    `json:"store_id"`
}

type DashboardTopProduct struct {
	ID            int    `json:"id"`
	ArticleNumber string `json:"article_number"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price"`
}

type DashboardTier struct {
	Label    string `json:"label"`
	Count    int    `json:"count"`
	AvgPrice string `json:"avg_price"`
}

type DashboardSupplier struct {
	ID         int     `json:"id"`
	Name       *string `json:"name"`
	BillCount  int     `json:"bill_count"`
	TotalSpent string  `json:"total_spent"`
	AvgTotal   string  `json:"avg_total"`
}

type DashboardTopClient struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	InvoiceCount int    `json:"invoice_count"`
	Total        string `json:"total"`
	LastInvoice  string `json:"last_invoice"`
}

type DashboardWeekdayRevenue struct {
	Day        int    `json:"day"`
	DayName    string `json:"day_name"`
	AvgRevenue string `json:"avg_revenue"`
}

type DashboardMonthlyReturn struct {
	Month       string `json:"month"`
	Invoices    int    `json:"invoices"`
	CreditNotes int    `json:"credit_notes"`
	Rate        string `json:"rate"`
}

type DashboardClientDist struct {
	Active   int64 `json:"active"`
	Inactive int64 `json:"inactive"`
}

type DashboardFilters struct {
	State     string `json:"state"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Months    int    `json:"months"`
}

// ── /api/v2/dashboard/analytics ─────────────────────────────────────────────

type DashboardAnalyticsResponse struct {
	ARAging      []DashboardAgingBucket   `json:"ar_aging"`
	APAging      []DashboardAgingBucket   `json:"ap_aging"`
	CashFlow     []DashboardCashFlowMonth `json:"cash_flow"`
	PnL          DashboardPnL             `json:"pnl"`
	KPITrends    DashboardKPITrends       `json:"kpi_trends"`
	VATQuarterly []DashboardVATQuarter    `json:"vat_quarterly"`
}

type DashboardVATQuarter struct {
	Quarter   string `json:"quarter"`
	OutputVAT string `json:"output_vat"`
	InputVAT  string `json:"input_vat"`
	NetVAT    string `json:"net_vat"`
}

type DashboardAgingBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
	Total string `json:"total"`
}

type DashboardCashFlowMonth struct {
	Month   string `json:"month"`
	Inflow  string `json:"inflow"`
	Outflow string `json:"outflow"`
	Net     string `json:"net"`
}

type DashboardPnL struct {
	Revenue      string   `json:"revenue"`
	COGS         string   `json:"cogs"`
	GrossProfit  string   `json:"gross_profit"`
	GrossMargin  string   `json:"gross_margin"`
	MonthLabels  []string `json:"month_labels"`
	MonthRevenue []string `json:"month_revenue"`
	MonthCOGS    []string `json:"month_cogs"`
	MonthProfit  []string `json:"month_profit"`
}

type DashboardKPITrends struct {
	Invoices       DashboardTrend `json:"invoices"`
	Revenue        DashboardTrend `json:"revenue"`
	PurchasesTotal DashboardTrend `json:"purchases_total"`
	GrossProfit    DashboardTrend `json:"gross_profit"`
}

type DashboardTrend struct {
	Direction string `json:"direction"` // "up" | "down" | "flat"
	Percent   string `json:"percent"`
	Arrow     string `json:"arrow"` // "↑" | "↓" | "—"
}

// ── /api/v2/dashboard/compare ───────────────────────────────────────────────

type DashboardCompareResponse struct {
	PeriodA DashboardComparePeriod `json:"period_a"`
	PeriodB DashboardComparePeriod `json:"period_b"`
}

type DashboardComparePeriod struct {
	Invoices   int64  `json:"invoices"`
	Revenue    string `json:"revenue"`
	Purchases  string `json:"purchases"`
	Profit     string `json:"profit"`
	AvgInvoice string `json:"avg_invoice"`
	Pending    string `json:"pending"`
	Margin     string `json:"margin"`
	Issued     int64  `json:"issued"`
	Draft      int64  `json:"draft"`
}
