package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

// requireNoteConflict fails unless the response refuses a request identity that a note holds, with the body that the note side uses.
func requireNoteConflict(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), requestConflictCode) || !strings.Contains(w.Body.String(), noteConflictMessage) {
		t.Fatalf("status %d %q; want 409 with %s and the note conflict message", w.Code, w.Body.String(), requestConflictCode)
	}
}

// expectNoNoteRequest is the read of the credit notes and then the debit notes, which a sale's request identity is checked against
// when neither holds it.
func expectNoNoteRequest(mock sqlmock.Sqlmock) {
	expectNoteRecord(mock, "credit_note", nil)
	expectNoteRecord(mock, "debit_note", nil)
}

// A sale that carries the identity of a credit note is refused before its bill row is inserted. The transaction rolls back. The
// expectations end with the rollback, so any other statement would fail the test.
func TestAddBillRefusesAnIdentityHeldByACreditNote(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectRequestRecord(mock, nil)
	expectNoteRecord(mock, "credit_note", &recordedNote{id: 9, billID: 42, fingerprint: strings.Repeat("0", 64)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.AddBill, "/bill", saleBody(1, "pos-1", creditRequestID), nil)
	requireNoteConflict(t, w)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A sale that carries the identity of a debit note is refused the same way. The credit notes are read first and hold nothing.
func TestAddBillRefusesAnIdentityHeldByADebitNote(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectRequestRecord(mock, nil)
	expectNoteRecord(mock, "credit_note", nil)
	expectNoteRecord(mock, "debit_note", &recordedNote{id: 10, billID: 42, fingerprint: strings.Repeat("0", 64)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.AddBill, "/bill", saleBody(1, "pos-1", debitRequestID), nil)
	requireNoteConflict(t, w)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A draft that is finalized with an identity a note holds is refused before its number is taken and before its identity is written.
func TestSubmitDraftBillRefusesAnIdentityHeldByANote(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectSaleStoreAccess(mock)
	mock.ExpectBegin()
	expectDraftState(mock, 0)
	expectNoteRecord(mock, "credit_note", &recordedNote{id: 9, billID: 42, fingerprint: strings.Repeat("0", 64)})
	mock.ExpectRollback()

	w := runSaleRequest(t, h.SubmitDraftBill, "/bill/42", saleBody(1, "pos-1", creditRequestID), gin.Params{{Key: "id", Value: "42"}})
	requireNoteConflict(t, w)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
