package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
)

func mustDecimal(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q): %v", s, err)
	}
	return d
}

func TestWithRunningBalance(t *testing.T) {
	entries := []supplierLedgerEntry{
		{Date: "2026-01-01", Debit: 100},
		{Date: "2026-01-05", Credit: 40},
		{Date: "2026-01-10", Debit: 25},
		{Date: "2026-01-15", Credit: 85},
	}
	got := withRunningBalance(entries)
	want := []float64{100, 60, 85, 0}
	for i, w := range want {
		if got[i].Balance != w {
			t.Fatalf("entry %d balance = %v, want %v (entries=%+v)", i, got[i].Balance, w, got)
		}
	}
}

func TestBuildSupplierLedgerEntriesOrdersChronologicallyPerSupplier(t *testing.T) {
	nameA := "Supplier A"
	seq := uint64(7)
	bills := []db.GetLedgerBillsForMerchantRow{
		{ID: 1, SupplierID: 10, SupplierName: &nameA, SupplierSequenceNumber: &seq,
			EffectiveDate: mustTime(t, "2026-02-01"), Total: mustDecimal(t, "150.00")},
	}
	supplierID := int32(10)
	payments := []db.GetLedgerPaymentsForMerchantRow{
		{ID: 2, SupplierID: &supplierID, SupplierName: nameA, VoucherNumber: 3,
			EffectiveDate: mustTime(t, "2026-01-15"), Amount: mustDecimal(t, "50.00")},
	}

	bySupplier := buildSupplierLedgerEntries(bills, payments)
	entries, ok := bySupplier[10]
	if !ok || len(entries) != 2 {
		t.Fatalf("expected 2 entries for supplier 10, got %+v", bySupplier)
	}
	// Payment (Jan 15) predates the bill (Feb 1), so it must come first.
	if entries[0].Type != "payment" || entries[1].Type != "bill" {
		t.Fatalf("entries not chronologically ordered: %+v", entries)
	}
	if entries[0].Credit != 50 || entries[1].Debit != 150 {
		t.Fatalf("unexpected debit/credit values: %+v", entries)
	}
	// Running balance: -50 after the payment, then +150 = 100.
	if entries[0].Balance != -50 || entries[1].Balance != 100 {
		t.Fatalf("unexpected running balance: %+v", entries)
	}
	if entries[1].SupplierNo != "7" {
		t.Fatalf("expected bill's supplier_no to be the supplier_sequence_number, got %q", entries[1].SupplierNo)
	}
}

func TestSupplierLedgerBreakdownsSumsPerSupplier(t *testing.T) {
	bySupplier := map[int32][]supplierLedgerEntry{
		10: {
			{SupplierID: 10, Type: "bill", Description: "Acme", Debit: 200, Balance: 200},
			{SupplierID: 10, Type: "payment", Debit: 0, Credit: 80, Balance: 120},
		},
		20: {
			{SupplierID: 20, Type: "bill", Description: "Beta", Debit: 50, Balance: 50},
		},
	}
	breakdowns := supplierLedgerBreakdowns(bySupplier)
	if len(breakdowns) != 2 {
		t.Fatalf("expected 2 supplier breakdowns, got %d: %+v", len(breakdowns), breakdowns)
	}
	bySupplierID := map[int32]supplierLedgerBreakdown{}
	for _, b := range breakdowns {
		bySupplierID[b.SupplierID] = b
	}
	if got := bySupplierID[10]; got.Debit != 200 || got.Credit != 80 || got.Balance != 120 || got.SupplierName != "Acme" {
		t.Fatalf("supplier 10 breakdown = %+v", got)
	}
	if got := bySupplierID[20]; got.Debit != 50 || got.Balance != 50 || got.SupplierName != "Beta" {
		t.Fatalf("supplier 20 breakdown = %+v", got)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("time.Parse(%q): %v", s, err)
	}
	return parsed
}

func TestGetSupplierLedgerRejectsInvalidDate(t *testing.T) {
	h, _, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	w := runPurchaseBillRequest(t, h.GetSupplierLedger, http.MethodGet, "/api/v2/supplier/ledger?from=not-a-date", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestGetSupplierLedgerRejectsInvalidSupplierID(t *testing.T) {
	h, _, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	w := runPurchaseBillRequest(t, h.GetSupplierLedger, http.MethodGet, "/api/v2/supplier/ledger?supplier_id=abc", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestGetSupplierLedgerSingleSupplierEndToEnd(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	billCols := []string{"id", "supplier_id", "supplier_name", "supplier_sequence_number", "effective_date", "total"}
	mock.ExpectQuery("FROM purchase_bill pb").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(billCols).AddRow(1, int32(10), "Acme", uint64(5), mustTime(t, "2026-01-10"), "300.00"))

	paymentCols := []string{"id", "supplier_id", "supplier_name", "voucher_number", "effective_date", "amount"}
	mock.ExpectQuery("FROM cash_voucher cv").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(paymentCols).AddRow(2, int32(10), "Acme", int32(9), mustTime(t, "2026-01-20"), "100.00"))

	w := runPurchaseBillRequest(t, h.GetSupplierLedger, http.MethodGet, "/api/v2/supplier/ledger?supplier_id=10", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var body struct {
		Summary struct {
			BillCount      int             `json:"bill_count"`
			PaymentCount   int             `json:"payment_count"`
			ClosingBalance decimal.Decimal `json:"closing_balance"`
		} `json:"summary"`
		Ledger    []map[string]interface{} `json:"ledger"`
		Suppliers []map[string]interface{} `json:"suppliers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if body.Summary.BillCount != 1 || body.Summary.PaymentCount != 1 {
		t.Fatalf("summary counts = %+v", body.Summary)
	}
	if !body.Summary.ClosingBalance.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("closing balance = %v, want 200 (300 debit - 100 credit)", body.Summary.ClosingBalance)
	}
	if len(body.Ledger) != 2 {
		t.Fatalf("expected 2 combined ledger entries, got %d: %+v", len(body.Ledger), body.Ledger)
	}
	// A single explicit supplier_id must NOT include the "suppliers" breakdown.
	if body.Suppliers != nil {
		t.Fatalf("expected no per-supplier breakdown when supplier_id is explicit, got %+v", body.Suppliers)
	}
	assertMockExpectations(t, mock)
}

func TestGetSupplierLedgerAllSuppliersIncludesBreakdown(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	billCols := []string{"id", "supplier_id", "supplier_name", "supplier_sequence_number", "effective_date", "total"}
	mock.ExpectQuery("FROM purchase_bill pb").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(billCols).
			AddRow(1, int32(10), "Acme", uint64(5), mustTime(t, "2026-01-10"), "300.00").
			AddRow(2, int32(20), "Beta", uint64(6), mustTime(t, "2026-01-11"), "75.00"))

	paymentCols := []string{"id", "supplier_id", "supplier_name", "voucher_number", "effective_date", "amount"}
	mock.ExpectQuery("FROM cash_voucher cv").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(paymentCols))

	w := runPurchaseBillRequest(t, h.GetSupplierLedger, http.MethodGet, "/api/v2/supplier/ledger", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var body struct {
		Suppliers []map[string]interface{} `json:"suppliers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if len(body.Suppliers) != 2 {
		t.Fatalf("expected 2 supplier breakdowns for supplier_id=all (or omitted), got %d: %+v", len(body.Suppliers), body.Suppliers)
	}
	assertMockExpectations(t, mock)
}
