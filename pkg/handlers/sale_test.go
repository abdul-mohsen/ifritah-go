package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
)

// saleFinalBody is a final sale of one catalog product at branch 7, with the point-of-sale identity.
const saleFinalBody = `{"store_id":1,"state":1,"discount":"0","maintenance_cost":"0","payment_method":1,"branch_id":7,` +
	`"pos_id":"pos-1","bill_request_id":"3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c",` +
	`"products":[{"product_id":null,"name":"Brake pad","price":"10","quantity":"1","discount":"0"}],"manual_products":[]}`

// saleDraftBody is the same sale saved as a draft: no identity, no number, no event.
const saleDraftBody = `{"store_id":1,"state":0,"discount":"0","maintenance_cost":"0","payment_method":1,"branch_id":7,` +
	`"products":[{"product_id":null,"name":"Brake pad","price":"10","quantity":"1","discount":"0"}],"manual_products":[]}`

// saleRequestID is a valid bill_request_id: a lowercase UUID.
const saleRequestID = "3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c"

// saleIdentityBody is a final sale of one catalog product with the given identity. An empty value leaves its
// field out of the request.
func saleIdentityBody(posID, requestID string) string {
	fields := []string{`"store_id":1`, `"state":1`, `"discount":"0"`, `"maintenance_cost":"0"`, `"payment_method":1`, `"branch_id":7`}
	if posID != "" {
		fields = append(fields, `"pos_id":"`+posID+`"`)
	}
	if requestID != "" {
		fields = append(fields, `"bill_request_id":"`+requestID+`"`)
	}
	fields = append(fields, `"products":[{"product_id":null,"name":"Brake pad","price":"10","quantity":"1","discount":"0"}]`, `"manual_products":[]`)
	return "{" + strings.Join(fields, ",") + "}"
}

func newSaleTestHandler(t *testing.T) (*handler, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	h := New(sqlDB, db.New(sqlDB), &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"})
	return &h, mock
}

func runSaleRequest(t *testing.T, handlerFunc gin.HandlerFunc, path, body string, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Params = params
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "cashier"})
	handlerFunc(c)
	c.Writer.WriteHeaderNow()
	return w
}

// expectSaleStoreAccess is the store check that runs before a sale is written.
func expectSaleStoreAccess(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("select store.id from store").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
}

// expectNumberAllocation is the increment of the branch's bill counter in branch_sequence.
func expectNumberAllocation(mock sqlmock.Sqlmock, number int64) {
	mock.ExpectExec("INSERT INTO branch_sequence").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(number, 1))
}

// expectSaleLines is the product insert, the totals and the stock lookup that a sale runs after its bill.
func expectSaleLines(mock sqlmock.Sqlmock) {
	mock.ExpectExec("insert into bill_product").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE bill\s+SET\s+total_before_vat`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT COALESCE\(value, 'disable'\)`).
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("disable"))
}

// expectSaleEvent is the outbox row of the sale's ZATCA event, written in the sale's transaction.
func expectSaleEvent(mock sqlmock.Sqlmock) {
	mock.ExpectExec("INSERT INTO outbox").
		WithArgs(sqlmock.AnyArg(), "acme:7:bill:42", "zatca.bill.acme_db.7", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func TestAddBillWritesTheSaleAndItsZatcaEventInOneTransaction(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectNumberAllocation(mock, 1)
	mock.ExpectExec("insert into bill \\(").WillReturnResult(sqlmock.NewResult(42, 1))
	expectSaleLines(mock)
	expectSaleEvent(mock)
	mock.ExpectCommit()

	w := runSaleRequest(t, h.AddBill, "/bill", saleFinalBody, nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "42" {
		t.Fatalf("AddBill = %d %q; want 201 with bill id 42", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAddBillSavesADraftWithoutANumberOrAnEvent(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	mock.ExpectExec("insert into bill \\(").WillReturnResult(sqlmock.NewResult(43, 1))
	expectSaleLines(mock)
	mock.ExpectCommit()

	w := runSaleRequest(t, h.AddBill, "/bill", saleDraftBody, nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "43" {
		t.Fatalf("AddBill draft = %d %q; want 201 with bill id 43", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A final sale must name its point of sale and its request. Each case is refused before anything is read or
// written, and each case fails on its own validator.
func TestAddBillRefusesAFinalSaleWithoutItsIdentity(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"no pos_id":                  {saleIdentityBody("", saleRequestID), "pos_id"},
		"no bill_request_id":         {saleIdentityBody("pos-1", ""), "bill_request_id"},
		"upper-case bill_request_id": {saleIdentityBody("pos-1", strings.ToUpper(saleRequestID)), "bill_request_id"},
		"pos_id with spaces":         {saleIdentityBody(" pos-1", saleRequestID), "pos_id"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, mock := newSaleTestHandler(t)
			w := runSaleRequest(t, h.AddBill, "/bill", tc.body, nil)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("AddBill = %d %q; want 400 naming %s", w.Code, w.Body.String(), tc.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A failed counter increment aborts the sale. No bill is inserted and no number is guessed.
func TestAddBillRollsBackWhenTheNumberCannotBeAllocated(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO branch_sequence").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errors.New("lock wait timeout"))
	mock.ExpectRollback()

	w := runSaleRequest(t, h.AddBill, "/bill", saleFinalBody, nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("AddBill with a failed counter = %d; want 500", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A retry with the same pos_id and bill_request_id hits the unique key. It gets 409 and no second bill, event
// or stock movement.
func TestAddBillReportsADuplicateSaleRequest(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectNumberAllocation(mock, 1)
	mock.ExpectExec("insert into bill \\(").WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.AddBill, "/bill", saleFinalBody, nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), duplicateSaleMessage) {
		t.Fatalf("AddBill duplicate = %d %q; want 409 with the duplicate message", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSubmitDraftBillFinalizesTheBillWithItsZatcaEvent(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT state FROM bill WHERE id = \? FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow(0))
	expectNumberAllocation(mock, 1)
	mock.ExpectExec("UPDATE bill SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM bill_product").WillReturnResult(sqlmock.NewResult(0, 1))
	expectSaleLines(mock)
	expectSaleEvent(mock)
	mock.ExpectCommit()

	w := runSaleRequest(t, h.SubmitDraftBill, "/bill/42", saleFinalBody, gin.Params{{Key: "id", Value: "42"}})
	if w.Code != http.StatusCreated {
		t.Fatalf("SubmitDraftBill = %d; want 201", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A bill that is already final is never numbered again, so its counter is not touched.
func TestSubmitDraftBillRefusesABillThatIsAlreadyFinal(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT state FROM bill WHERE id = \? FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow(1))
	mock.ExpectRollback()

	w := runSaleRequest(t, h.SubmitDraftBill, "/bill/42", saleFinalBody, gin.Params{{Key: "id", Value: "42"}})
	if w.Code != http.StatusConflict {
		t.Fatalf("SubmitDraftBill on a final bill = %d; want 409", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Credit notes are disabled until their ZATCA submission uses the outbox (B6). The request is refused before
// anything is read or written.
func TestCreditBillIsDisabledUntilItUsesTheOutbox(t *testing.T) {
	h, mock := newSaleTestHandler(t)

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", `{"bill_id":42,"note":"return"}`, nil)
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("CreditBill = %d; want 501", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
