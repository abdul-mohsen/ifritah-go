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

// debitRequestID is the bill_request_id that the debit note tests send.
const debitRequestID = "c3d4e5f6-0a1b-4c2d-8e3f-4a5b6c7d8e9f"

// debitTowing is the one charge of the debit note tests: a tow at 5.00 with 15% VAT, a total of 5.75.
const debitTowing = `{"name":"Towing","quantity":"1","price":"5.00","vat":"15.00"}`

// debitBody is a debit note of bill 42 at branch 7, with the given reason, amount and lines.
func debitBody(reason, amount, lines string) string {
	return fmt.Sprintf(`{"bill_id":42,"branch_id":7,"pos_id":"pos-1","bill_request_id":"%s","reason":"%s","amount":"%s","lines":[%s]}`,
		debitRequestID, reason, amount, lines)
}

// debitFingerprintOf is the fingerprint that the handler computes for a debit note body.
func debitFingerprintOf(t *testing.T, body string) string {
	t.Helper()
	var request model.DebitNoteRequest
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatal(err)
	}
	if err := verifyDebitRequest(&request); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := debitFingerprint(request)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

// expectDebitUpToLines is the reads that a debit note makes before its lines are written: the store of the bill, the bill locked with the
// given state, branch and total, and the records of its request id.
func expectDebitUpToLines(mock sqlmock.Sqlmock, state, branch int64, total string) {
	expectNoteBillStore(mock)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, state, branch, total)
	expectNoteRecord(mock, "debit_note", nil)
	expectBillRecord(mock, false)
	expectNoteRecord(mock, "credit_note", nil)
}

// A debit note is verified against its bill, and then its note, its lines and its ZATCA event are written in one transaction.
func TestDebitBillWritesTheNoteItsLinesAndItsEventInOneTransaction(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectDebitUpToLines(mock, 1, 7, "40.25")
	expectNoteInsert(mock, "debit_note", 10)
	mock.ExpectExec("INSERT INTO debit_note_line").WillReturnResult(sqlmock.NewResult(1, 1))
	expectNoteEvent(mock, "acme:7:debit:10", "zatca.debit.acme_db.7")
	mock.ExpectCommit()

	w := runSaleRequest(t, h.DebitBill, "/bill/debit", debitBody("extra service", "5.75", debitTowing), nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "10" {
		t.Fatalf("DebitBill = %d %q; want 201 with note id 10", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A repeat of a recorded debit note returns the original note, and writes nothing.
func TestDebitBillReplaysTheOriginalNoteForTheSameContent(t *testing.T) {
	body := debitBody("extra service", "5.75", debitTowing)
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 7, "40.25")
	expectNoteRecord(mock, "debit_note", &recordedNote{id: 10, billID: 42, fingerprint: debitFingerprintOf(t, body)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.DebitBill, "/bill/debit", body, nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "10" {
		t.Fatalf("DebitBill retry = %d %q; want 201 with the original note id 10", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The same request id with different content is a conflict, and the record is left as it was.
func TestDebitBillRefusesADifferentNoteUnderARecordedRequest(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 7, "40.25")
	expectNoteRecord(mock, "debit_note", &recordedNote{id: 10, billID: 42, fingerprint: strings.Repeat("0", 64)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.DebitBill, "/bill/debit", debitBody("extra service", "5.75", debitTowing), nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), requestConflictCode) {
		t.Fatalf("DebitBill with changed content = %d %q; want 409 %s", w.Code, w.Body.String(), requestConflictCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The amount of a debit note is the total of its lines. A different amount is refused, and nothing is written.
func TestDebitBillRefusesAnAmountThatIsNotTheTotalOfItsLines(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectDebitUpToLines(mock, 1, 7, "40.25")
	mock.ExpectRollback()

	w := runSaleRequest(t, h.DebitBill, "/bill/debit", debitBody("extra service", "5.00", debitTowing), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "amount must be the total of the lines, 5.75") {
		t.Fatalf("DebitBill with the wrong amount = %d %q; want 400 with the total", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A debit note adds an amount above zero. A zero amount is refused before anything is read.
func TestDebitBillRefusesAZeroAmountBeforeAnythingIsRead(t *testing.T) {
	h, mock := newSaleTestHandler(t)

	w := runSaleRequest(t, h.DebitBill, "/bill/debit", debitBody("extra service", "0", debitTowing), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "amount must be above zero") {
		t.Fatalf("DebitBill with a zero amount = %d %q; want 400 about the amount", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A draft has no number, so no note can be written against it.
func TestDebitBillRefusesADraftBill(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 0, 7, "40.25")
	mock.ExpectRollback()

	w := runSaleRequest(t, h.DebitBill, "/bill/debit", debitBody("extra service", "5.75", debitTowing), nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "draft") {
		t.Fatalf("DebitBill on a draft = %d %q; want 409 about the draft", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A bill of another branch cannot be debited from this branch's request.
func TestDebitBillRefusesABillOfAnotherBranch(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 8, "40.25")
	mock.ExpectRollback()

	w := runSaleRequest(t, h.DebitBill, "/bill/debit", debitBody("extra service", "5.75", debitTowing), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "another branch") {
		t.Fatalf("DebitBill on another branch's bill = %d %q; want 400 about the branch", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A debit note cannot reuse the request id of a bill. Nothing is written.
func TestDebitBillRefusesARequestIdThatABillUses(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 7, "40.25")
	expectNoteRecord(mock, "debit_note", nil)
	expectBillRecord(mock, true)
	mock.ExpectRollback()

	w := runSaleRequest(t, h.DebitBill, "/bill/debit", debitBody("extra service", "5.75", debitTowing), nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), requestConflictCode) {
		t.Fatalf("DebitBill with a bill's request id = %d %q; want 409 %s", w.Code, w.Body.String(), requestConflictCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A debit note cannot reuse the request id of a credit note, because the daemon claims request ids for the whole database. Nothing is
// written.
func TestDebitBillRefusesTheRequestIdOfACreditNote(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectNoteBillStore(mock)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectLockedBill(mock, 1, 7, "40.25")
	expectNoteRecord(mock, "debit_note", nil)
	expectBillRecord(mock, false)
	expectNoteRecord(mock, "credit_note", &recordedNote{id: 9, billID: 42, fingerprint: strings.Repeat("0", 64)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.DebitBill, "/bill/debit", debitBody("extra service", "5.75", debitTowing), nil)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), requestConflictCode) {
		t.Fatalf("DebitBill with a credit note's request id = %d %q; want 409 %s", w.Code, w.Body.String(), requestConflictCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A concurrent request that records the same request id first wins. This request rolls back, and it gets the winner's note.
func TestDebitBillReplaysTheWinnerWhenAConcurrentRequestRecordedFirst(t *testing.T) {
	body := debitBody("extra service", "5.75", debitTowing)
	h, mock := newSaleTestHandler(t)
	expectDebitUpToLines(mock, 1, 7, "40.25")
	mock.ExpectExec("INSERT INTO debit_note \\(").WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectRollback()
	expectNoteRecord(mock, "debit_note", &recordedNote{id: 10, billID: 42, fingerprint: debitFingerprintOf(t, body)})

	w := runSaleRequest(t, h.DebitBill, "/bill/debit", body, nil)
	if w.Code != http.StatusCreated || strings.TrimSpace(w.Body.String()) != "10" {
		t.Fatalf("DebitBill that lost a race = %d %q; want 201 with the winner's note id 10", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
