package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
)

func TestPurchaseBillRequestHasPositiveTotal(t *testing.T) {
	tests := []struct {
		name string
		req  model.AddPurchaseBillRequest
		want bool
	}{
		{
			name: "empty bill",
			req:  model.AddPurchaseBillRequest{},
			want: false,
		},
		{
			name: "positive manual item",
			req: model.AddPurchaseBillRequest{
				ManualProducts: []model.PurchaseBillProduct{{
					Name:     "Filter",
					Price:    decimal.NewFromInt(100),
					Quantity: decimal.NewFromInt(1),
				}},
			},
			want: true,
		},
		{
			name: "full discount",
			req: model.AddPurchaseBillRequest{
				Discount: decimal.NewFromInt(100),
				ManualProducts: []model.PurchaseBillProduct{{
					Name:     "Filter",
					Price:    decimal.NewFromInt(100),
					Quantity: decimal.NewFromInt(1),
				}},
			},
			want: false,
		},
		{
			name: "database rounding reduces total to zero",
			req: model.AddPurchaseBillRequest{
				ManualProducts: []model.PurchaseBillProduct{{
					Name:     "Tiny item",
					Price:    decimal.NewFromFloat(0.01),
					Quantity: decimal.NewFromFloat(0.001),
				}},
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := purchaseBillRequestHasPositiveTotal(test.req); got != test.want {
				t.Fatalf("purchaseBillRequestHasPositiveTotal() = %v, want %v (total=%s)", got, test.want, purchaseBillRequestTotal(test.req))
			}
		})
	}
}

func TestPurchaseBillRequestTotalAppliesPercentageDiscount(t *testing.T) {
	request := model.AddPurchaseBillRequest{
		Discount: decimal.NewFromInt(50),
		ManualProducts: []model.PurchaseBillProduct{{
			Name:     "Filter",
			Price:    decimal.NewFromInt(100),
			Quantity: decimal.NewFromInt(1),
		}},
	}

	if got, want := purchaseBillRequestTotal(request).StringFixed(2), "57.50"; got != want {
		t.Fatalf("purchaseBillRequestTotal() = %s, want %s", got, want)
	}
}

func TestAddPurchaseBillRejectsZeroTotalBeforeOpeningTransaction(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("select store.id from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmockRows("id", 1))

	w := runPurchaseBillRequest(t, h.AddPurchaseBill, http.MethodPost, "/api/v2/purchase_bill",
		`{"store_id":1,"products":[],"manual_products":[],"supplier_id":123,"supplier_sequence_number":456,"payment_method":10}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), purchaseBillZeroTotalMessage) {
		t.Fatalf("body = %q, want zero-total message", w.Body.String())
	}
	assertMockExpectations(t, mock)
}

func TestAddPurchaseBillRejectsDiscountToZeroBeforeOpeningTransaction(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("select store.id from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmockRows("id", 1))

	w := runPurchaseBillRequest(t, h.AddPurchaseBill, http.MethodPost, "/api/v2/purchase_bill",
		`{"store_id":1,"products":[],"manual_products":[{"name":"Filter","price":"50","cost_price":"50","quantity":"1"}],"discount":"100","supplier_id":123,"supplier_sequence_number":456,"payment_method":10}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), purchaseBillZeroTotalMessage) {
		t.Fatalf("body = %q, want zero-total message", w.Body.String())
	}
	assertMockExpectations(t, mock)
}

func sqlmockRows(column string, value int32) *sqlmock.Rows {
	return sqlmock.NewRows([]string{column}).AddRow(value)
}
