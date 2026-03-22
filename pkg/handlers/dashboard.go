package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// pkg/handlers/dashboard.go
func (h *handler) GetDashboard(c *gin.Context) {
	user := GetSessionInfo(c)

	storeIds := h.getStoreIds(c)
	if len(storeIds) == 0 {
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	// Build placeholders for IN clause
	placeholders := strings.Repeat("?,", len(storeIds)-1) + "?"
	args := make([]interface{}, len(storeIds))
	for i, id := range storeIds {
		args[i] = id
	}

	type DashboardResponse struct {
		TotalInvoices   int            `json:"total_invoices"`
		TotalRevenue    string         `json:"total_revenue"`
		TotalProducts   int            `json:"total_products"`
		TotalClients    int            `json:"total_clients"`
		TotalSuppliers  int            `json:"total_suppliers"`
		PendingInvoices int            `json:"pending_invoices"`
		PendingAmount   string         `json:"pending_amount"`
		MonthlyRevenue  []MonthRevenue `json:"monthly_revenue"`
		StatusCounts    StatusCount    `json:"status_counts"`
		RecentInvoices  []RecentInv    `json:"recent_invoices"`
		TopProducts     []TopProduct   `json:"top_products"`
	}

	var resp DashboardResponse

	// 1. Invoice stats (single query using bill_totals view)
	h.DB.QueryRow(fmt.Sprintf(`
        SELECT 
            COUNT(*),
            COALESCE(ROUND(SUM(total), 2), 0),
            COALESCE(SUM(CASE WHEN state = 0 THEN 1 ELSE 0 END), 0),
            COALESCE(ROUND(SUM(CASE WHEN state = 0 THEN total ELSE 0 END), 2), 0)
        FROM bill_totals 
        WHERE store_id IN (%s) AND state >= 0
    `, placeholders), args...).Scan(
		&resp.TotalInvoices, &resp.TotalRevenue,
		&resp.PendingInvoices, &resp.PendingAmount,
	)

	// 2. Product count
	h.DB.QueryRow(fmt.Sprintf(`
        SELECT COUNT(*) FROM product WHERE store_id IN (%s) AND is_deleted = 0
    `, placeholders), args...).Scan(&resp.TotalProducts)

	// 3. Client count (company-wide)
	h.DB.QueryRow(`SELECT COUNT(*) FROM client WHERE is_deleted = 0`).Scan(&resp.TotalClients)

	// 4. Supplier count
	companyArgs := []interface{}{user.id}
	h.DB.QueryRow(`
        SELECT COUNT(*) FROM supplier s 
        JOIN company c ON s.company_id = c.id 
        JOIN user u ON u.company_id = c.id AND u.id = ?
        WHERE s.is_deleted = 0
    `, companyArgs...).Scan(&resp.TotalSuppliers)

	// 5. Monthly revenue (last 6 months)
	rows, _ := h.DB.Query(fmt.Sprintf(`
        SELECT 
            DATE_FORMAT(effective_date, '%%Y-%%m') as month,
            ROUND(SUM(total), 2) as revenue
        FROM bill_totals
        WHERE store_id IN (%s) AND state > 0
            AND effective_date >= DATE_SUB(NOW(), INTERVAL 6 MONTH)
        GROUP BY month ORDER BY month
    `, placeholders), args...)
	defer rows.Close()
	for rows.Next() {
		var mr MonthRevenue
		rows.Scan(&mr.Month, &mr.Revenue)
		resp.MonthlyRevenue = append(resp.MonthlyRevenue, mr)
	}

	// 6. Status counts
	h.DB.QueryRow(fmt.Sprintf(`
        SELECT 
            SUM(CASE WHEN state = 0 THEN 1 ELSE 0 END),
            SUM(CASE WHEN state = 1 THEN 1 ELSE 0 END),
            SUM(CASE WHEN state = 2 THEN 1 ELSE 0 END),
            SUM(CASE WHEN state = 3 THEN 1 ELSE 0 END)
        FROM bill WHERE store_id IN (%s) AND state >= 0
    `, placeholders), args...).Scan(
		&resp.StatusCounts.Draft, &resp.StatusCounts.Processing,
		&resp.StatusCounts.Processed, &resp.StatusCounts.Issued,
	)

	// 7. Recent 5 invoices
	recentRows, _ := h.DB.Query(fmt.Sprintf(`
        SELECT id, sequence_number, ROUND(total, 2), state, effective_date, userName
        FROM bill_totals 
        WHERE store_id IN (%s) AND state >= 0
        ORDER BY effective_date DESC LIMIT 5
    `, placeholders), args...)
	defer recentRows.Close()
	for recentRows.Next() {
		var ri RecentInv
		recentRows.Scan(&ri.ID, &ri.SeqNum, &ri.Total, &ri.State, &ri.Date, &ri.UserName)
		resp.RecentInvoices = append(resp.RecentInvoices, ri)
	}

	c.JSON(http.StatusOK, resp)
}

type MonthRevenue struct {
	Month   string `json:"month"`
	Revenue string `json:"revenue"`
}

type StatusCount struct {
	Draft      int `json:"draft"`
	Processing int `json:"processing"`
	Processed  int `json:"processed"`
	Issued     int `json:"issued"`
}

type RecentInv struct {
	ID       int32     `json:"id"`
	SeqNum   int32     `json:"sequence_number"`
	Total    string    `json:"total"`
	State    int32     `json:"state"`
	Date     time.Time `json:"date"`
	UserName *string   `json:"user_name"`
}

type TopProduct struct {
	Name     string `json:"name"`
	Quantity string `json:"total_quantity"`
	Revenue  string `json:"total_revenue"`
}
