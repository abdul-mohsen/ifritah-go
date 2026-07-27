package handlers

import "log"

const (
	currentReleaseTitle   = "تحديث النظام - يوليو 2026"
	currentReleaseMessage = "تحسينات الجودة: بحث وفلاتر موحّدة، حفظ إعدادات موثوق، منع الفواتير ذات الإجمالي الصفري، وإشعارات مخزون منخفض أكثر دقة."
)

// ensureCurrentReleaseNotification creates the current release note once for
// each user. The NOT EXISTS guard keeps repeated bell polling idempotent.
func (h *handler) ensureCurrentReleaseNotification(userID int64) error {
	const query = `INSERT INTO notifications (user_id, type, title, message)
		SELECT ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1
			FROM notifications
			WHERE user_id = ? AND type = ? AND title = ? AND message = ?
		)`

	_, err := h.DB.Exec(
		query,
		userID,
		notificationTypeSystem,
		currentReleaseTitle,
		currentReleaseMessage,
		userID,
		notificationTypeSystem,
		currentReleaseTitle,
		currentReleaseMessage,
	)
	if err != nil {
		log.Printf("ensureCurrentReleaseNotification: %v", err)
	}
	return err
}
