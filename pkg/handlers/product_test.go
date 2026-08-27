package handlers

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAddQuantityAcceptsFreeTextProduct(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("select store.id from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))
	mock.ExpectExec("INSERT INTO product").
		WithArgs(nil, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "A1", int32(1), "Generic brake pad").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"store_id":1,"products":[{"name":"  Generic brake pad ","quantity":"2","price":"15.50","cost_price":"10.00","shelf_number":"A1"}]}`
	w := runPurchaseBillRequest(t, h.AddQuantity, http.MethodPost, "/api/v2/product", body)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	assertMockExpectations(t, mock)
}

func TestAddQuantityRejectsProductWithoutIDOrName(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("select store.id from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int32(1)))

	body := `{"store_id":1,"products":[{"quantity":"1","price":"15.50","cost_price":"10.00"}]}`
	w := runPurchaseBillRequest(t, h.AddQuantity, http.MethodPost, "/api/v2/product", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	assertMockExpectations(t, mock)
}
