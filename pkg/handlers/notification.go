package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Notification Settings (per user)
// ============================================================================

// GetNotificationConfig returns the notification preferences for the current user.
// GET /api/v2/notification/config
//
// Response:
//
//	{
//	  "data": {
//	    "low_stock_alert": true,
//	    "low_stock_threshold": 5,
//	    "pending_invoice_days": 7,
//	    "new_order_alert": true,
//	    "payment_due_alert": true,
//	    "daily_summary": false,
//	    "email_enabled": false
//	  }
//	}
func (h *handler) GetNotificationConfig(c *gin.Context) {
	userID := c.GetInt64("userId")

	var config struct {
		LowStockAlert     bool `json:"low_stock_alert"`
		LowStockThreshold int  `json:"low_stock_threshold"`
		PendingDays       int  `json:"pending_invoice_days"`
		NewOrderAlert     bool `json:"new_order_alert"`
		PaymentDueAlert   bool `json:"payment_due_alert"`
		DailySummary      bool `json:"daily_summary"`
		EmailEnabled      bool `json:"email_enabled"`
	}

	err := h.DB.QueryRow(
		`SELECT low_stock_alert, low_stock_threshold, pending_invoice_days,
		        new_order_alert, payment_due_alert, daily_summary, email_enabled
		 FROM notification_settings WHERE user_id = ?`,
		userID,
	).Scan(&config.LowStockAlert, &config.LowStockThreshold, &config.PendingDays,
		&config.NewOrderAlert, &config.PaymentDueAlert, &config.DailySummary, &config.EmailEnabled)

	if err == sql.ErrNoRows {
		// Return defaults
		config.LowStockAlert = true
		config.LowStockThreshold = 5
		config.PendingDays = 7
		config.NewOrderAlert = true
		config.PaymentDueAlert = true
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

// UpdateNotificationConfig saves notification preferences for the current user.
// PUT /api/v2/notification/config
//
// Request:
//
//	{
//	  "low_stock_alert": true,
//	  "low_stock_threshold": 10,
//	  "pending_invoice_days": 14,
//	  "new_order_alert": false,
//	  "payment_due_alert": true,
//	  "daily_summary": true,
//	  "email_enabled": false
//	}
//
// Response: {"detail": "success"}
func (h *handler) UpdateNotificationConfig(c *gin.Context) {
	userID := c.GetInt64("userId")

	var req struct {
		LowStockAlert     *bool `json:"low_stock_alert"`
		LowStockThreshold *int  `json:"low_stock_threshold"`
		PendingDays       *int  `json:"pending_invoice_days"`
		NewOrderAlert     *bool `json:"new_order_alert"`
		PaymentDueAlert   *bool `json:"payment_due_alert"`
		DailySummary      *bool `json:"daily_summary"`
		EmailEnabled      *bool `json:"email_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	// Upsert: insert if not exists, update if exists
	_, err := h.DB.Exec(
		`INSERT INTO notification_settings
		 (user_id, low_stock_alert, low_stock_threshold, pending_invoice_days, new_order_alert, payment_due_alert, daily_summary, email_enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   low_stock_alert = COALESCE(VALUES(low_stock_alert), low_stock_alert),
		   low_stock_threshold = COALESCE(VALUES(low_stock_threshold), low_stock_threshold),
		   pending_invoice_days = COALESCE(VALUES(pending_invoice_days), pending_invoice_days),
		   new_order_alert = COALESCE(VALUES(new_order_alert), new_order_alert),
		   payment_due_alert = COALESCE(VALUES(payment_due_alert), payment_due_alert),
		   daily_summary = COALESCE(VALUES(daily_summary), daily_summary),
		   email_enabled = COALESCE(VALUES(email_enabled), email_enabled)`,
		userID,
		boolDefault(req.LowStockAlert, true),
		intDefault(req.LowStockThreshold, 5),
		intDefault(req.PendingDays, 7),
		boolDefault(req.NewOrderAlert, true),
		boolDefault(req.PaymentDueAlert, true),
		boolDefault(req.DailySummary, false),
		boolDefault(req.EmailEnabled, false),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to save notification config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

// ============================================================================
// Notifications List (system-generated alerts)
// ============================================================================

// GetNotifications returns unread notifications for the current user.
// GET /api/v2/notification
//
// Response:
//
//	{
//	  "data": [
//	    {"id":1, "type":"low_stock", "title":"مخزون منخفض", "message":"فلتر زيت - الكمية: 2", "read":false, "created_at":"..."},
//	    ...
//	  ]
//	}
func (h *handler) GetNotifications(c *gin.Context) {
	userID := c.GetInt64("userId")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := h.DB.Query(
		`SELECT id, type, title, message, is_read, created_at
		 FROM notifications
		 WHERE user_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch notifications"})
		return
	}
	defer rows.Close()

	type Notification struct {
		ID        int64  `json:"id"`
		Type      string `json:"type"`
		Title     string `json:"title"`
		Message   string `json:"message"`
		IsRead    bool   `json:"read"`
		CreatedAt string `json:"created_at"`
	}

	var notifs []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Message, &n.IsRead, &n.CreatedAt); err != nil {
			continue
		}
		notifs = append(notifs, n)
	}
	if notifs == nil {
		notifs = []Notification{}
	}

	c.JSON(http.StatusOK, gin.H{"data": notifs})
}

// MarkNotificationRead marks a notification as read.
// PUT /api/v2/notification/:id/read
//
// Response: {"detail": "success"}
func (h *handler) MarkNotificationRead(c *gin.Context) {
	userID := c.GetInt64("userId")
	notifID := c.Param("id")

	_, err := h.DB.Exec(
		"UPDATE notifications SET is_read = 1 WHERE id = ? AND user_id = ?",
		notifID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

// MarkAllNotificationsRead marks all notifications as read for the current user.
// PUT /api/v2/notification/read-all
//
// Response: {"detail": "success"}
func (h *handler) MarkAllNotificationsRead(c *gin.Context) {
	userID := c.GetInt64("userId")

	_, _ = h.DB.Exec("UPDATE notifications SET is_read = 1 WHERE user_id = ?", userID)

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

// ============================================================================
// Helpers
// ============================================================================

func boolDefault(ptr *bool, def bool) bool {
	if ptr != nil {
		return *ptr
	}
	return def
}

func intDefault(ptr *int, def int) int {
	if ptr != nil {
		return *ptr
	}
	return def
}

// ============================================================================
// Low Stock Check — call this after any bill/sale that reduces inventory
// ============================================================================

// CheckAndNotifyLowStock checks all products in a bill and creates
// notifications for any that fell below the threshold.
//
// Flow:
//  1. Read `low_stock_threshold` from system settings table
//  2. For each product in the bill, check current stock
//  3. If stock <= threshold, find users who have low_stock_alert enabled
//  4. Insert a notification row for each such user
//
// Called from: AddBill, UpdateBill (any handler that reduces stock)
func (h *handler) CheckAndNotifyLowStock(productIDs []string) {
	// 1. Get threshold from system settings (default 5)
	var threshold int
	err := h.DB.QueryRow(
		"SELECT COALESCE(value, '5') FROM settings WHERE setting_key = 'low_stock_threshold'",
	).Scan(&threshold)
	if err != nil {
		threshold = 5
	}

	// 2. Find products that are now at or below threshold
	for _, pid := range productIDs {
		var productName string
		var currentStock int
		err := h.DB.QueryRow(
			"SELECT name, stock FROM products WHERE id = ?", pid,
		).Scan(&productName, &currentStock)
		if err != nil || currentStock > threshold {
			continue // skip if not found or stock is fine
		}

		// 3. Find all users who have low_stock_alert = true
		rows, err := h.DB.Query(
			"SELECT user_id FROM notification_settings WHERE low_stock_alert = 1",
		)
		if err != nil {
			continue
		}

		// 4. Insert a notification for each subscribed user
		for rows.Next() {
			var userID int64
			if rows.Scan(&userID) != nil {
				continue
			}
			h.DB.Exec(
				`INSERT INTO notifications (user_id, type, title, message)
				 VALUES (?, 'low_stock', ?, ?)`,
				userID,
				"مخزون منخفض", // "Low stock"
				productName+" — الكمية المتبقية: "+strconv.Itoa(currentStock), // "Remaining qty: X"
			)
		}
		rows.Close()
	}
}

// ============================================================================
// Sample: How AddBill calls CheckAndNotifyLowStock
// ============================================================================
//
// func (h *Handler) AddBill(c *gin.Context) {
//     var req struct {
//         Products []struct {
//             ProductID string `json:"product_id"`
//             Quantity  int    `json:"quantity"`
//         } `json:"products"`
//         // ... other bill fields ...
//     }
//     if err := c.ShouldBindJSON(&req); err != nil {
//         c.JSON(400, gin.H{"detail": "invalid request"})
//         return
//     }
//
//     // ... create the bill, deduct stock per product ...
//     // for _, p := range req.Products {
//     //     h.DB.Exec("UPDATE products SET stock = stock - ? WHERE id = ?", p.Quantity, p.ProductID)
//     // }
//
//     // After successful bill creation, check low stock
//     var productIDs []string
//     for _, p := range req.Products {
//         productIDs = append(productIDs, p.ProductID)
//     }
//     go h.CheckAndNotifyLowStock(productIDs)  // async — don't block the response
//
//     c.JSON(201, gin.H{"detail": "bill created"})
// }
