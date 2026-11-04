package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"

	"ifritah/web-service-gin/pkg/model"
)

// creditRequestID is the bill_request_id that the credit note tests send.
const creditRequestID = "5e1a7c2d-9b3f-4c8e-a6d1-2f0b9c4e7a13"

// creditBrakePad credits one of the two brake pads of bill 42, at the price and VAT rate of its bill line.
const creditBrakePad = `{"bill_product_id":5,"quantity":"1","price":"10.00","vat":"15.00"}`

// creditBody is a credit note of bill 42 at branch 7, with the given reason, amount and lines.
func creditBody(reason, amount, lines string) string {
	return fmt.Sprintf(`{"bill_id":42,"branch_id":7,"pos_id":"pos-1","bill_request_id":"%s","reason":"%s","amount":"%s","lines":[%s]}`,
		creditRequestID, reason, amount, lines)
}

// creditFingerprintOf is the fingerprint that the handler computes for a credit note body.
func creditFingerprintOf(t *testing.T, body string) string {
	t.Helper()
	var request model.CreditNoteRequest
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatal(err)
	}
	if err := verifyCreditRequest(&request); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := creditFingerprint(request)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

// recordedNote is a note that recorded a request id, as the lookup returns it.
type recordedNote struct {
	id, billID  int64
	fingerprint string
}

// expectNoteBillStore is the read of the store of the bill that a note refers to. It runs before the transaction.
func expectNoteBillStore(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT store_id FROM bill WHERE id = \?`).WillReturnRows(sqlmock.NewRows([]string{"store_id"}).AddRow(1))
}

// expectLockedBill is the locked read of the bill that a note is verified against.
func expectLockedBill(mock sqlmock.Sqlmock, state, branch int64, total string) {
	mock.ExpectQuery(`SELECT id, state, branch_id, store_id, total FROM bill WHERE id = \? FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "state", "branch_id", "store_id", "total"}).AddRow(42, state, branch, 1, total))
}

// expectNoteRecord is the read of the note of one kind that recorded a request id. A nil record means none.
func expectNoteRecord(mock sqlmock.Sqlmock, table string, record *recordedNote) {
	rows := sqlmock.NewRows([]string{"id", "bill_id", "request_fingerprint", "request_fingerprint_version"})
	if record != nil {
		rows.AddRow(record.id, record.billID, record.fingerprint, 1)
	}
	mock.ExpectQuery(`SELECT id, bill_id, request_fingerprint, request_fingerprint_version FROM ` + table + ` WHERE pos_id = \? AND bill_request_id = \?`).
		WillReturnRows(rows)
}

// expectBillRecord is the read of a bill that recorded the request id. found says whether one did.
func expectBillRecord(mock sqlmock.Sqlmock, found bool) {
	rows := sqlmock.NewRows([]string{"id", "request_fingerprint", "request_fingerprint_version"})
	if found {
		rows.AddRow(42, strings.Repeat("0", 64), 1)
	}
	mock.ExpectQuery(`SELECT id, request_fingerprint, request_fingerprint_version FROM bill WHERE pos_id = \? AND bill_request_id = \?`).
		WillReturnRows(rows)
}

// expectBillLines is the read of the lines of bill 42: a brake pad, two at 10.00, and a maintenance cost of 15.00. Both lines carry 15% VAT.
// The bill total is 40.25.
func expectBillLines(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT id, product_id, name, price, quantity, discount, vat FROM bill_product WHERE bill_id = \?`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "name", "price", "quantity", "discount", "vat"}).
			AddRow(5, 7, "Brake pad", "10.00", "2", "0.00", "15.00").
			AddRow(6, nil, "maintenance_cost", "15.00", "1", "0.00", "15.00"))
}

// expectCreditHistory is the read of what earlier notes recorded for bill 42. Bill 42 has no earlier credit lines, and the amounts are
// the amounts of its earlier notes.
func expectCreditHistory(mock sqlmock.Sqlmock, amounts ...string) {
	mock.ExpectQuery(`FROM credit_note_line cl`).WillReturnRows(sqlmock.NewRows([]string{"bill_product_id", "quantity"}))
	rows := sqlmock.NewRows([]string{"amount"})
	for _, amount := range amounts {
		rows.AddRow(amount)
	}
	mock.ExpectQuery(`SELECT amount FROM credit_note WHERE bill_id = \? FOR UPDATE`).WillReturnRows(rows)
}

// expectNoteInsert is the insert of the note row, which takes the given id.
func expectNoteInsert(mock sqlmock.Sqlmock, table string, id int64) {
	mock.ExpectExec(`INSERT INTO ` + table + ` \(`).WillReturnResult(sqlmock.NewResult(id, 1))
}

// expectStockMode is the read of the stock enforcement mode in the settings.
func expectStockMode(mock sqlmock.Sqlmock, mode string) {
	mock.ExpectQuery(`SELECT COALESCE\(value, 'disable'\)`).WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(mode))
}

// expectStockRestore is the increase of a catalog product's stock, and the movement that records it.
func expectStockRestore(mock sqlmock.Sqlmock) {
	mock.ExpectExec(`UPDATE product\s+SET quantity = quantity \+ \?`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO stock_movements`).WillReturnResult(sqlmock.NewResult(1, 1))
}

// expectNoteEvent is the outbox row of the note's ZATCA event, written in the note's transaction.
func expectNoteEvent(mock sqlmock.Sqlmock, key, subject string) {
	mock.ExpectExec("INSERT INTO outbox").
		WithArgs(sqlmock.AnyArg(), key, subject, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

// expectCreditUpToLines is the reads that a credit note makes before its lines are checked: the store of the bill, the bill locked with the
// given state, branch and total, and the records of its request id.
func expectCreditUpToLines(mock sqlmock.Sqlmock, state, branch int64, total string) {
	expectNoteBillStore(mock)
	expectStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, state, branch, total)
	expectNoteRecord(mock, "credit_note", nil)
	expectBillRecord(mock, false)
	expectNoteRecord(mock, "debit_note", nil)
}

func TestCreditBillWritesTheNoteItsLinesAndItsEventInOneTransaction(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectCreditUpToLines(mock, 1, 7, "40.25")
	expectBillLines(mock)
	expectCreditHistory(mock)
	expectNoteInsert(mock, "credit_note", 9)
	mock.ExpectExec("INSERT INTO credit_note_line").WillReturnResult(sqlmock.NewResult(1, 1))
	expectStockMode(mock, "disable")
	expectNoteEvent(mock, "acme:7:credit:9", "zatca.credit.acme_db.7")
	mock.ExpectCommit()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "9" {
		t.Fatalf("CreditBill = %d %q; want 201 with note id 9", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A repeat of a recorded credit note returns the original note, and writes nothing.
func TestCreditBillReplaysTheOriginalNoteForTheSameContent(t *testing.T) {
	body := creditBody("returned", "11.50", creditBrakePad)
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 7, "40.25")
	expectNoteRecord(mock, "credit_note", &recordedNote{id: 9, billID: 42, fingerprint: creditFingerprintOf(t, body)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", body, nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "9" {
		t.Fatalf("CreditBill retry = %d %q; want 201 with the original note id 9", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The same request id with different content is a conflict, and the record is left as it was.
func TestCreditBillRefusesADifferentNoteUnderARecordedRequest(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 7, "40.25")
	expectNoteRecord(mock, "credit_note", &recordedNote{id: 9, billID: 42, fingerprint: strings.Repeat("0", 64)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), requestConflictCode) {
		t.Fatalf("CreditBill with changed content = %d %q; want 409 %s", w.Code, w.Body.String(), requestConflictCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The same request id on another bill is a conflict too, even when the content is the same.
func TestCreditBillRefusesTheRequestIdOfANoteOnAnotherBill(t *testing.T) {
	body := creditBody("returned", "11.50", creditBrakePad)
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 7, "40.25")
	expectNoteRecord(mock, "credit_note", &recordedNote{id: 9, billID: 43, fingerprint: creditFingerprintOf(t, body)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", body, nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), requestConflictCode) {
		t.Fatalf("CreditBill on another bill = %d %q; want 409 %s", w.Code, w.Body.String(), requestConflictCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A request id that a bill already uses cannot be used by a note, because the daemon claims request ids for the whole database.
func TestCreditBillRefusesARequestIdThatABillUses(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 7, "40.25")
	expectNoteRecord(mock, "credit_note", nil)
	expectBillRecord(mock, true)
	mock.ExpectRollback()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), requestConflictCode) {
		t.Fatalf("CreditBill with a bill's request id = %d %q; want 409 %s", w.Code, w.Body.String(), requestConflictCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A credit note cannot reuse the request id of a debit note. The daemon claims request ids for the whole database, so the note would
// never be invoiced. Nothing is written.
func TestCreditBillRefusesTheRequestIdOfADebitNote(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 7, "40.25")
	expectNoteRecord(mock, "credit_note", nil)
	expectBillRecord(mock, false)
	expectNoteRecord(mock, "debit_note", &recordedNote{id: 10, billID: 42, fingerprint: strings.Repeat("0", 64)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), requestConflictCode) {
		t.Fatalf("CreditBill with a debit note's request id = %d %q; want 409 %s", w.Code, w.Body.String(), requestConflictCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Two notes on one bill may credit it in turn, but not beyond its remaining amount. Bill 42 has 40.25, and 40.00 is already credited.
func TestCreditBillRefusesMoreThanTheBillsRemainingAmount(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectCreditUpToLines(mock, 1, 7, "40.25")
	expectBillLines(mock)
	expectCreditHistory(mock, "40.00")
	mock.ExpectRollback()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "remaining amount") {
		t.Fatalf("CreditBill over the remaining amount = %d %q; want 400 about the remaining amount", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A note without a reason is refused before anything is read or written.
func TestCreditBillRefusesANoteWithoutAReason(t *testing.T) {
	h, mock := newSaleTestHandler(t)

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("   ", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "reason is required") {
		t.Fatalf("CreditBill without a reason = %d %q; want 400 naming the reason", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A draft has no number, so nothing can be credited against it. The bill is locked, the note is refused, and nothing is written.
func TestCreditBillRefusesADraftBill(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 0, 7, "40.25")
	mock.ExpectRollback()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "draft") {
		t.Fatalf("CreditBill on a draft = %d %q; want 409 about the draft", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A bill of another branch cannot be credited from this branch's request.
func TestCreditBillRefusesABillOfAnotherBranch(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 8, "40.25")
	mock.ExpectRollback()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "another branch") {
		t.Fatalf("CreditBill on another branch's bill = %d %q; want 400 about the branch", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A deleted bill is not found, and it cannot take a note. No transaction is written for it.
func TestCreditBillReportsADeletedBillAsNotFound(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, -1, 7, "40.25")
	mock.ExpectRollback()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("CreditBill on a deleted bill = %d %q; want 404", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A bill that does not exist is not found, and no transaction starts.
func TestCreditBillReportsAnUnknownBill(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	mock.ExpectQuery(`SELECT store_id FROM bill WHERE id = \?`).WillReturnRows(sqlmock.NewRows([]string{"store_id"}))

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("CreditBill on an unknown bill = %d; want 404", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A line must credit a line of the bill, at the bill's price and VAT rate, and no further than the quantity the bill line has left. Each
// case is refused after the bill has been read and before anything is written.
func TestCreditBillRefusesALineThatDoesNotMatchTheBill(t *testing.T) {
	cases := map[string]struct {
		lines, amount, want string
	}{
		"not a line of the bill": {`{"bill_product_id":99,"quantity":"1","price":"10.00","vat":"15.00"}`, "11.50", "is not a line of this bill"},
		"another price":          {`{"bill_product_id":5,"quantity":"1","price":"12.00","vat":"15.00"}`, "13.80", "price must be the bill line's price"},
		"another VAT rate":       {`{"bill_product_id":5,"quantity":"1","price":"10.00","vat":"5.00"}`, "10.50", "VAT rate must be the bill line's rate"},
		"more than is left":      {`{"bill_product_id":5,"quantity":"3","price":"10.00","vat":"15.00"}`, "34.50", "quantity is more than"},
		"amount not the total":   {creditBrakePad, "11.00", "amount must be the total of the lines"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, mock := newSaleTestHandler(t)
			expectCreditUpToLines(mock, 1, 7, "40.25")
			expectBillLines(mock)
			expectCreditHistory(mock)
			mock.ExpectRollback()

			w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", tc.amount, tc.lines), nil)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("CreditBill = %d %q; want 400 containing %q", w.Code, w.Body.String(), tc.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A concurrent request that records the same request id first wins. This request rolls back, and it gets the winner's note.
func TestCreditBillReplaysTheWinnerWhenAConcurrentRequestRecordedFirst(t *testing.T) {
	body := creditBody("returned", "11.50", creditBrakePad)
	h, mock := newSaleTestHandler(t)
	expectCreditUpToLines(mock, 1, 7, "40.25")
	expectBillLines(mock)
	expectCreditHistory(mock)
	mock.ExpectExec("INSERT INTO credit_note \\(").WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectRollback()
	expectNoteRecord(mock, "credit_note", &recordedNote{id: 9, billID: 42, fingerprint: creditFingerprintOf(t, body)})

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", body, nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "9" {
		t.Fatalf("CreditBill that lost a race = %d %q; want 201 with the winner's note id 9", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// With stock tracking on, a credited catalog line puts its quantity back into stock, and records the movement against the note.
func TestCreditBillPutsTheStockOfACatalogLineBack(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectCreditUpToLines(mock, 1, 7, "40.25")
	expectBillLines(mock)
	expectCreditHistory(mock)
	expectNoteInsert(mock, "credit_note", 9)
	mock.ExpectExec("INSERT INTO credit_note_line").WillReturnResult(sqlmock.NewResult(1, 1))
	expectStockMode(mock, "enforce")
	expectStockRestore(mock)
	expectNoteEvent(mock, "acme:7:credit:9", "zatca.credit.acme_db.7")
	mock.ExpectCommit()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", creditBody("returned", "11.50", creditBrakePad), nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreditBill with stock = %d %q; want 201", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A line without a catalog product has no stock, so the stock setting is not read for it.
func TestCreditBillWritesNoStockMovementForALineWithoutAProduct(t *testing.T) {
	body := creditBody("maintenance refund", "17.25", `{"bill_product_id":6,"quantity":"1","price":"15.00","vat":"15.00"}`)
	h, mock := newSaleTestHandler(t)
	expectCreditUpToLines(mock, 1, 7, "40.25")
	expectBillLines(mock)
	expectCreditHistory(mock)
	expectNoteInsert(mock, "credit_note", 9)
	mock.ExpectExec("INSERT INTO credit_note_line").WillReturnResult(sqlmock.NewResult(1, 1))
	expectNoteEvent(mock, "acme:7:credit:9", "zatca.credit.acme_db.7")
	mock.ExpectCommit()

	w := runSaleRequest(t, h.CreditBill, "/bill/credit", body, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreditBill of a maintenance line = %d %q; want 201", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
