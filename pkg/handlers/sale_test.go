package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
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

// saleRequestID is a valid bill_request_id: a lowercase UUID.
const saleRequestID = "3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c"

// saleLines is the one catalog product that every test sale sells.
const saleLines = `"products":[{"product_id":null,"name":"Brake pad","price":"10","quantity":"1","discount":"0"}],"manual_products":[]`

// saleBody is a sale of one product at branch 7. A point-of-sale field whose value is empty is left out.
func saleBody(state int, posID, requestID string) string {
	fields := []string{`"store_id":1`, fmt.Sprintf(`"state":%d`, state), `"discount":"0"`, `"maintenance_cost":"0"`, `"payment_method":1`, `"branch_id":7`}
	if posID != "" {
		fields = append(fields, `"pos_id":"`+posID+`"`)
	}
	if requestID != "" {
		fields = append(fields, `"bill_request_id":"`+requestID+`"`)
	}
	return "{" + strings.Join(append(fields, saleLines), ",") + "}"
}

// saleFinalBody is a final sale with its point-of-sale identity. saleDraftBody is the same sale saved as a draft: no identity,
// no number, and no event.
var (
	saleFinalBody = saleBody(1, "pos-1", saleRequestID)
	saleDraftBody = saleBody(0, "", "")
)

// recordedRequest is the bill that already recorded a sale's request id, as the lookup returns it.
type recordedRequest struct {
	id          int64
	fingerprint string
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

// fingerprintOf is the fingerprint the handler computes for a sale body.
func fingerprintOf(t *testing.T, body string) string {
	t.Helper()
	request := newSaleRequest()
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := saleFingerprint(&request)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

// expectSaleStoreAccess is the store check that runs before a sale is written: the store is the caller's, and it is in branch 7, the
// branch that saleBody sells for.
func expectSaleStoreAccess(mock sqlmock.Sqlmock) {
	expectStoreAccess(mock)
	expectStoreBranch(mock, 7)
}

// expectStoreAccess is the caller's access to store 1.
func expectStoreAccess(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("select store.id from store").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
}

// expectStoreBranch is the lookup of the branch of store 1. A nil branch is a store with no branch.
func expectStoreBranch(mock sqlmock.Sqlmock, branch any) {
	mock.ExpectQuery(`SELECT branch_id FROM store WHERE id = \?`).WillReturnRows(sqlmock.NewRows([]string{"branch_id"}).AddRow(branch))
}

// expectRequestRecord is the read of the bill that recorded a sale's request id. A nil record means none.
func expectRequestRecord(mock sqlmock.Sqlmock, record *recordedRequest) {
	rows := sqlmock.NewRows([]string{"id", "request_fingerprint", "request_fingerprint_version"})
	if record != nil {
		rows.AddRow(record.id, record.fingerprint, 1)
	}
	mock.ExpectQuery(`SELECT id, request_fingerprint, request_fingerprint_version FROM bill WHERE pos_id = \? AND bill_request_id = \?`).
		WillReturnRows(rows)
}

// expectBillRow is the bill row that a sale creates before its fields are written.
func expectBillRow(mock sqlmock.Sqlmock, billID int64) {
	mock.ExpectExec("insert into bill \\(").WillReturnResult(sqlmock.NewResult(billID, 1))
}

// expectBillWrite is the update that writes a sale's fields. A new sale and a draft take the same statement.
func expectBillWrite(mock sqlmock.Sqlmock) {
	mock.ExpectExec("UPDATE bill SET").WillReturnResult(sqlmock.NewResult(0, 1))
}

// expectDraftState is the locked read of a draft's state.
func expectDraftState(mock sqlmock.Sqlmock, state int) {
	mock.ExpectQuery(`SELECT state FROM bill WHERE id = \? FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow(state))
}

// expectNumberAllocation is the increment of the branch's bill counter in branch_sequence.
func expectNumberAllocation(mock sqlmock.Sqlmock, number int64) {
	mock.ExpectExec("INSERT INTO branch_sequence").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(number, 1))
}

// expectSaleLines is the product insert, the totals, and the stock setting that a sale reads after its bill.
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
	expectRequestRecord(mock, nil)
	expectNoNoteRequest(mock)
	expectBillRow(mock, 42)
	expectNumberAllocation(mock, 1)
	expectBillWrite(mock)
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
	expectBillRow(mock, 43)
	expectBillWrite(mock)
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

// A store of the sale's branch passes the check, and the sale is written as usual.
func TestAddBillProceedsWhenTheStoreIsInTheSaleBranch(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectStoreAccess(mock)
	expectStoreBranch(mock, 7)
	mock.ExpectBegin()
	expectRequestRecord(mock, nil)
	expectNoNoteRequest(mock)
	expectBillRow(mock, 42)
	expectNumberAllocation(mock, 1)
	expectBillWrite(mock)
	expectSaleLines(mock)
	expectSaleEvent(mock)
	mock.ExpectCommit()

	w := runSaleRequest(t, h.AddBill, "/bill", saleFinalBody, nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "42" {
		t.Fatalf("AddBill with a store of branch 7 = %d %q; want 201 with bill id 42", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A store of another branch is refused before the transaction begins, so nothing is written and no number is taken.
func TestAddBillRefusesAStoreOfAnotherBranchWithNothingWritten(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectStoreAccess(mock)
	expectStoreBranch(mock, 8)

	w := runSaleRequest(t, h.AddBill, "/bill", saleFinalBody, nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "store does not belong to the branch") {
		t.Fatalf("AddBill with a store of branch 8 = %d %q; want 400 about the branch", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A store with no branch is refused before the transaction begins, the same as a store of another branch.
func TestAddBillRefusesAStoreWithNoBranch(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectStoreAccess(mock)
	expectStoreBranch(mock, nil)

	w := runSaleRequest(t, h.AddBill, "/bill", saleFinalBody, nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "store does not belong to the branch") {
		t.Fatalf("AddBill with a store that has no branch = %d %q; want 400 about the branch", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A draft submit takes the same check, so a store of another branch is refused there too, before anything is written.
func TestSubmitDraftBillRefusesAStoreOfAnotherBranchWithNothingWritten(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectStoreAccess(mock)
	expectStoreBranch(mock, 8)

	w := runSaleRequest(t, h.SubmitDraftBill, "/bill/42", saleFinalBody, gin.Params{{Key: "id", Value: "42"}})
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "store does not belong to the branch") {
		t.Fatalf("SubmitDraftBill with a store of branch 8 = %d %q; want 400 about the branch", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A final sale must name its point of sale and its request. Each case is refused before anything is read or written, and each
// case fails on its own validator.
func TestAddBillRefusesAFinalSaleWithoutItsIdentity(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"no pos_id":                  {saleBody(1, "", saleRequestID), "pos_id"},
		"no bill_request_id":         {saleBody(1, "pos-1", ""), "bill_request_id"},
		"upper-case bill_request_id": {saleBody(1, "pos-1", strings.ToUpper(saleRequestID)), "bill_request_id"},
		"pos_id with spaces":         {saleBody(1, " pos-1", saleRequestID), "pos_id"},
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

// A failed counter increment aborts the sale. No number is guessed, and the bill row is rolled back with it.
func TestAddBillRollsBackWhenTheNumberCannotBeAllocated(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectRequestRecord(mock, nil)
	expectNoNoteRequest(mock)
	expectBillRow(mock, 42)
	mock.ExpectExec("INSERT INTO branch_sequence").WillReturnError(errors.New("lock wait timeout"))
	mock.ExpectRollback()

	w := runSaleRequest(t, h.AddBill, "/bill", saleFinalBody, nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("AddBill with a failed counter = %d; want 500", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A retry of a sale that was recorded returns the original bill. It writes nothing: no bill, no number, no event, no stock.
func TestAddBillReplaysTheOriginalSaleForTheSameContent(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectRequestRecord(mock, &recordedRequest{id: 42, fingerprint: fingerprintOf(t, saleFinalBody)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.AddBill, "/bill", saleFinalBody, nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "42" {
		t.Fatalf("AddBill retry = %d %q; want 201 with the original bill id 42", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The same request id with different content is a conflict. The record is left as it was.
func TestAddBillRefusesADifferentSaleUnderARecordedRequest(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectRequestRecord(mock, &recordedRequest{id: 42, fingerprint: strings.Repeat("0", 64)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.AddBill, "/bill", saleFinalBody, nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), requestConflictCode) {
		t.Fatalf("AddBill with changed content = %d %q; want 409 %s", w.Code, w.Body.String(), requestConflictCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Two requests with the same id can race past the check. The second one loses on the unique key, rolls back, and then
// replays the first one's bill, because that bill is committed by the time it reads again.
func TestAddBillReplaysTheWinnerOfARace(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectRequestRecord(mock, nil)
	expectNoNoteRequest(mock)
	expectBillRow(mock, 43)
	expectNumberAllocation(mock, 1)
	mock.ExpectExec("UPDATE bill SET").WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectRollback()
	expectRequestRecord(mock, &recordedRequest{id: 42, fingerprint: fingerprintOf(t, saleFinalBody)})

	w := runSaleRequest(t, h.AddBill, "/bill", saleFinalBody, nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "42" {
		t.Fatalf("AddBill that lost a race = %d %q; want 201 with the winner's bill id 42", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A duplicate whose winner cannot be read again is refused with the duplicate message. Nothing is recorded twice.
func TestAddBillReportsADuplicateWhenTheWinnerCannotBeRead(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectRequestRecord(mock, nil)
	expectNoNoteRequest(mock)
	expectBillRow(mock, 43)
	expectNumberAllocation(mock, 1)
	mock.ExpectExec("UPDATE bill SET").WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectRollback()
	expectRequestRecord(mock, nil)

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
	expectDraftState(mock, 0)
	expectNoNoteRequest(mock)
	expectNumberAllocation(mock, 1)
	expectBillWrite(mock)
	mock.ExpectExec("DELETE FROM bill_product").WillReturnResult(sqlmock.NewResult(0, 1))
	expectSaleLines(mock)
	expectSaleEvent(mock)
	mock.ExpectExec("DELETE FROM bill_payment").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	w := runSaleRequest(t, h.SubmitDraftBill, "/bill/42", saleFinalBody, gin.Params{{Key: "id", Value: "42"}})
	if w.Code != http.StatusCreated {
		t.Fatalf("SubmitDraftBill = %d; want 201", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A submit of a draft that is already final, with a request id nothing recorded, is refused. The bill is never numbered again.
func TestSubmitDraftBillRefusesABillThatIsAlreadyFinal(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectDraftState(mock, 1)
	expectRequestRecord(mock, nil)
	mock.ExpectRollback()

	w := runSaleRequest(t, h.SubmitDraftBill, "/bill/42", saleFinalBody, gin.Params{{Key: "id", Value: "42"}})
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "already final") {
		t.Fatalf("SubmitDraftBill on a final bill = %d %q; want 409 already final", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A submit repeated after it succeeded gets the original result, and no second finalization happens.
func TestSubmitDraftBillReplaysTheSameSubmit(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectDraftState(mock, 1)
	expectRequestRecord(mock, &recordedRequest{id: 42, fingerprint: fingerprintOf(t, saleFinalBody)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.SubmitDraftBill, "/bill/42", saleFinalBody, gin.Params{{Key: "id", Value: "42"}})
	if w.Code != http.StatusCreated {
		t.Fatalf("SubmitDraftBill retry = %d; want 201", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A request id recorded on another bill cannot finalize this draft.
func TestSubmitDraftBillRefusesARequestRecordedForAnotherBill(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectDraftState(mock, 1)
	expectRequestRecord(mock, &recordedRequest{id: 99, fingerprint: fingerprintOf(t, saleFinalBody)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.SubmitDraftBill, "/bill/42", saleFinalBody, gin.Params{{Key: "id", Value: "42"}})
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), requestConflictCode) {
		t.Fatalf("SubmitDraftBill with another bill's request = %d %q; want 409 %s", w.Code, w.Body.String(), requestConflictCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// withPaidAmount is a sale body with a paid_amount added.
func withPaidAmount(body, paid string) string {
	return strings.Replace(body, `"store_id":1,`, `"store_id":1,"paid_amount":"`+paid+`",`, 1)
}

// expectPaymentWrite is the payment of a sale: the bill total is read, SAR is looked up, and the payment row is written.
func expectPaymentWrite(mock sqlmock.Sqlmock, total string) {
	mock.ExpectQuery(`SELECT total FROM bill WHERE id = \?`).WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(total))
	mock.ExpectQuery(`SELECT id FROM currency WHERE code = \?`).WithArgs("SAR").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectExec("INSERT INTO bill_payment").WillReturnResult(sqlmock.NewResult(1, 1))
}

// A payment is written in the sale's transaction, after the lines and totals, and the sale commits with it.
func TestAddBillRecordsThePaymentInTheSaleTransaction(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectRequestRecord(mock, nil)
	expectNoNoteRequest(mock)
	expectBillRow(mock, 42)
	expectNumberAllocation(mock, 1)
	expectBillWrite(mock)
	expectSaleLines(mock)
	expectSaleEvent(mock)
	expectPaymentWrite(mock, "30.00")
	mock.ExpectCommit()

	w := runSaleRequest(t, h.AddBill, "/bill", withPaidAmount(saleFinalBody, "10.00"), nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "42" {
		t.Fatalf("AddBill with a payment = %d %q; want 201 with bill id 42", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A payment above the bill's total is refused, and the sale rolls back with nothing recorded.
func TestAddBillRefusesAPaymentAboveTheTotal(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectRequestRecord(mock, nil)
	expectNoNoteRequest(mock)
	expectBillRow(mock, 42)
	expectNumberAllocation(mock, 1)
	expectBillWrite(mock)
	expectSaleLines(mock)
	expectSaleEvent(mock)
	mock.ExpectQuery(`SELECT total FROM bill WHERE id = \?`).WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow("10.00"))
	mock.ExpectRollback()

	w := runSaleRequest(t, h.AddBill, "/bill", withPaidAmount(saleFinalBody, "25.00"), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "more than the bill total") {
		t.Fatalf("AddBill above the total = %d %q; want 400 about the total", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A negative amount, or one with more than two decimal places, is refused before anything is read or written.
func TestAddBillRefusesAnInvalidPaidAmount(t *testing.T) {
	cases := map[string]string{"negative": "-1.00", "three decimals": "10.005"}
	for name, paid := range cases {
		t.Run(name, func(t *testing.T) {
			h, mock := newSaleTestHandler(t)
			w := runSaleRequest(t, h.AddBill, "/bill", withPaidAmount(saleFinalBody, paid), nil)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "paid_amount") {
				t.Fatalf("AddBill with paid %s = %d %q; want 400 naming paid_amount", paid, w.Code, w.Body.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A draft's payment is replaced when the draft is submitted, so the payment is recorded once.
func TestSubmitDraftBillReplacesTheEarlierPayment(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectDraftState(mock, 0)
	expectNoNoteRequest(mock)
	expectNumberAllocation(mock, 1)
	expectBillWrite(mock)
	mock.ExpectExec("DELETE FROM bill_product").WillReturnResult(sqlmock.NewResult(0, 1))
	expectSaleLines(mock)
	expectSaleEvent(mock)
	mock.ExpectExec("DELETE FROM bill_payment").WillReturnResult(sqlmock.NewResult(0, 1))
	expectPaymentWrite(mock, "30.00")
	mock.ExpectCommit()

	w := runSaleRequest(t, h.SubmitDraftBill, "/bill/42", withPaidAmount(saleFinalBody, "10.00"), gin.Params{{Key: "id", Value: "42"}})
	if w.Code != http.StatusCreated {
		t.Fatalf("SubmitDraftBill with a payment = %d; want 201", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A final sale is refused when its lines cannot be reported: no lines, a line with no quantity, or a negative price. Nothing is read or
// written, so the cashier sees the reason and can fix the sale.
func TestAddBillRefusesAFinalSaleThatCannotBeVerified(t *testing.T) {
	const head = `{"store_id":1,"state":1,"discount":"0","maintenance_cost":"0","payment_method":1,"branch_id":7,"pos_id":"pos-1","bill_request_id":"` + saleRequestID + `",`
	cases := map[string]struct{ body, want string }{
		"no lines":       {head + `"products":[],"manual_products":[]}`, "at least one line"},
		"zero quantity":  {head + `"products":[{"product_id":null,"name":"Brake pad","price":"10","quantity":"0","discount":"0"}],"manual_products":[]}`, "quantity must be positive"},
		"negative price": {head + `"products":[{"product_id":null,"name":"Brake pad","price":"-10","quantity":"1","discount":"0"}],"manual_products":[]}`, "price must not be negative"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, mock := newSaleTestHandler(t)
			w := runSaleRequest(t, h.AddBill, "/bill", tc.body, nil)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("AddBill = %d %q; want 400 about %q", w.Code, w.Body.String(), tc.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
