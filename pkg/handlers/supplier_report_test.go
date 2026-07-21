package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestParseSupplierIDList(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    []int32
		wantErr bool
	}{
		{name: "single", raw: "12", want: []int32{12}},
		{name: "multiple", raw: "12,34,56", want: []int32{12, 34, 56}},
		{name: "dedupes", raw: "12,12,34", want: []int32{12, 34}},
		{name: "trims whitespace", raw: " 12 , 34 ", want: []int32{12, 34}},
		{name: "skips empty segments", raw: "12,,34", want: []int32{12, 34}},
		{name: "empty string is an error", raw: "", wantErr: true},
		{name: "non-numeric is an error", raw: "12,abc", wantErr: true},
		{name: "zero is an error", raw: "0", wantErr: true},
		{name: "negative is an error", raw: "-5", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSupplierIDList(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got ids=%v", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.raw, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// expectFullSupplierReport sets up sqlmock expectations for one full,
// successful pass through buildSupplierReport (all 8 queries), returning
// minimal/empty result sets - this test is about the multi-supplier
// looping/combining/skip-on-error behavior in GetMultiSupplierReport, not
// about re-verifying the report SQL itself (already exercised in
// production via the pre-existing single-supplier endpoint).
func expectFullSupplierReport(mock sqlmock.Sqlmock, supplierID int64) {
	mock.ExpectQuery("From supplier where is_deleted").
		WithArgs(supplierID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "company_id", "name", "address", "short_address", "phone_number",
			"number", "vat_number", "commercial_registration", "is_deleted", "bank_account",
			"created_at", "updated_at", "is_postpaid", "credit_limit", "payment_terms_days",
			"preferred_payment_method", "email",
		}).AddRow(
			supplierID, int32(1), "Supplier", nil, nil, nil,
			nil, nil, nil, false, nil,
			time.Now(), time.Now(), false, "0", int32(30),
			int32(10), nil,
		))
	mock.ExpectQuery("FROM purchase_bill pb").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"bill_count", "total_spent", "total_before_vat", "total_vat",
			"unpaid_total", "paid_total", "received_count", "avg_bill", "total_discount",
		}).AddRow("0", "0", "0", "0", "0", "0", "0", "0", "0"))
	mock.ExpectQuery("FROM cash_voucher cv").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"payment_count", "total_payments", "cash_total", "bank_transfer_total",
		}).AddRow("0", "0", "0", "0"))
	mock.ExpectQuery("FROM purchase_bill pb").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "sequence_number", "supplier_sequence_number", "supplier_id", "store_id",
			"state", "discount", "effective_date", "payment_due_date", "received_at",
			"received_by", "total_before_vat", "total_vat", "total",
		}))
	mock.ExpectQuery("FROM cash_voucher cv").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "voucher_number", "voucher_type", "effective_date", "amount",
			"payment_method", "state", "reference_type", "reference_id", "description", "created_at",
		}))
	mock.ExpectQuery("FROM purchase_bill_product pbp").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"item_name", "total_qty", "total_value", "avg_price", "bill_count",
		}))
	mock.ExpectQuery("FROM purchase_bill pb").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"bucket", "bill_count", "bucket_total"}))
	mock.ExpectQuery("FROM purchase_bill pb").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"month", "bill_count", "total_spent"}))
}

func TestGetMultiSupplierReportCombinesAndSkipsUnresolvable(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	// Supplier 10: resolves fully.
	expectFullSupplierReport(mock, 10)
	// Supplier 99: GetSupplier fails (not found / wrong tenant) - should be
	// skipped rather than failing the whole combined statement.
	mock.ExpectQuery("From supplier where is_deleted").
		WithArgs(int64(99)).
		WillReturnError(sql.ErrNoRows)

	w := runPurchaseBillRequest(t, h.GetMultiSupplierReport, http.MethodGet, "/api/v2/supplier/report/multi?ids=10,99", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var body struct {
		Suppliers []map[string]interface{} `json:"suppliers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if len(body.Suppliers) != 1 {
		t.Fatalf("expected exactly 1 resolved supplier report (id=99 should be skipped), got %d: %+v", len(body.Suppliers), body.Suppliers)
	}
	assertMockExpectations(t, mock)
}

func TestGetMultiSupplierReportRequiresIDs(t *testing.T) {
	h, _, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	w := runPurchaseBillRequest(t, h.GetMultiSupplierReport, http.MethodGet, "/api/v2/supplier/report/multi", "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestGetMultiSupplierReportRejectsTooManyIDs(t *testing.T) {
	h, _, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	ids := ""
	for i := 1; i <= maxMultiSupplierReportIDs+1; i++ {
		if ids != "" {
			ids += ","
		}
		ids += strconv.Itoa(i)
	}

	w := runPurchaseBillRequest(t, h.GetMultiSupplierReport, http.MethodGet, "/api/v2/supplier/report/multi?ids="+ids, "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}
