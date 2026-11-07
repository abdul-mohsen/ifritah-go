package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
)

func newBillScopeHandler(t *testing.T) (*handler, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	h := New(sqlDB, db.New(sqlDB), nil)
	return &h, mock
}

func billScopeRequest(t *testing.T, role string, method, path, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "scoped-user", Role: role})
	c.Set("user_id", int64(7))
	c.Set("userId", int64(7))
	c.Set("user_role", role)
	return c
}

func expectScopeStoreIDs(mock sqlmock.Sqlmock, role string, storeIDs ...int32) {
	rows := sqlmock.NewRows([]string{"id", "addressId", "name"})
	for _, id := range storeIDs {
		rows.AddRow(id, nil, nil)
	}
	mock.ExpectQuery("SELECT s\\.id").
		WithArgs(int32(7), role, role, role, role, role).
		WillReturnRows(rows)
}

func assertScopeStatus(t *testing.T, mock sqlmock.Sqlmock, c *gin.Context, want int) {
	t.Helper()
	if c.Writer.Status() != want {
		t.Fatalf("status = %d, want %d", c.Writer.Status(), want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func assertBillScope(t *testing.T, got billVisibilityScope, want billVisibilityScope) {
	t.Helper()
	if !got.enforced {
		t.Fatal("scope must be enforced for authenticated role")
	}
	if len(got.storeIDs) != len(want.storeIDs) {
		t.Fatalf("store scope = %v, want %v", got.storeIDs, want.storeIDs)
	}
	if (got.merchantID == nil) != (want.merchantID == nil) {
		t.Fatalf("merchant scope = %v, want %v", got.merchantID, want.merchantID)
	}
	if got.merchantID != nil && *got.merchantID != *want.merchantID {
		t.Fatalf("merchant scope = %d, want %d", *got.merchantID, *want.merchantID)
	}
}

func TestBillVisibilityScopeRoleMatrix(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		storeIDs   []int32
		merchantID *int32
	}{
		{name: "employee owns bills in assigned company stores", role: RoleEmployee, storeIDs: []int32{1}, merchantID: func() *int32 { id := int32(7); return &id }()},
		{name: "manager receives managed branch stores", role: RoleManager, storeIDs: []int32{2}},
		{name: "admin receives every company store", role: RoleAdmin, storeIDs: []int32{1, 2}},
		{name: "owner receives every company store", role: "owner", storeIDs: []int32{1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, mock := newBillScopeHandler(t)
			expectScopeStoreIDs(mock, tt.role, tt.storeIDs...)

			c := billScopeRequest(t, tt.role, http.MethodGet, "/bill/1", "")
			scope, err := h.billVisibilityScope(c)
			if err != nil {
				t.Fatalf("billVisibilityScope: %v", err)
			}
			assertBillScope(t, scope, billVisibilityScope{storeIDs: tt.storeIDs, merchantID: tt.merchantID})
			assertScopeStatus(t, mock, c, http.StatusOK)
		})
	}
}

func TestBillVisibilityScopePropagatesStoreScopeQueryError(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	mock.ExpectQuery("SELECT s\\.id").
		WithArgs(int32(7), RoleManager, RoleManager, RoleManager, RoleManager, RoleManager).
		WillReturnError(errors.New("store scope unavailable"))

	c := billScopeRequest(t, RoleManager, http.MethodGet, "/bill/1", "")
	if _, err := h.billVisibilityScope(c); err == nil {
		t.Fatal("billVisibilityScope should propagate store scope query errors")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetBillsRejectsStoreOutsideRoleScope(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	expectScopeStoreIDs(mock, RoleManager, 1)

	c := billScopeRequest(t, RoleManager, http.MethodPost, "/bill/all", `{"store_ids":[2],"limit":1}`)
	h.GetBills(c)
	assertScopeStatus(t, mock, c, http.StatusBadRequest)
}

func TestGetBillsPassesScopedStoresThroughCursorQuery(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	expectScopeStoreIDs(mock, RoleEmployee, 1)
	mock.ExpectQuery(`bill\.store_id IN \(\?\)`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	cursor := pagination.Encode(pagination.Cursor{
		K: []any{"2026-01-02T03:04:05Z", int64(42), int64(0)},
		S: "-effective_date",
		D: pagination.DirectionAfter,
	})
	body := `{"store_ids":[1],"limit":1,"cursor":"` + cursor + `","query":"Alice","sequence_number":"10"}`
	c := billScopeRequest(t, RoleEmployee, http.MethodPost, "/bill/all", body)
	h.GetBills(c)
	assertScopeStatus(t, mock, c, http.StatusOK)
}

func TestGetBillDetailHidesCrossStoreDirectID(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	expectScopeStoreIDs(mock, RoleManager, 1)
	mock.ExpectQuery(`WHERE b\.id = \?\s+AND b\.store_id IN \(\?\)`).
		WithArgs(uint64(42), int32(1), nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	c := billScopeRequest(t, RoleManager, http.MethodGet, "/bill/42", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	h.GetBillDetail(c)
	assertScopeStatus(t, mock, c, http.StatusNotFound)
}

func TestGetBillPDFHidesCrossStoreDirectID(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	expectScopeStoreIDs(mock, RoleManager, 1)
	mock.ExpectQuery(`WHERE b\.id = \?\s+AND b\.store_id IN \(\?\)`).
		WithArgs(uint64(42), int32(1), nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	c := billScopeRequest(t, RoleManager, http.MethodGet, "/bill/pdf/42", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	h.GetBillPDF(c)
	assertScopeStatus(t, mock, c, http.StatusNotFound)
}

func TestSendBillWhatsAppHidesCrossStoreDirectID(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	expectScopeStoreIDs(mock, RoleEmployee, 1)
	expectListSettings(mock, enabledWhatsAppSettings(defaultWhatsAppInvoiceMessage))
	mock.ExpectQuery(getBillPDFPattern).
		WithArgs(uint64(42), int32(1), int32(7), int32(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	c := billScopeRequest(t, RoleEmployee, http.MethodPost, "/bill/42/whatsapp", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	h.SendBillWhatsApp(c)
	assertScopeStatus(t, mock, c, http.StatusNotFound)
}

func TestGetCreditBillDetailHidesCrossStoreDirectID(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	expectScopeStoreIDs(mock, RoleManager, 1)
	mock.ExpectQuery(`CONCAT\('https://ifritah\.com/bill/'`).
		WithArgs(uint64(42), int32(1), nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	c := billScopeRequest(t, RoleManager, http.MethodGet, "/credit_bill/42", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	h.GetBillCreditDetail(c)
	assertScopeStatus(t, mock, c, http.StatusNotFound)
}

func TestEmployeeDirectIDScopeIncludesMerchantOwner(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	expectScopeStoreIDs(mock, RoleEmployee, 1)
	mock.ExpectQuery("SELECT b\\.id").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	c := billScopeRequest(t, RoleEmployee, http.MethodGet, "/bill/42", "")
	if h.checkBillIDAccess(c, 42) {
		t.Fatal("cross-owner bill should be denied")
	}
	assertScopeStatus(t, mock, c, http.StatusNotFound)
}
