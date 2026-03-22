-- pkg/db/queries/notification.sql

-- name: GetNotifications :many
SELECT id, user_id, type, title, message, is_read, created_at
FROM notifications
WHERE user_id = ? 
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetUnreadCount :one
SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0;

-- name: CreateNotification :exec
INSERT INTO notifications (user_id, type, title, message)
VALUES (?, ?, ?, ?);

-- name: MarkAsRead :exec
UPDATE notifications SET is_read = 1 WHERE id = ? AND user_id = ?;

-- name: MarkAllAsRead :exec
UPDATE notifications SET is_read = 1 WHERE user_id = ?;

-- name: GetNotificationSettings :one
SELECT * FROM notification_settings WHERE user_id = ?;

-- name: UpsertNotificationSettings :exec
INSERT INTO notification_settings (user_id, low_stock_alert, low_stock_threshold, 
    pending_invoice_days, new_order_alert, paymen_due_alert, daily_summary, email_enabled)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    low_stock_alert = VALUES(low_stock_alert),
    low_stock_threshold = VALUES(low_stock_threshold),
    pending_invoice_days = VALUES(pending_invoice_days),
    new_order_alert = VALUES(new_order_alert),
    paymen_due_alert = VALUES(paymen_due_alert),
    daily_summary = VALUES(daily_summary),
    email_enabled = VALUES(email_enabled);
