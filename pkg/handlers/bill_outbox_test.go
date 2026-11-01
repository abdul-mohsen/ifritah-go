package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

// billSaleBody is a final sale of one catalog product, at store 1 and branch 7.
const billSaleBody = `{"store_id":1,"state":1,"discount":"0","maintenance_cost":"0","payment_method":1,"branch_id":7,` +
	`"products":[{"product_id":null,"name":"Brake pad","price":"10","quantity":"1","discount":"0"}],"manual_products":[]}`

// newBillEventTestHandler returns a handler on a mock database. Its publisher has the tenant config of
// a test server, so the events it writes have known keys and subjects.
func newBillEventTestHandler(t *testing.T) (*handler, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	h := New(sqlDB, db.New(sqlDB), &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"})
	return &h, mock
}

func runBillRequest(t *testing.T, handlerFunc gin.HandlerFunc, method, path, body string, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Params = params
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "cashier"})
	handlerFunc(c)
	c.Writer.WriteHeaderNow()
	return w
}

// expectSaleHead sets up the statements that a sale runs before it writes the bill: the store check,
// the transaction, and the sequence number.
func expectSaleHead(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("select store.id from store").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery("max\\(sequence_number\\)").WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(10))
}

// expectSaleTail sets up the statements after the bill's product, and the outbox row that carries the
// ZATCA event for bill 42 at branch 7. outboxErr is what that insert returns. nil means the sale
// commits. An error means the sale rolls back.
func expectSaleTail(mock sqlmock.Sqlmock, outboxErr error) {
	mock.ExpectExec("insert into bill_product").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE bill\\s+SET\\s+total_before_vat").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT COALESCE\\(value, 'disable'\\)").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("disable"))
	insert := mock.ExpectExec("INSERT INTO outbox").
		WithArgs(sqlmock.AnyArg(), "acme:7:bill:42", "zatca.bill.acme_db.7", sqlmock.AnyArg(), sqlmock.AnyArg())
	if outboxErr != nil {
		insert.WillReturnError(outboxErr)
		mock.ExpectRollback()
		return
	}
	insert.WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
}

func TestBillEventIsKeyedByTheConfiguredTenant(t *testing.T) {
	p := &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"}
	event, err := p.BillEvent(42, 7)
	if err != nil {
		t.Fatal(err)
	}
	if event.DocumentKey != "acme:7:bill:42" || event.Subject != "zatca.bill.acme_db.7" {
		t.Fatalf("event key %q and subject %q; want acme:7:bill:42 and zatca.bill.acme_db.7", event.DocumentKey, event.Subject)
	}
	if got, want := string(event.Payload), `{"doc_type":"bill","id":42,"branch_id":7,"db_name":"acme_db"}`; got != want {
		t.Fatalf("payload %s; want %s", got, want)
	}
}

// A sale writes the ZATCA event in its own transaction, after the bill and its products, and before the
// commit. The event is published only after the commit, by the drainer.
func TestAddBillWritesTheZatcaEventInTheSaleTransaction(t *testing.T) {
	h, mock := newBillEventTestHandler(t)
	expectSaleHead(mock)
	mock.ExpectExec("insert into bill \\(").WillReturnResult(sqlmock.NewResult(42, 1))
	expectSaleTail(mock, nil)

	w := runBillRequest(t, h.AddBill, http.MethodPost, "/bill", billSaleBody, nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "42" {
		t.Fatalf("AddBill = %d %q; want 201 with bill id 42", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSubmitDraftBillWritesTheZatcaEventInTheSaleTransaction(t *testing.T) {
	h, mock := newBillEventTestHandler(t)
	expectSaleHead(mock)
	mock.ExpectExec("UPDATE bill SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM bill_product").WillReturnResult(sqlmock.NewResult(0, 1))
	expectSaleTail(mock, nil)

	w := runBillRequest(t, h.SubmitDraftBill, http.MethodPut, "/bill/42", billSaleBody, gin.Params{{Key: "id", Value: "42"}})
	if w.Code != http.StatusCreated {
		t.Fatalf("SubmitDraftBill = %d; want 201", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// When the event cannot be written, the sale must not commit. Otherwise a bill could exist without
// the submission that ZATCA needs.
func TestAddBillRollsBackWhenTheZatcaEventCannotBeWritten(t *testing.T) {
	h, mock := newBillEventTestHandler(t)
	expectSaleHead(mock)
	mock.ExpectExec("insert into bill \\(").WillReturnResult(sqlmock.NewResult(42, 1))
	expectSaleTail(mock, errors.New("outbox unavailable"))

	w := runBillRequest(t, h.AddBill, http.MethodPost, "/bill", billSaleBody, nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("AddBill = %d; want 500 when the event cannot be written", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
