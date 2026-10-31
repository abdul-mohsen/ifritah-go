package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

func newAtomicSaleTestHandler(t *testing.T) (handler, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	cleanup := func() { _ = sqlDB.Close() }
	return New(sqlDB, db.New(sqlDB), nil), mock, cleanup
}

func atomicSaleContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v2/bill", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("decoded_jwt", &model.Claims{Id: 7, Username: "cashier"})
	return ctx, recorder
}

func atomicSaleBody(price string) string {
	return `{
		"tenant_id": 42,
		"pos_id": "pos-1",
		"bill_request_id": "request-1",
		"store_id": 2,
		"branch_id": 7,
		"state": 1,
		"discount": "0",
		"paid_amount": "20",
		"maintenance_cost": "0",
		"payment_method": 10,
		"products": [
			{"product_id": 10, "name": "Brake pad", "price": ` + price + `, "quantity": "1", "discount": "0"}
		],
		"manual_products": []
	}`
}

func expectAtomicSaleResolution(mock sqlmock.Sqlmock, resolution string) {
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT.*FROM bill AS b.*WHERE b\.tenant_id = \?.*b\.pos_id = \?.*b\.bill_request_id = \?.*LIMIT 1`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"tenant_id",
			"pos_id",
			"bill_request_id",
			"request_fingerprint",
			"request_fingerprint_version",
			"invoice_uuid",
			"request_resolution",
		}).AddRow(
			uint64(99),
			uint64(42),
			"pos-1",
			"request-1",
			strings.Repeat("a", 64),
			uint16(1),
			"invoice-99",
			resolution,
		))
	mock.ExpectRollback()
}

func TestAddBillReturnsSameContentReplay(t *testing.T) {
	h, mock, cleanup := newAtomicSaleTestHandler(t)
	defer cleanup()

	expectAtomicSaleResolution(mock, "replay")
	ctx, recorder := atomicSaleContext(atomicSaleBody("100"))

	h.AddBill(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if response["replayed"] != true {
		t.Fatalf("replayed response = %#v, want true", response["replayed"])
	}
	if response["id"] != float64(99) {
		t.Fatalf("replayed id = %#v, want 99", response["id"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAddBillRejectsChangedPayloadConflict(t *testing.T) {
	h, mock, cleanup := newAtomicSaleTestHandler(t)
	defer cleanup()

	expectAtomicSaleResolution(mock, "conflict")
	ctx, recorder := atomicSaleContext(atomicSaleBody("101"))

	h.AddBill(ctx)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusConflict, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if response["error"] != "sale_request_conflict" {
		t.Fatalf("conflict error = %#v, want sale_request_conflict", response["error"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAddBillRejectsCrossStoreSaleBeforeMutation(t *testing.T) {
	h, mock, cleanup := newAtomicSaleTestHandler(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT.*FROM bill AS b.*WHERE b\.tenant_id = \?.*b\.pos_id = \?.*b\.bill_request_id = \?.*LIMIT 1`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?is)SELECT.*FROM store AS s.*JOIN branches AS b.*JOIN `+"`user`"+` AS u.*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	ctx, recorder := atomicSaleContext(atomicSaleBody("100"))
	h.AddBill(ctx)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAddBillRejectsProductFromAnotherStore(t *testing.T) {
	h, mock, cleanup := newAtomicSaleTestHandler(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT.*FROM bill AS b.*WHERE b\.tenant_id = \?.*b\.pos_id = \?.*b\.bill_request_id = \?.*LIMIT 1`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?is)SELECT.*FROM store AS s.*JOIN branches AS b.*JOIN `+"`user`"+` AS u.*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "branch_id", "company_id"}).AddRow(2, 7, 11))
	mock.ExpectQuery(`(?is)SELECT.*FROM product AS p.*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	ctx, recorder := atomicSaleContext(atomicSaleBody("100"))
	h.AddBill(ctx)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func expectNewAtomicSale(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT.*FROM bill AS b.*WHERE b\.tenant_id = \?.*b\.pos_id = \?.*b\.bill_request_id = \?.*LIMIT 1`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?is)SELECT.*FROM store AS s.*JOIN branches AS b.*JOIN `+"`user`"+` AS u.*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "branch_id", "company_id"}).AddRow(2, 7, 11))
	mock.ExpectQuery(`(?is)SELECT.*FROM product AS p.*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "store_id", "quantity"}).AddRow(10, 2, "5.000"))
	mock.ExpectExec(`(?i)INSERT INTO branch_sequence`).
		WithArgs(sqlmock.AnyArg(), "bill").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`(?i)insert into bill \(`).
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectExec(`(?i)insert into bill_product`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`(?i)UPDATE bill\s+SET\s+total_before_vat`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?i)INSERT INTO bill_payment`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`(?i)SELECT COALESCE\(value, 'disable'\)`).
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("enforce"))
	mock.ExpectQuery(`(?i)SELECT quantity FROM product WHERE id = \? FOR UPDATE`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"quantity"}).AddRow("5.000"))
	mock.ExpectExec(`(?i)UPDATE product\s+SET quantity = quantity \+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?i)INSERT INTO stock_movements`).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func TestAddBillRollsBackPaymentInventoryAndOutboxOnFailure(t *testing.T) {
	h, mock, cleanup := newAtomicSaleTestHandler(t)
	defer cleanup()

	expectNewAtomicSale(mock)
	mock.ExpectExec(`(?i)INSERT INTO outbox`).
		WillReturnError(&mysql.MySQLError{Number: 1456, Message: "outbox unavailable"})
	mock.ExpectRollback()

	ctx, recorder := atomicSaleContext(atomicSaleBody("100"))
	h.AddBill(ctx)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAddBillCommitsCanonicalAtomicSale(t *testing.T) {
	h, mock, cleanup := newAtomicSaleTestHandler(t)
	defer cleanup()

	expectNewAtomicSale(mock)
	mock.ExpectExec(`(?i)INSERT INTO outbox`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	ctx, recorder := atomicSaleContext(atomicSaleBody("100"))
	h.AddBill(ctx)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	var response atomicSaleResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode created response: %v", err)
	}
	if response.ID != 100 || response.TenantID != 42 || response.PosID != "pos-1" ||
		response.BillRequestID != "request-1" || response.Replayed {
		t.Fatalf("created response = %#v", response)
	}
	if response.InvoiceUUID == "" {
		t.Fatal("created response has empty invoice_uuid")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
