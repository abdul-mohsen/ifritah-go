package handlers

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
)

// d parses a decimal literal in a test.
func d(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

// The totals of a note line are rounded as the database rounds bill_product: the line is rounded before and after its discount, and the
// VAT is rounded on its own. A half rounds up.
func TestNoteLineTotalRoundsAsTheDatabaseDoes(t *testing.T) {
	cases := map[string]struct{ quantity, price, discount, vat, want string }{
		"a discount on several units": {"3", "10.00", "1.00", "15", "33.35"},
		"half a halala rounds up":     {"3", "0.10", "0.00", "15", "0.35"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := noteLineTotal(d(tc.quantity), d(tc.price), d(tc.discount), d(tc.vat))
			if !got.Equal(d(tc.want)) {
				t.Fatalf("noteLineTotal = %s; want %s", got, tc.want)
			}
		})
	}
}

// A credit line carries its share of the bill line's discount, in proportion to the quantity it credits.
func TestCreditLinesShareTheDiscountOfTheBillLineInProportion(t *testing.T) {
	vat := d("15.00")
	billLines := map[uint64]db.ListBillLinesForNoteRow{
		5: {ID: 5, Price: d("10.00"), Quantity: d("3"), Discount: d("3.00"), Vat: &vat},
	}
	lines, err := creditLines([]model.CreditNoteLine{{BillProductID: 5, Quantity: d("1"), Price: d("10.00"), VAT: d("15.00")}}, billLines, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !lines[0].discount.Equal(d("1.00")) || !lines[0].total.Equal(d("10.35")) {
		t.Fatalf("credit line discount %s and total %s; want 1.00 and 10.35", lines[0].discount, lines[0].total)
	}
}

// The quantity of a bill line is shared between the notes. A request cannot credit more than is left, even across its own lines.
func TestCreditLinesRefuseMoreThanTheBillLineHasLeft(t *testing.T) {
	vat := d("15.00")
	billLines := map[uint64]db.ListBillLinesForNoteRow{5: {ID: 5, Price: d("10.00"), Quantity: d("3"), Discount: decimal.Zero, Vat: &vat}}
	twoAndTwo := []model.CreditNoteLine{
		{BillProductID: 5, Quantity: d("2"), Price: d("10.00"), VAT: d("15.00")},
		{BillProductID: 5, Quantity: d("2"), Price: d("10.00"), VAT: d("15.00")},
	}
	if _, err := creditLines(twoAndTwo, billLines, nil); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("four of three units = %v; want a refusal of line 2", err)
	}
	if _, err := creditLines(twoAndTwo[:1], billLines, map[uint64]decimal.Decimal{5: d("2")}); err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("two units after two were credited = %v; want a refusal of line 1", err)
	}
}

// A credit note that cannot be checked from its own request is refused before the bill is read.
func TestVerifyCreditRequestRefusesWhatCannotBeChecked(t *testing.T) {
	good := func() model.CreditNoteRequest {
		return model.CreditNoteRequest{
			BillID: 42, BranchID: 7, Reason: "returned", Amount: d("11.50"),
			Lines: []model.CreditNoteLine{{BillProductID: 5, Quantity: d("1"), Price: d("10.00"), VAT: d("15.00")}},
		}
	}
	cases := map[string]struct {
		change func(*model.CreditNoteRequest)
		want   string
	}{
		"blank reason":                {func(r *model.CreditNoteRequest) { r.Reason = "  " }, "reason is required"},
		"long reason":                 {func(r *model.CreditNoteRequest) { r.Reason = strings.Repeat("a", 513) }, "at most 512"},
		"zero amount":                 {func(r *model.CreditNoteRequest) { r.Amount = d("0") }, "above zero"},
		"amount with three decimals":  {func(r *model.CreditNoteRequest) { r.Amount = d("11.501") }, "two decimal places"},
		"no lines":                    {func(r *model.CreditNoteRequest) { r.Lines = nil }, "at least one line"},
		"line without a bill line":    {func(r *model.CreditNoteRequest) { r.Lines[0].BillProductID = 0 }, "bill_product_id is required"},
		"zero quantity":               {func(r *model.CreditNoteRequest) { r.Lines[0].Quantity = d("0") }, "quantity must be above zero"},
		"quantity with four decimals": {func(r *model.CreditNoteRequest) { r.Lines[0].Quantity = d("1.0001") }, "three decimal places"},
		"VAT above 100":               {func(r *model.CreditNoteRequest) { r.Lines[0].VAT = d("101") }, "VAT rate"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			request := good()
			tc.change(&request)
			if err := verifyCreditRequest(&request); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("verifyCreditRequest = %v; want an error containing %q", err, tc.want)
			}
		})
	}

	request := good()
	request.Reason = "  returned  "
	if err := verifyCreditRequest(&request); err != nil || request.Reason != "returned" {
		t.Fatalf("a valid note = %v, reason %q; want no error and the trimmed reason", err, request.Reason)
	}
}

// A debit note's charges need a name, and the same amount rules as a credit note.
func TestVerifyDebitRequestRefusesWhatCannotBeChecked(t *testing.T) {
	good := func() model.DebitNoteRequest {
		return model.DebitNoteRequest{
			BillID: 42, BranchID: 7, Reason: "extra", Amount: d("5.75"),
			Lines: []model.DebitNoteLine{{Name: "Towing", Quantity: d("1"), Price: d("5.00"), VAT: d("15.00")}},
		}
	}
	cases := map[string]struct {
		change func(*model.DebitNoteRequest)
		want   string
	}{
		"blank reason":   {func(r *model.DebitNoteRequest) { r.Reason = "" }, "reason is required"},
		"zero amount":    {func(r *model.DebitNoteRequest) { r.Amount = d("0") }, "above zero"},
		"no lines":       {func(r *model.DebitNoteRequest) { r.Lines = nil }, "at least one line"},
		"blank name":     {func(r *model.DebitNoteRequest) { r.Lines[0].Name = "  " }, "name is required"},
		"long name":      {func(r *model.DebitNoteRequest) { r.Lines[0].Name = strings.Repeat("n", 256) }, "at most 255"},
		"zero quantity":  {func(r *model.DebitNoteRequest) { r.Lines[0].Quantity = d("0") }, "quantity must be above zero"},
		"negative price": {func(r *model.DebitNoteRequest) { r.Lines[0].Price = d("-1") }, "price must be above zero"},
		"VAT below zero": {func(r *model.DebitNoteRequest) { r.Lines[0].VAT = d("-1") }, "VAT rate"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			request := good()
			tc.change(&request)
			if err := verifyDebitRequest(&request); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("verifyDebitRequest = %v; want an error containing %q", err, tc.want)
			}
		})
	}

	request := good()
	request.Lines[0].Name = "  Towing  "
	if err := verifyDebitRequest(&request); err != nil || request.Lines[0].Name != "Towing" {
		t.Fatalf("a valid note = %v, name %q; want no error and the trimmed name", err, request.Lines[0].Name)
	}
}

// The amount of a note must be the total of its lines, and the remaining amount of a bill is its total less the earlier notes.
func TestNoteTotalsAndTheRemainingAmountOfABill(t *testing.T) {
	lines := []noteLine{{total: d("11.50")}, {total: d("2.25")}}
	total, err := checkNoteTotal(d("13.75"), lines)
	if err != nil || !total.Equal(d("13.75")) {
		t.Fatalf("checkNoteTotal = %s, %v; want 13.75 and no error", total, err)
	}
	if _, err := checkNoteTotal(d("13.00"), lines); err == nil || !strings.Contains(err.Error(), "13.75") {
		t.Fatalf("checkNoteTotal with a different amount = %v; want an error with the total 13.75", err)
	}
	if remaining := creditRemaining(d("40.25"), []decimal.Decimal{d("40.00"), d("0.10")}); !remaining.Equal(d("0.15")) {
		t.Fatalf("creditRemaining = %s; want 0.15", remaining)
	}
}
