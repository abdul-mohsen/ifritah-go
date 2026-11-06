package handlers

import (
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
			rows := sqlmock.NewRows([]string{"id"})
			for _, id := range tt.storeIDs {
				rows.AddRow(id)
			}
			mock.ExpectQuery("SELECT s\\.id").
				WithArgs(int64(7), tt.role, tt.role, tt.role).
				WillReturnRows(rows)

			c := billScopeRequest(t, tt.role, http.MethodGet, "/bill/1", "")
			scope, err := h.billVisibilityScope(c)
			if err != nil {
				t.Fatalf("billVisibilityScope: %v", err)
			}
			if !scope.enforced {
				t.Fatal("scope must be enforced for authenticated role")
			}
			if len(scope.storeIDs) != len(tt.storeIDs) {
				t.Fatalf("store scope = %v, want %v", scope.storeIDs, tt.storeIDs)
			}
			if (scope.merchantID == nil) != (tt.merchantID == nil) {
				t.Fatalf("merchant scope = %v, want %v", scope.merchantID, tt.merchantID)
			}
			if scope.merchantID != nil && *scope.merchantID != *tt.merchantID {
				t.Fatalf("merchant scope = %d, want %d", *scope.merchantID, *tt.merchantID)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGetBillsRejectsStoreOutsideRoleScope(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	mock.ExpectQuery("SELECT s\\.id").
		WithArgs(int64(7), RoleManager, RoleManager, RoleManager).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))

	c := billScopeRequest(t, RoleManager, http.MethodPost, "/bill/all", `{"store_ids":[2],"limit":1}`)
	h.GetBills(c)
	if c.Writer.Status() != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", c.Writer.Status())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetBillsPassesScopedStoresThroughCursorQuery(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	mock.ExpectQuery("SELECT s\\.id").
		WithArgs(int64(7), RoleEmployee, RoleEmployee, RoleEmployee).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))
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
	if c.Writer.Status() != http.StatusOK {
		t.Fatalf("status = %d, want 200", c.Writer.Status())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetBillDetailHidesCrossStoreDirectID(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	mock.ExpectQuery("SELECT s\\.id").
		WithArgs(int64(7), RoleManager, RoleManager, RoleManager).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))
	mock.ExpectQuery(`WHERE b\.id = \?\s+AND b\.store_id IN \(\?\)`).
		WithArgs(uint64(42), int32(1), nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	c := billScopeRequest(t, RoleManager, http.MethodGet, "/bill/42", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	h.GetBillDetail(c)
	if c.Writer.Status() != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", c.Writer.Status())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetBillPDFHidesCrossStoreDirectID(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	mock.ExpectQuery("SELECT s\\.id").
		WithArgs(int64(7), RoleManager, RoleManager, RoleManager).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))
	mock.ExpectQuery(`WHERE b\.id = \?\s+AND b\.store_id IN \(\?\)`).
		WithArgs(uint64(42), int32(1), nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	c := billScopeRequest(t, RoleManager, http.MethodGet, "/bill/pdf/42", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	h.GetBillPDF(c)
	if c.Writer.Status() != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", c.Writer.Status())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSendBillWhatsAppHidesCrossStoreDirectID(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	mock.ExpectQuery("SELECT s\\.id").
		WithArgs(int64(7), RoleEmployee, RoleEmployee, RoleEmployee).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))
	expectListSettings(mock, enabledWhatsAppSettings(defaultWhatsAppInvoiceMessage))
	mock.ExpectQuery(getBillPDFPattern).
		WithArgs(uint64(42), int32(1), int32(7), int32(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	c := billScopeRequest(t, RoleEmployee, http.MethodPost, "/bill/42/whatsapp", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	h.SendBillWhatsApp(c)
	if c.Writer.Status() != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", c.Writer.Status())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetCreditBillDetailHidesCrossStoreDirectID(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	mock.ExpectQuery("SELECT s\\.id").
		WithArgs(int64(7), RoleManager, RoleManager, RoleManager).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))
	mock.ExpectQuery(`CONCAT\('https://ifritah\.com/bill/'`).
		WithArgs(uint64(42), int32(1), nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	c := billScopeRequest(t, RoleManager, http.MethodGet, "/credit_bill/42", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	h.GetBillCreditDetail(c)
	if c.Writer.Status() != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", c.Writer.Status())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEmployeeDirectIDScopeIncludesMerchantOwner(t *testing.T) {
	h, mock := newBillScopeHandler(t)
	mock.ExpectQuery("SELECT s\\.id").
		WithArgs(int64(7), RoleEmployee, RoleEmployee, RoleEmployee).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))
	mock.ExpectQuery("SELECT b\\.id").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	c := billScopeRequest(t, RoleEmployee, http.MethodGet, "/bill/42", "")
	if h.checkBillIDAccess(c, 42) {
		t.Fatal("cross-owner bill should be denied")
	}
	if c.Writer.Status() != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", c.Writer.Status())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
