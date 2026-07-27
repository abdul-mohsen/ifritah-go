package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func runNotificationRequest(t *testing.T, handlerFunc gin.HandlerFunc, method, path, body string, userID int64) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userId", userID)
	c.Set("decoded_jwt", &model.Claims{Id: userID, Username: "admin"})
	handlerFunc(c)
	return w
}

func TestGetUnreadNotificationCount(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectExec("INSERT INTO notifications").
		WithArgs(int64(7), notificationTypeSystem, currentReleaseTitle, currentReleaseMessage,
			int64(7), notificationTypeSystem, currentReleaseTitle, currentReleaseMessage).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM notifications WHERE user_id = \\? AND is_read = 0").
		WithArgs(int32(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	w := runNotificationRequest(t, h.GetUnreadNotificationCount, http.MethodGet, "/api/v2/notification/unread-count", "", 7)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	var got map[string]int64
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["count"] != 3 {
		t.Fatalf("count = %d, want 3", got["count"])
	}
	assertMockExpectations(t, mock)
}

func TestGetNotificationsReturnsFrontendEnvelope(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectExec("INSERT INTO notifications").
		WithArgs(int64(7), notificationTypeSystem, currentReleaseTitle, currentReleaseMessage,
			int64(7), notificationTypeSystem, currentReleaseTitle, currentReleaseMessage).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, type, title, message, is_read, created_at").
		WithArgs(int64(7), 2, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "title", "message", "is_read", "created_at"}).
			AddRow(uint64(7), int8(1), "مخزون منخفض", "فلتر زيت", false, time.Date(2026, time.February, 14, 10, 0, 0, 0, time.UTC)).
			AddRow(uint64(8), int8(0), "النظام", "تم الحفظ", true, time.Date(2026, time.February, 13, 10, 0, 0, 0, time.UTC)))

	w := runNotificationRequest(t, h.GetNotifications, http.MethodGet, "/api/v2/notification?limit=1", "", 7)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Items []struct {
			Type string `json:"type"`
			Read bool   `json:"read"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
		HasMore    bool   `json:"has_more"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Type != "low_stock" || got.Items[0].Read {
		t.Fatalf("items = %#v", got.Items)
	}
	if !got.HasMore || got.NextCursor != "1" {
		t.Fatalf("pagination = has_more:%t next:%q", got.HasMore, got.NextCursor)
	}
	assertMockExpectations(t, mock)
}

func TestUpdateNotificationConfigPreservesOmittedValues(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT low_stock_alert, low_stock_threshold, pending_invoice_days").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"low_stock_alert", "low_stock_threshold", "pending_invoice_days",
			"new_order_alert", "payment_due_alert", "daily_summary", "email_enabled",
		}).AddRow(true, uint32(12), uint32(14), false, true, true, false))
	mock.ExpectExec("INSERT INTO notification_settings").
		WithArgs(int32(7), true, uint32(12), uint32(14), false, true, true, false).
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := runNotificationRequest(t, h.UpdateNotificationConfig, http.MethodPut, "/api/v2/notification/config",
		`{"new_order_alert":false}`, 7)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Data notificationConfigResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Data.LowStockThreshold != 12 || got.Data.PendingDays != 14 ||
		got.Data.DailySummary != true || got.Data.NewOrderAlert != false {
		t.Fatalf("config = %#v", got.Data)
	}
	assertMockExpectations(t, mock)
}

func TestNotificationTypeLabelFallback(t *testing.T) {
	if got := notificationTypeLabel(notificationTypePaymentDue); got != "payment_due" {
		t.Fatalf("payment type = %q", got)
	}
	if got := notificationTypeLabel(99); got != "system" {
		t.Fatalf("unknown type = %q", got)
	}
}

func TestCheckAndNotifyLowStockCreatesNotificationFromProductQuantity(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT name, quantity FROM product WHERE id = \\?").
		WithArgs("42").
		WillReturnRows(sqlmock.NewRows([]string{"name", "quantity"}).AddRow("Oil filter", "2.500"))
	mock.ExpectQuery("SELECT user_id, low_stock_threshold FROM notification_settings WHERE low_stock_alert = 1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "low_stock_threshold"}).AddRow(int64(7), uint32(5)))
	mock.ExpectExec("INSERT INTO notifications").
		WithArgs(int64(7), int8(1), "مخزون منخفض", "Oil filter — الكمية المتبقية: 2.5").
		WillReturnResult(sqlmock.NewResult(1, 1))

	h.CheckAndNotifyLowStock([]string{"42"})

	assertMockExpectations(t, mock)
}

func TestLowStockProductIDsDeduplicatesCatalogProducts(t *testing.T) {
	firstID := uint64(42)
	secondID := uint64(84)

	got := lowStockProductIDs([]model.BillProduct{
		{ProductId: &firstID},
		{ProductId: nil},
		{ProductId: &firstID},
		{ProductId: &secondID},
	})

	want := []string{"42", "84"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("low-stock product IDs = %#v, want %#v", got, want)
	}
}

func TestEnsureCurrentReleaseNotificationIsIdempotent(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	expect := func(rows int64) {
		mock.ExpectExec("INSERT INTO notifications").
			WithArgs(int64(7), notificationTypeSystem, currentReleaseTitle, currentReleaseMessage,
				int64(7), notificationTypeSystem, currentReleaseTitle, currentReleaseMessage).
			WillReturnResult(sqlmock.NewResult(1, rows))
	}
	expect(1)
	expect(0)

	if err := h.ensureCurrentReleaseNotification(7); err != nil {
		t.Fatalf("first release notification ensure failed: %v", err)
	}
	if err := h.ensureCurrentReleaseNotification(7); err != nil {
		t.Fatalf("second release notification ensure failed: %v", err)
	}

	assertMockExpectations(t, mock)
}
