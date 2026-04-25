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
	ARAging         []DashboardAgingBucket   `json:"ar_aging"`
	APAging         []DashboardAgingBucket   `json:"ap_aging"`
	CashFlow        []DashboardCashFlowMonth `json:"cash_flow"`
	PnL             DashboardPnL             `json:"pnl"`
	KPITrends       DashboardKPITrends       `json:"kpi_trends"`
	VATQuarterly    []DashboardVATQuarter    `json:"vat_quarterly"`
	BalanceSheet    DashboardBalanceSheet    `json:"balance_sheet"`
	OpEx            DashboardOpEx            `json:"opex"`
	ZATCA           DashboardZATCAStats      `json:"zatca"`
	PaymentTracking DashboardPaymentTracking `json:"payment_tracking"`
	Liquidity       DashboardLiquidity       `json:"liquidity"`
}

// Balance Sheet — totals plus subtype breakdown for both Assets and Liabilities.
type DashboardBalanceSheet struct {
	AsOf             string                  `json:"as_of"`
	TotalAssets      string                  `json:"total_assets"`
	TotalLiabilities string                  `json:"total_liabilities"`
	TotalEquity      string                  `json:"total_equity"`
	NetWorth         string                  `json:"net_worth"`
	Assets           []DashboardAccountGroup `json:"assets"`
	Liabilities      []DashboardAccountGroup `json:"liabilities"`
	Equity           []DashboardAccountGroup `json:"equity"`
}

type DashboardAccountGroup struct {
	Subtype string `json:"subtype"`
	Amount  string `json:"amount"`
}

// Operating Expenses — period totals + per-category breakdown + derived ratios.
type DashboardOpEx struct {
	StartDate    string                     `json:"start_date"`
	EndDate      string                     `json:"end_date"`
	TotalOpEx    string                     `json:"total_opex"`
	OpExVAT      string                     `json:"opex_vat"`
	ExpenseCount int64                      `json:"expense_count"`
	NetIncome    string                     `json:"net_income"`
	OpExRatio    string                     `json:"opex_ratio"`
	ByCategory   []DashboardExpenseCategory `json:"by_category"`
}

type DashboardExpenseCategory struct {
	CategoryID   int    `json:"category_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	TotalAmount  string `json:"total_amount"`
	ExpenseCount int64  `json:"expense_count"`
}

// ZATCA submission stats.
type DashboardZATCAStats struct {
	TotalSubmissions    int64  `json:"total_submissions"`
	PendingCount        int64  `json:"pending_count"`
	SubmittedCount      int64  `json:"submitted_count"`
	AcceptedCount       int64  `json:"accepted_count"`
	RejectedCount       int64  `json:"rejected_count"`
	WarningCount        int64  `json:"warning_count"`
	AcceptanceRate      string `json:"acceptance_rate"`
	AvgRetries          string `json:"avg_retries"`
	AvgClearanceSeconds string `json:"avg_clearance_seconds"`
}

// Payment tracking from real bill_payment / purchase_bill_payment tables.
type DashboardPaymentTracking struct {
	AROutstandingCount int64  `json:"ar_outstanding_count"`
	AROutstandingTotal string `json:"ar_outstanding_total"`
	APOutstandingCount int64  `json:"ap_outstanding_count"`
	APOutstandingTotal string `json:"ap_outstanding_total"`
	PaymentsReceived   string `json:"payments_received"`
	PaymentsMade       string `json:"payments_made"`
	NetCashPosition    string `json:"net_cash_position"`
}

// Liquidity ratios derived from the Balance Sheet subtypes.
type DashboardLiquidity struct {
	CurrentAssets      string `json:"current_assets"`
	CurrentLiabilities string `json:"current_liabilities"`
	Inventory          string `json:"inventory"`
	CurrentRatio       string `json:"current_ratio"`
	QuickRatio         string `json:"quick_ratio"`
	DebtToEquity       string `json:"debt_to_equity"`
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
