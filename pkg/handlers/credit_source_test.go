package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"ifritah/web-service-gin/pkg/model"
)

type creditSourceTestResponse struct {
	BillID   uint64                 `json:"bill_id"`
	BranchID uint32                 `json:"branch_id"`
	Amount   string                 `json:"amount"`
	Lines    []creditSourceTestLine `json:"lines"`
	Message  string                 `json:"message"`
}

type creditSourceTestLine struct {
	BillProductID int32  `json:"bill_product_id"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price"`
	VAT           string `json:"vat"`
}

func runCreditSourceRequest(t *testing.T, handlerFunc gin.HandlerFunc, id string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v2/bill/"+id+"/credit-source", nil)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "cashier"})
	handlerFunc(c)
	c.Writer.WriteHeaderNow()
	return w
}

func expectCreditSourceBill(mock sqlmock.Sqlmock, state int64, branch any, total string) {
	mock.ExpectQuery(`SELECT store_id FROM bill WHERE id = \?`).
		WillReturnRows(sqlmock.NewRows([]string{"store_id"}).AddRow(1))
	expectStoreAccess(mock)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, state, branch_id, store_id, total FROM bill WHERE id = \? FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "state", "branch_id", "store_id", "total"}).
			AddRow(42, state, branch, 1, total))
	expectStoreAccess(mock)
}

func expectCreditSourceHistory(mock sqlmock.Sqlmock, credited []creditSourceTestCredit, amounts ...string) {
	creditRows := sqlmock.NewRows([]string{"bill_product_id", "quantity"})
	for _, row := range credited {
		creditRows.AddRow(row.billProductID, row.quantity)
	}
	mock.ExpectQuery(`FROM credit_note_line cl`).WillReturnRows(creditRows)

	amountRows := sqlmock.NewRows([]string{"amount"})
	for _, amount := range amounts {
		amountRows.AddRow(amount)
	}
	mock.ExpectQuery(`SELECT amount FROM credit_note WHERE bill_id = \? FOR UPDATE`).WillReturnRows(amountRows)
}

type creditSourceTestCredit struct {
	billProductID int64
	quantity      string
}

func decodeCreditSourceResponse(t *testing.T, w *httptest.ResponseRecorder) creditSourceTestResponse {
	t.Helper()
	var got creditSourceTestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return got
}

func assertNoCreditSourcePayload(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if strings.Contains(w.Body.String(), `"bill_id"`) || strings.Contains(w.Body.String(), `"lines"`) {
		t.Fatalf("error response contains a credit-source payload: %q", w.Body.String())
	}
}

func TestGetBillCreditSourceReturnsFullCreditWithNoHistory(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectCreditSourceBill(mock, 1, int64(7), "40.25")
	expectBillLines(mock)
	expectCreditSourceHistory(mock, nil)
	mock.ExpectRollback()

	w := runCreditSourceRequest(t, h.GetBillCreditSource, "42")
	if w.Code != http.StatusOK {
		t.Fatalf("GetBillCreditSource = %d %q; want 200", w.Code, w.Body.String())
	}
	got := decodeCreditSourceResponse(t, w)
	if got.BillID != 42 || got.BranchID != 7 || got.Amount != "40.25" {
		t.Fatalf("credit source identity/amount = %d/%d/%q; want 42/7/40.25", got.BillID, got.BranchID, got.Amount)
	}
	want := []creditSourceTestLine{
		{BillProductID: 5, Quantity: "2.000", Price: "10.00", VAT: "15.00"},
		{BillProductID: 6, Quantity: "1.000", Price: "15.00", VAT: "15.00"},
	}
	if len(got.Lines) != len(want) {
		t.Fatalf("credit source lines = %#v; want %#v", got.Lines, want)
	}
	for i := range want {
		if got.Lines[i] != want[i] {
			t.Fatalf("credit source line %d = %#v; want %#v", i, got.Lines[i], want[i])
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetBillCreditSourceReturnsRemainingCreditAfterHistory(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectCreditSourceBill(mock, 1, int64(7), "40.25")
	expectBillLines(mock)
	expectCreditSourceHistory(mock, []creditSourceTestCredit{{billProductID: 5, quantity: "1.000"}}, "11.50")
	mock.ExpectRollback()

	w := runCreditSourceRequest(t, h.GetBillCreditSource, "42")
	if w.Code != http.StatusOK {
		t.Fatalf("GetBillCreditSource = %d %q; want 200", w.Code, w.Body.String())
	}
	got := decodeCreditSourceResponse(t, w)
	if got.BillID != 42 || got.BranchID != 7 || got.Amount != "28.75" {
		t.Fatalf("remaining credit identity/amount = %d/%d/%q; want 42/7/28.75", got.BillID, got.BranchID, got.Amount)
	}
	want := []creditSourceTestLine{
		{BillProductID: 5, Quantity: "1.000", Price: "10.00", VAT: "15.00"},
		{BillProductID: 6, Quantity: "1.000", Price: "15.00", VAT: "15.00"},
	}
	if len(got.Lines) != len(want) {
		t.Fatalf("remaining lines = %#v; want %#v", got.Lines, want)
	}
	for i := range want {
		if got.Lines[i] != want[i] {
			t.Fatalf("remaining line %d = %#v; want %#v", i, got.Lines[i], want[i])
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetBillCreditSourceRefusesFullyCreditedBill(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectCreditSourceBill(mock, 1, int64(7), "40.25")
	expectBillLines(mock)
	expectCreditSourceHistory(mock, []creditSourceTestCredit{
		{billProductID: 5, quantity: "2.000"},
		{billProductID: 6, quantity: "1.000"},
	}, "40.25")
	mock.ExpectRollback()

	w := runCreditSourceRequest(t, h.GetBillCreditSource, "42")
	if w.Code != http.StatusConflict {
		t.Fatalf("GetBillCreditSource for fully credited bill = %d %q; want 409", w.Code, w.Body.String())
	}
	assertNoCreditSourcePayload(t, w)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetBillCreditSourceRefusesMissingOrUnauthorizedBill(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		h, mock := newSaleTestHandler(t)
		mock.ExpectQuery(`SELECT store_id FROM bill WHERE id = \?`).
			WillReturnRows(sqlmock.NewRows([]string{"store_id"}))

		w := runCreditSourceRequest(t, h.GetBillCreditSource, "42")
		if w.Code != http.StatusNotFound {
			t.Fatalf("GetBillCreditSource for missing bill = %d %q; want 404", w.Code, w.Body.String())
		}
		assertNoCreditSourcePayload(t, w)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unauthorized store", func(t *testing.T) {
		h, mock := newSaleTestHandler(t)
		mock.ExpectQuery(`SELECT store_id FROM bill WHERE id = \?`).
			WillReturnRows(sqlmock.NewRows([]string{"store_id"}).AddRow(1))
		mock.ExpectQuery("select store.id from store").
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		w := runCreditSourceRequest(t, h.GetBillCreditSource, "42")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("GetBillCreditSource for unauthorized store = %d %q; want 400", w.Code, w.Body.String())
		}
		assertNoCreditSourcePayload(t, w)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGetBillCreditSourceRefusesUnverifiableBillData(t *testing.T) {
	tests := []struct {
		name        string
		state       int64
		branch      any
		wantStatus  int
		expectLines bool
		lineRows    *sqlmock.Rows
		credited    []creditSourceTestCredit
		amounts     []string
	}{
		{name: "draft", state: 0, branch: int64(7)},
		{name: "deleted", state: -1, branch: int64(7), wantStatus: http.StatusNotFound},
		{name: "missing branch", state: 1, branch: nil},
		{
			name:        "missing lines",
			state:       1,
			branch:      int64(7),
			expectLines: true,
			lineRows:    sqlmock.NewRows([]string{"id", "product_id", "name", "price", "quantity", "discount", "vat"}),
		},
		{
			name:        "missing VAT",
			state:       1,
			branch:      int64(7),
			expectLines: true,
			lineRows: sqlmock.NewRows([]string{"id", "product_id", "name", "price", "quantity", "discount", "vat"}).
				AddRow(5, 7, "Brake pad", "10.00", "2", "0.00", nil),
		},
		{
			name:        "unknown credited line",
			state:       1,
			branch:      int64(7),
			expectLines: true,
			credited:    []creditSourceTestCredit{{billProductID: 99, quantity: "1.000"}},
			amounts:     []string{"11.50"},
		},
		{
			name:        "amount does not match remaining lines",
			state:       1,
			branch:      int64(7),
			expectLines: true,
			credited:    []creditSourceTestCredit{{billProductID: 5, quantity: "1.000"}},
			amounts:     []string{"10.00"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, mock := newSaleTestHandler(t)
			expectCreditSourceBill(mock, tt.state, tt.branch, "40.25")
			if tt.expectLines {
				if tt.lineRows == nil {
					expectBillLines(mock)
				} else {
					mock.ExpectQuery(`SELECT id, product_id, name, price, quantity, discount, vat FROM bill_product WHERE bill_id = \?`).
						WillReturnRows(tt.lineRows)
				}
				expectCreditSourceHistory(mock, tt.credited, tt.amounts...)
			}
			mock.ExpectRollback()

			w := runCreditSourceRequest(t, h.GetBillCreditSource, "42")
			wantStatus := tt.wantStatus
			if wantStatus == 0 {
				wantStatus = http.StatusConflict
			}
			if w.Code != wantStatus {
				t.Fatalf("GetBillCreditSource = %d %q; want %d", w.Code, w.Body.String(), wantStatus)
			}
			assertNoCreditSourcePayload(t, w)
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
