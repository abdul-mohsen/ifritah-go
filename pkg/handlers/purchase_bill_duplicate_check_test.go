package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

const duplicatePurchaseBillQuery = `SELECT b.id
FROM purchase_bill AS b
JOIN supplier ON supplier.id = b.supplier_id
JOIN user ON user.id = ? AND user.company_id = supplier.company_id
WHERE b.supplier_id = ?
  AND b.supplier_sequence_number = ?
LIMIT 1`

const purchaseBillSettingQuery = `SELECT COALESCE(value, '') AS value
FROM settings
WHERE setting_key = ?
LIMIT 1`

func newPurchaseBillTestHandler(t *testing.T) (*handler, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	h := New(sqlDB, db.New(sqlDB), nil)
	return &h, mock, func() { _ = sqlDB.Close() }
}

func runPurchaseBillRequest(t *testing.T, handlerFunc gin.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "admin"})

	handlerFunc(c)
	return w
}

func expectDuplicateCheck(mock sqlmock.Sqlmock, supplierID int32, purchaseBillID *uint64) {
	expectation := mock.ExpectQuery(regexp.QuoteMeta(duplicatePurchaseBillQuery)).
		WithArgs(int32(7), supplierID, sqlmock.AnyArg())
	if purchaseBillID == nil {
		expectation.WillReturnError(sql.ErrNoRows)
		return
	}
	expectation.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(*purchaseBillID))
}

func assertDuplicateCheckResponse(t *testing.T, w *httptest.ResponseRecorder, exists bool, purchaseBillID *uint64) {
	t.Helper()

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var response model.PurchaseBillDuplicateCheckResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Exists != exists {
		t.Fatalf("exists = %v, want %v; response=%+v", response.Exists, exists, response)
	}
	if purchaseBillID == nil {
		if response.PurchaseBillId != nil {
			t.Fatalf("purchase_bill_id = %v, want omitted", *response.PurchaseBillId)
		}
		return
	}
	if response.PurchaseBillId == nil || *response.PurchaseBillId != *purchaseBillID {
		t.Fatalf("purchase_bill_id = %v, want %d", response.PurchaseBillId, *purchaseBillID)
	}
}

func assertMockExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestCheckPurchaseBillDuplicate(t *testing.T) {
	purchaseBillID := uint64(789)
	testCases := []struct {
		name           string
		foundID        *uint64
		wantExists     bool
		wantResponseID *uint64
	}{
		{name: "unused pair", foundID: nil, wantExists: false, wantResponseID: nil},
		{name: "same company duplicate", foundID: &purchaseBillID, wantExists: true, wantResponseID: &purchaseBillID},
		{name: "other company hidden", foundID: nil, wantExists: false, wantResponseID: nil},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			h, mock, cleanup := newPurchaseBillTestHandler(t)
			defer cleanup()
			expectDuplicateCheck(mock, 123, testCase.foundID)

			w := runPurchaseBillRequest(t, h.CheckPurchaseBillDuplicate, http.MethodPost, "/api/v2/purchase_bill/duplicate-check", `{"supplier_id":123,"supplier_sequence_number":456}`)

			assertDuplicateCheckResponse(t, w, testCase.wantExists, testCase.wantResponseID)
			assertMockExpectations(t, mock)
		})
	}
}

func TestCheckPurchaseBillDuplicateDatabaseErrorIsExplicit(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta(duplicatePurchaseBillQuery)).
		WithArgs(int32(7), int32(123), sqlmock.AnyArg()).
		WillReturnError(errors.New("database unavailable"))

	w := runPurchaseBillRequest(t, h.CheckPurchaseBillDuplicate, http.MethodPost, "/api/v2/purchase_bill/duplicate-check", `{"supplier_id":123,"supplier_sequence_number":456}`)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "unable to verify duplicate purchase bill") {
		t.Fatalf("expected explicit duplicate-check error, body=%s", w.Body.String())
	}
	assertMockExpectations(t, mock)
}

func TestAddPurchaseBillDuplicateConflictStillReturnsConflict(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("select store.id from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(purchaseBillSettingQuery)).
		WithArgs("pb_pdf_required").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("disabled"))
	mock.ExpectExec("insert into purchase_bill").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectRollback()

	body := `{"store_id":1,"products":[],"manual_products":[],"supplier_id":123,"supplier_sequence_number":456,"payment_method":10}`
	w := runPurchaseBillRequest(t, h.AddPurchaseBill, http.MethodPost, "/api/v2/purchase_bill", body)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
	assertMockExpectations(t, mock)
}

func TestValidatePurchaseBillPDFRequirement(t *testing.T) {
	tests := []struct {
		name       string
		setting    string
		settingErr error
		pdfLink    *string
		wantOK     bool
		wantStatus int
		wantCode   string
	}{
		{
			name:       "required setting rejects missing PDF",
			setting:    "required",
			wantStatus: http.StatusBadRequest,
			wantCode:   "PURCHASE_BILL_PDF_REQUIRED",
		},
		{
			name:    "required setting accepts PDF link",
			setting: "required",
			pdfLink: strPtr("/api/v2/files/invoice.pdf"),
			wantOK:  true,
		},
		{
			name:    "optional setting accepts missing PDF",
			setting: "optional",
			wantOK:  true,
		},
		{
			name:    "disabled setting accepts missing PDF",
			setting: "disabled",
			wantOK:  true,
		},
		{
			name:       "missing setting fails closed",
			settingErr: sql.ErrNoRows,
			wantStatus: http.StatusBadRequest,
			wantCode:   "PURCHASE_BILL_PDF_REQUIRED",
		},
		{
			name:       "settings lookup failure is explicit",
			settingErr: errors.New("settings database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "PURCHASE_BILL_PDF_SETTING_UNAVAILABLE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, mock, cleanup := newPurchaseBillTestHandler(t)
			defer cleanup()

			query := mock.ExpectQuery(regexp.QuoteMeta(purchaseBillSettingQuery)).
				WithArgs("pb_pdf_required")
			if tc.settingErr != nil {
				query.WillReturnError(tc.settingErr)
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(tc.setting))
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v2/purchase_bill", nil)

			got := h.validatePurchaseBillPDFRequirement(c, model.AddPurchaseBillRequest{PDFLink: tc.pdfLink})
			if got != tc.wantOK {
				t.Fatalf("validatePurchaseBillPDFRequirement() = %v, want %v", got, tc.wantOK)
			}
			if tc.wantStatus != 0 && w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantCode != "" {
				var body map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
				}
				if body["code"] != tc.wantCode {
					t.Fatalf("code = %q, want %q", body["code"], tc.wantCode)
				}
			}
			assertMockExpectations(t, mock)
		})
	}
}

func TestAddPurchaseBillRequiredPDFRejectsBeforeInsert(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("select store.id from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(purchaseBillSettingQuery)).
		WithArgs("pb_pdf_required").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("required"))
	mock.ExpectRollback()

	body := `{"store_id":1,"products":[],"manual_products":[],"supplier_id":123,"supplier_sequence_number":456,"payment_method":10}`
	w := runPurchaseBillRequest(t, h.AddPurchaseBill, http.MethodPost, "/api/v2/purchase_bill", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if response["code"] != "PURCHASE_BILL_PDF_REQUIRED" {
		t.Fatalf("code = %q, want PURCHASE_BILL_PDF_REQUIRED", response["code"])
	}
	assertMockExpectations(t, mock)
}
