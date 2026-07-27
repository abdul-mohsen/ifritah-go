package handlers

import (
	"database/sql"
	db "ifritah/web-service-gin/pkg/db/gen"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const (
	notificationTypeSystem     int8 = 0
	notificationTypeLowStock   int8 = 1
	notificationTypePaymentDue int8 = 2
	notificationTypeNewOrder   int8 = 3
)

type notificationConfigResponse struct {
	UserID            int64  `json:"user_id"`
	LowStockAlert     bool   `json:"low_stock_alert"`
	LowStockThreshold uint32 `json:"low_stock_threshold"`
	PendingDays       uint32 `json:"pending_invoice_days"`
	NewOrderAlert     bool   `json:"new_order_alert"`
	PaymentDueAlert   bool   `json:"payment_due_alert"`
	DailySummary      bool   `json:"daily_summary"`
	EmailEnabled      bool   `json:"email_enabled"`
}

func defaultNotificationConfig(userID int64) notificationConfigResponse {
	return notificationConfigResponse{
		UserID:            userID,
		LowStockAlert:     true,
		LowStockThreshold: 5,
		PendingDays:       7,
		NewOrderAlert:     true,
		PaymentDueAlert:   true,
	}
}

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

	config := defaultNotificationConfig(userID)

	err := h.DB.QueryRow(
		`SELECT low_stock_alert, low_stock_threshold, pending_invoice_days,
		        new_order_alert, payment_due_alert, daily_summary, email_enabled
		 FROM notification_settings WHERE user_id = ?`,
		userID,
	).Scan(&config.LowStockAlert, &config.LowStockThreshold, &config.PendingDays,
		&config.NewOrderAlert, &config.PaymentDueAlert, &config.DailySummary, &config.EmailEnabled)

	if err == sql.ErrNoRows {
		// Return defaults.
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
	user := GetSessionInfo(c)

	var req struct {
		LowStockAlert     *bool   `json:"low_stock_alert"`
		LowStockThreshold *uint32 `json:"low_stock_threshold"`
		PendingDays       *uint32 `json:"pending_invoice_days"`
		NewOrderAlert     *bool   `json:"new_order_alert"`
		PaymentDueAlert   *bool   `json:"payment_due_alert"`
		DailySummary      *bool   `json:"daily_summary"`
		EmailEnabled      *bool   `json:"email_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("UpdateNotificationConfig: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	config := defaultNotificationConfig(user.id)
	err := h.DB.QueryRow(
		`SELECT low_stock_alert, low_stock_threshold, pending_invoice_days,
		        new_order_alert, payment_due_alert, daily_summary, email_enabled
		 FROM notification_settings WHERE user_id = ?`,
		user.id,
	).Scan(&config.LowStockAlert, &config.LowStockThreshold, &config.PendingDays,
		&config.NewOrderAlert, &config.PaymentDueAlert, &config.DailySummary, &config.EmailEnabled)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("UpdateNotificationConfig read existing: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load notification config"})
		return
	}

	if req.LowStockAlert != nil {
		config.LowStockAlert = *req.LowStockAlert
	}
	if req.LowStockThreshold != nil {
		config.LowStockThreshold = *req.LowStockThreshold
	}
	if req.PendingDays != nil {
		config.PendingDays = *req.PendingDays
	}
	if req.NewOrderAlert != nil {
		config.NewOrderAlert = *req.NewOrderAlert
	}
	if req.PaymentDueAlert != nil {
		config.PaymentDueAlert = *req.PaymentDueAlert
	}
	if req.DailySummary != nil {
		config.DailySummary = *req.DailySummary
	}
	if req.EmailEnabled != nil {
		config.EmailEnabled = *req.EmailEnabled
	}

	args := db.UpsertNotificationSettingsParams{
		UserID:             int32(user.id),
		LowStockAlert:      config.LowStockAlert,
		LowStockThreshold:  config.LowStockThreshold,
		PendingInvoiceDays: config.PendingDays,
		NewOrderAlert:      config.NewOrderAlert,
		PaymentDueAlert:    config.PaymentDueAlert,
		DailySummary:       config.DailySummary,
		EmailEnabled:       config.EmailEnabled,
	}
	err = h.queries.UpsertNotificationSettings(c.Request.Context(), args)
	if err != nil {
		log.Printf("UpdateNotificationConfig: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to save notification config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success", "data": config})
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
	if err := h.ensureCurrentReleaseNotification(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to prepare notifications"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := 0
	if cursor := c.Query("cursor"); cursor != "" {
		if parsed, parseErr := strconv.Atoi(cursor); parseErr == nil && parsed > 0 {
			offset = parsed
		}
	}
	unreadOnly := c.Query("unread_only") == "1" || c.Query("unread_only") == "true"

	query := `SELECT id, type, title, message, is_read, created_at
		FROM notifications
		WHERE user_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?`
	args := []any{userID, limit + 1, offset}
	if unreadOnly {
		query = `SELECT id, type, title, message, is_read, created_at
			FROM notifications
			WHERE user_id = ? AND is_read = 0
			ORDER BY created_at DESC, id DESC
			LIMIT ? OFFSET ?`
	}

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		log.Printf("GetNotifications: %v", err)
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
		var typeCode int8
		var createdAt time.Time
		if err := rows.Scan(&n.ID, &typeCode, &n.Title, &n.Message, &n.IsRead, &createdAt); err != nil {
			log.Printf("GetNotifications scan: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to read notifications"})
			return
		}
		n.Type = notificationTypeLabel(typeCode)
		n.CreatedAt = createdAt.Format(time.RFC3339)
		notifs = append(notifs, n)
	}
	if err := rows.Err(); err != nil {
		log.Printf("GetNotifications rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to read notifications"})
		return
	}
	if notifs == nil {
		notifs = []Notification{}
	}

	hasMore := len(notifs) > limit
	if hasMore {
		notifs = notifs[:limit]
	}
	nextCursor := ""
	if hasMore {
		nextCursor = strconv.Itoa(offset + limit)
	}
	prevCursor := ""
	if offset > 0 {
		previous := offset - limit
		if previous < 0 {
			previous = 0
		}
		prevCursor = strconv.Itoa(previous)
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       notifs,
		"data":        notifs,
		"next_cursor": nextCursor,
		"prev_cursor": prevCursor,
		"has_more":    hasMore,
	})
}

// GetUnreadNotificationCount returns the unread notification count for the
// current user.
func (h *handler) GetUnreadNotificationCount(c *gin.Context) {
	userID := c.GetInt64("userId")
	if err := h.ensureCurrentReleaseNotification(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to prepare notifications"})
		return
	}
	count, err := h.queries.GetUnreadCount(c.Request.Context(), int32(userID))
	if err != nil {
		log.Printf("GetUnreadNotificationCount: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to count notifications"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

// MarkNotificationRead marks a notification as read.
// PUT /api/v2/notification/:id/read
//
// Response: {"detail": "success"}
func (h *handler) MarkNotificationRead(c *gin.Context) {
	userID := c.GetInt64("userId")
	notifID := c.Param("id")

	result, err := h.DB.Exec(
		"UPDATE notifications SET is_read = 1 WHERE id = ? AND user_id = ?",
		notifID, userID,
	)
	if err != nil {
		log.Printf("MarkNotificationRead: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update"})
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Printf("MarkNotificationRead rows affected: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update"})
		return
	}
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "notification not found"})
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

	result, err := h.DB.Exec("UPDATE notifications SET is_read = 1 WHERE user_id = ?", userID)
	if err != nil {
		log.Printf("MarkAllNotificationsRead: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update"})
		return
	}
	count, err := result.RowsAffected()
	if err != nil {
		log.Printf("MarkAllNotificationsRead rows affected: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success", "count": count})
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

func uintDefault(ptr *uint, def uint) uint {
	if ptr != nil {
		return *ptr
	}
	return def
}

func uint32Default(ptr *uint32, def uint32) uint32 {
	if ptr != nil {
		return *ptr
	}
	return def
}

func notificationTypeLabel(typeCode int8) string {
	switch typeCode {
	case notificationTypeLowStock:
		return "low_stock"
	case notificationTypePaymentDue:
		return "payment_due"
	case notificationTypeNewOrder:
		return "new_order"
	case notificationTypeSystem:
		return "system"
	default:
		return "system"
	}
}

// ============================================================================
// Low Stock Check — call this after any bill/sale that reduces inventory
// ============================================================================

// CheckAndNotifyLowStock checks all products in a bill and creates
// notifications for any that fell below the threshold.
//
// Flow:
//  1. For each product in the bill, check current stock
//  2. Find users who have low_stock_alert enabled and their thresholds
//  3. Insert a notification row for each user whose threshold is reached
//
// Called after a successful stock-changing transaction.
func (h *handler) CheckAndNotifyLowStock(productIDs []string) {
	// Find products that are now at or below a subscriber's threshold.
	for _, pid := range productIDs {
		var productName sql.NullString
		var currentStock decimal.Decimal
		err := h.DB.QueryRow(
			"SELECT name, quantity FROM product WHERE id = ?", pid,
		).Scan(&productName, &currentStock)
		if err != nil {
			log.Printf("CheckAndNotifyLowStock: load product %s: %v", pid, err)
			continue
		}
		// Find all users who have low_stock_alert enabled and retain each
		// user's persisted threshold.
		rows, err := h.DB.Query(
			"SELECT user_id, low_stock_threshold FROM notification_settings WHERE low_stock_alert = 1",
		)
		if err != nil {
			log.Printf("CheckAndNotifyLowStock: load subscribers: %v", err)
			continue
		}

		// Insert a notification for each subscribed user whose threshold is met.
		for rows.Next() {
			var userID int64
			var threshold uint32
			if err := rows.Scan(&userID, &threshold); err != nil {
				log.Printf("CheckAndNotifyLowStock: read subscriber: %v", err)
				continue
			}
			if currentStock.GreaterThan(decimal.NewFromInt(int64(threshold))) {
				continue
			}
			name := productName.String
			if !productName.Valid {
				name = pid
			}
			if _, err := h.DB.Exec(
				`INSERT INTO notifications (user_id, type, title, message)
				 VALUES (?, ?, ?, ?)`,
				userID,
				notificationTypeLowStock,
				"مخزون منخفض", // "Low stock"
				name+" — الكمية المتبقية: "+currentStock.String(), // "Remaining qty"
			); err != nil {
				log.Printf("CheckAndNotifyLowStock: create notification for user %d: %v", userID, err)
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("CheckAndNotifyLowStock: iterate subscribers: %v", err)
		}
		rows.Close()
	}
}
