package handlers

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

const createCashVoucherQuery = `INSERT INTO cash_voucher (
  voucher_number, voucher_type, effective_date, amount, payment_method,
  state, reference_type, reference_id,
  recipient_type, recipient_id, recipient_name,
  description, note, bank_name, bank_account, transaction_reference,
  store_id, merchant_id, branch_id, created_by, approved_by
) VALUES (
  ?, ?, ?, ?, ?,
  0, ?, ?,
  ?, ?, ?,
  ?, ?, ?, ?, ?,
  ?, ?, ?, ?, ?
)`

// TestCreateCashVoucherStartsUnapproved is a regression test for a bug where
// CreateCashVoucher populated approved_by with the creator's id at creation
// time, even though a new voucher starts in Draft (state=0) and has not gone
// through the /approve workflow. approved_by must stay NULL until a
// manager/admin actually approves it via ApproveCashVoucher; otherwise the
// audit trail is misleading (a Draft record would show a non-null approver).
func TestCreateCashVoucherStartsUnapproved(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM store WHERE id = ?")).
		WithArgs(int32(54)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(voucher_number), 0) + 1 FROM cash_voucher WHERE merchant_id = ?")).
		WithArgs(int32(7)).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(1))

	mock.ExpectExec(regexp.QuoteMeta(createCashVoucherQuery)).
		WithArgs(
			sqlmock.AnyArg(), // voucher_number
			sqlmock.AnyArg(), // voucher_type
			sqlmock.AnyArg(), // effective_date
			sqlmock.AnyArg(), // amount
			sqlmock.AnyArg(), // payment_method
			sqlmock.AnyArg(), // reference_type
			sqlmock.AnyArg(), // reference_id
			sqlmock.AnyArg(), // recipient_type
			sqlmock.AnyArg(), // recipient_id
			sqlmock.AnyArg(), // recipient_name
			sqlmock.AnyArg(), // description
			sqlmock.AnyArg(), // note
			sqlmock.AnyArg(), // bank_name
			sqlmock.AnyArg(), // bank_account
			sqlmock.AnyArg(), // transaction_reference
			sqlmock.AnyArg(), // store_id
			sqlmock.AnyArg(), // merchant_id
			sqlmock.AnyArg(), // branch_id
			sqlmock.AnyArg(), // created_by
			nil,              // approved_by: must be NULL — voucher isn't approved yet
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{
		"voucher_type": "disbursement",
		"effective_date": "2026-07-18T00:00:00Z",
		"amount": "100.00",
		"recipient_type": "supplier",
		"recipient_name": "Test Supplier",
		"store_id": 54,
		"branch_id": 1
	}`
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v2/cash_voucher", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "admin"})

	h.CreateCashVoucher(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations (this fails today because CreateCashVoucher binds the creator's id as approved_by instead of NULL): %v", err)
	}
}
