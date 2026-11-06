package handlers

import (
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

// A final sale is an issued document. Deleting it is refused inside the transaction, before any stock is put back, and nothing
// is deleted.
func TestDeleteBillRefusesAFinalSale(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT state FROM bill WHERE id = \?`).WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow(1))
	mock.ExpectRollback()

	w := runSaleRequest(t, h.DeleteBillDetail, "/bill/42", "", gin.Params{{Key: "id", Value: "42"}})
	if w.Code != http.StatusConflict {
		t.Fatalf("DeleteBillDetail on a final sale = %d; want 409", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
