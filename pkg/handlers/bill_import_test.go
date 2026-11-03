package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"

	"ifritah/web-service-gin/pkg/model"
)

// importSale is a final sale of one product at branch 7 and store 1, with no identity, as a row of an import sends it.
func importSale() string {
	return saleBody(1, "", "")
}

// importStockSale is a final sale of one catalog product at branch 7 and store 1, with no identity.
func importStockSale(productID int, price, quantity string) string {
	return `{"store_id":1,"state":1,"discount":"0","maintenance_cost":"0","payment_method":1,"branch_id":7,` +
		`"products":[{"product_id":` + fmt.Sprint(productID) + `,"name":"Brake pad","price":"` + price + `","quantity":"` + quantity + `","discount":"0"}],"manual_products":[]}`
}

// importBody is the body of a check of the given rows.
func importBody(rows ...string) string {
	return `{"rows":[` + strings.Join(rows, ",") + `]}`
}

// checkImport runs a check, and returns the answer for each row. It fails the test unless the check answers 200.
func checkImport(t *testing.T, h *handler, body string) []importRowResult {
	t.Helper()
	w := runSaleRequest(t, h.CheckBillImport, "/bill/import/check", body, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("CheckBillImport = %d %q; want 200", w.Code, w.Body.String())
	}
	var answer struct {
		Rows []importRowResult `json:"rows"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	return answer.Rows
}

// hasImportCode reports whether one of the issues has the code.
func hasImportCode(issues []importIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

// expectImportStart is what a check reads before its rows: the stores the caller can sell from, and the stock mode.
func expectImportStart(mock sqlmock.Sqlmock, stockMode string) {
	expectStoreAccess(mock)
	mock.ExpectQuery(`SELECT COALESCE\(value, 'disable'\)`).WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(stockMode))
}

// expectImportBranch is the read of a branch: its seller VAT, its EGS id, and whether the daemon runs it. The branch is read once per check.
func expectImportBranch(mock sqlmock.Sqlmock, vat, egsID string, daemonRuns bool) {
	mock.ExpectQuery(`COALESCE\(c\.vat_registration_number, ''\) AS seller_vat`).
		WillReturnRows(sqlmock.NewRows([]string{"seller_vat", "egs_id", "daemon_runs"}).AddRow(vat, egsID, daemonRuns))
}

// expectNoImportBranch is the read of a branch that does not exist.
func expectNoImportBranch(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`COALESCE\(c\.vat_registration_number, ''\) AS seller_vat`).
		WillReturnRows(sqlmock.NewRows([]string{"seller_vat", "egs_id", "daemon_runs"}))
}

// expectImportDuplicate is the lookup of a bill on branch 7 with the row's fingerprint. A nil id means that no bill has it.
func expectImportDuplicate(mock sqlmock.Sqlmock, fingerprint string, id *int64) {
	rows := sqlmock.NewRows([]string{"id"})
	if id != nil {
		rows.AddRow(*id)
	}
	mock.ExpectQuery(`SELECT id FROM bill WHERE branch_id = \? AND request_fingerprint = \? AND request_fingerprint_version = \?`).
		WithArgs(int64(7), fingerprint, int64(1)).
		WillReturnRows(rows)
}

// expectImportProductStock is the read of a product's quantity for the stock check.
func expectImportProductStock(mock sqlmock.Sqlmock, productID int64, quantity string) {
	mock.ExpectQuery(`SELECT quantity FROM product WHERE id = \? FOR UPDATE`).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"quantity"}).AddRow(quantity))
}

func TestCheckBillImportAcceptsAValidSale(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	expectStoreBranch(mock, 7)
	expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
	expectImportDuplicate(mock, fingerprintOf(t, importSale()), nil)

	w := runSaleRequest(t, h.CheckBillImport, "/bill/import/check", importBody(importSale()), nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"errors":[],"warnings":[],"duplicate_of":null`) {
		t.Fatalf("CheckBillImport = %d %q; want 200 with an empty error list, an empty warning list, and no duplicate", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"index":0,"valid":true`) {
		t.Fatalf("CheckBillImport = %q; want row 0 valid", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A payment equal to the bill's total is accepted. The total is the lines with their VAT: 10 plus 15 percent.
func TestCheckBillImportAcceptsAPaymentUpToTheTotal(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	expectStoreBranch(mock, 7)
	expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
	expectImportDuplicate(mock, fingerprintOf(t, withPaidAmount(importSale(), "11.50")), nil)

	rows := checkImport(t, h, importBody(withPaidAmount(importSale(), "11.50")))
	if !rows[0].Valid || len(rows[0].Errors) != 0 {
		t.Fatalf("row = %+v; want a valid row for a payment equal to the total", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Each case breaks one rule. A row that breaks a rule is invalid, and its errors name the rule's code. The stores, the branch and the
// duplicate are read for every row that has them.
func TestCheckBillImportReportsEachRefusal(t *testing.T) {
	const head = `{"store_id":1,"state":1,"discount":"0","maintenance_cost":"0","payment_method":1,"branch_id":7,`
	line := func(price, quantity string) string {
		return `{"product_id":null,"name":"Brake pad","price":"` + price + `","quantity":"` + quantity + `","discount":"0"}`
	}
	withLines := func(lines string) string {
		return head + `"products":[` + lines + `],"manual_products":[]}`
	}
	cases := map[string]struct {
		body    string
		code    string
		setup   func(mock sqlmock.Sqlmock)
		noReads bool
	}{
		"a row that is not a sale": {
			body:    `"not a sale"`,
			code:    importInvalidRequest,
			noReads: true,
		},
		"no branch": {
			body:    strings.Replace(importSale(), `"branch_id":7,`, "", 1),
			code:    importInvalidRequest,
			noReads: true,
		},
		"a draft, which is not final": {
			body: saleBody(0, "", ""),
			code: importNotFinal,
		},
		"a negative payment": {
			body: withPaidAmount(importSale(), "-1.00"),
			code: "paid_amount_negative",
		},
		"a payment with three decimals": {
			body: withPaidAmount(importSale(), "11.505"),
			code: "paid_amount_precision",
		},
		"a payment above the total": {
			body: withPaidAmount(importSale(), "11.51"),
			code: "paid_amount_above_total",
		},
		"no lines": {
			body: head + `"products":[],"manual_products":[]}`,
			code: "no_lines",
		},
		"a zero quantity": {
			body: withLines(line("10", "0")),
			code: "line_quantity",
		},
		"a quantity that stores as zero": {
			body: withLines(line("10", "0.0004")),
			code: "line_quantity",
		},
		"a negative price": {
			body: withLines(line("-10", "1")),
			code: "line_price",
		},
		"a zero price": {
			body: withLines(line("0", "1")),
			code: "line_price",
		},
		"a sub-cent price": {
			body: withLines(line("0.004", "1")),
			code: "line_price",
		},
		"a sub-cent maintenance cost": {
			body: strings.Replace(withLines(line("10", "1")), `"maintenance_cost":"0"`, `"maintenance_cost":"0.004"`, 1),
			code: "line_price",
		},
		"a store the caller cannot sell from": {
			body: strings.Replace(importSale(), `"store_id":1,`, `"store_id":9,`, 1),
			code: importStoreNotAllowed,
		},
		"a branch that does not exist": {
			body:  importSale(),
			code:  importBranchNotFound,
			setup: expectNoImportBranch,
		},
		"a branch with no seller VAT": {
			body:  importSale(),
			code:  importSellerVATMissing,
			setup: func(mock sqlmock.Sqlmock) { expectImportBranch(mock, "", "egs-uuid-7", true) },
		},
		"a branch with no EGS id": {
			body:  importSale(),
			code:  importEGSIDMissing,
			setup: func(mock sqlmock.Sqlmock) { expectImportBranch(mock, "300000000000003", "", true) },
		},
		"a branch the daemon does not run": {
			body:  importSale(),
			code:  importBranchNotRegistered,
			setup: func(mock sqlmock.Sqlmock) { expectImportBranch(mock, "300000000000003", "egs-uuid-7", false) },
		},
		"a sale dated after today": {
			body: withEffectiveDate(importSale(), dayAtNoonUTC(time.Now().Add(36*time.Hour))),
			code: importDateInFuture,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, mock := newSaleTestHandler(t)
			expectImportStart(mock, "disable")
			switch {
			case tc.noReads:
			case tc.setup != nil:
				expectStoreBranch(mock, 7)
				tc.setup(mock)
				expectImportDuplicate(mock, fingerprintOf(t, tc.body), nil)
			default:
				expectStoreBranch(mock, 7)
				expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
				expectImportDuplicate(mock, fingerprintOf(t, tc.body), nil)
			}

			rows := checkImport(t, h, importBody(tc.body))
			if rows[0].Valid || !hasImportCode(rows[0].Errors, tc.code) {
				t.Fatalf("row = %+v; want an invalid row with the error %s", rows[0], tc.code)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The sale path refuses a store that belongs to another branch, or to none. The check refuses such a row with the same words.
func TestCheckBillImportRefusesAStoreOfAnotherBranch(t *testing.T) {
	cases := map[string]struct {
		storeBranch any
	}{
		"a store of another branch": {storeBranch: 8},
		"a store with no branch":    {storeBranch: nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, mock := newSaleTestHandler(t)
			expectImportStart(mock, "disable")
			expectStoreBranch(mock, tc.storeBranch)
			expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
			expectImportDuplicate(mock, fingerprintOf(t, importSale()), nil)

			rows := checkImport(t, h, importBody(importSale()))
			if rows[0].Valid || !hasImportCode(rows[0].Errors, importStoreNotInBranch) {
				t.Fatalf("row = %+v; want an invalid row with %s", rows[0], importStoreNotInBranch)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The daemon runs a branch that is active and has a production username and password. A row on a branch it does not run is refused with
// branch_not_registered. A row on a branch it runs is not. The migrated-database test sets each state of the branch.
func TestCheckBillImportRefusesABranchTheDaemonDoesNotRun(t *testing.T) {
	cases := map[string]struct {
		daemonRuns, refused bool
	}{
		"a branch with no production credentials": {daemonRuns: false, refused: true},
		"an active branch with credentials":       {daemonRuns: true, refused: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, mock := newSaleTestHandler(t)
			expectImportStart(mock, "disable")
			expectStoreBranch(mock, 7)
			expectImportBranch(mock, "300000000000003", "egs-uuid-7", tc.daemonRuns)
			expectImportDuplicate(mock, fingerprintOf(t, importSale()), nil)

			rows := checkImport(t, h, importBody(importSale()))
			if hasImportCode(rows[0].Errors, importBranchNotRegistered) != tc.refused || rows[0].Valid == tc.refused {
				t.Fatalf("row = %+v; want branch_not_registered %v and valid %v", rows[0], tc.refused, !tc.refused)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A row that matches a bill already saved on its branch is valid, and carries the bill's id. The user decides what to do with it.
func TestCheckBillImportFlagsASaleThatIsAlreadyABill(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	expectStoreBranch(mock, 7)
	expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
	id := int64(42)
	expectImportDuplicate(mock, fingerprintOf(t, importSale()), &id)

	rows := checkImport(t, h, importBody(importSale()))
	row := rows[0]
	if !row.Valid || len(row.Errors) != 0 || row.DuplicateOf == nil || *row.DuplicateOf != 42 || !hasImportCode(row.Warnings, importDuplicateSale) {
		t.Fatalf("row = %+v; want a valid row with a duplicate warning for bill 42", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A row that repeats an earlier row of the same check is a warning. Each row is still looked up, and the branch is read once.
func TestCheckBillImportFlagsARowThatRepeatsAnEarlierRow(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	expectStoreBranch(mock, 7)
	expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
	expectImportDuplicate(mock, fingerprintOf(t, importSale()), nil)
	expectStoreBranch(mock, 7)
	expectImportDuplicate(mock, fingerprintOf(t, importSale()), nil)

	rows := checkImport(t, h, importBody(importSale(), importSale()))
	if !rows[0].Valid || len(rows[0].Warnings) != 0 {
		t.Fatalf("first row = %+v; want a valid row with no warning", rows[0])
	}
	if !rows[1].Valid || !hasImportCode(rows[1].Warnings, importDuplicateInBatch) || rows[1].DuplicateOf != nil {
		t.Fatalf("second row = %+v; want a valid row with a duplicate_in_batch warning", rows[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A row answers the fingerprint of its sale, which is the one the sale path stores for duplicate detection. The client gets the same value
// that the check uses to look up bills on the branch.
func TestCheckBillImportAnswersTheFingerprintOfARow(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	expectStoreBranch(mock, 7)
	expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
	fingerprint := fingerprintOf(t, importSale())
	expectImportDuplicate(mock, fingerprint, nil)

	w := runSaleRequest(t, h.CheckBillImport, "/bill/import/check", importBody(importSale()), nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"request_fingerprint":"`+fingerprint+`"`) {
		t.Fatalf("CheckBillImport = %d %q; want 200 with the row's request_fingerprint %s", w.Code, w.Body.String(), fingerprint)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Two rows with the same content have the same fingerprint, and each row is looked up.
func TestCheckBillImportGivesIdenticalRowsOneFingerprint(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	expectStoreBranch(mock, 7)
	expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
	fingerprint := fingerprintOf(t, importSale())
	expectImportDuplicate(mock, fingerprint, nil)
	expectStoreBranch(mock, 7)
	expectImportDuplicate(mock, fingerprint, nil)

	w := runSaleRequest(t, h.CheckBillImport, "/bill/import/check", importBody(importSale(), importSale()), nil)
	if w.Code != http.StatusOK || strings.Count(w.Body.String(), `"request_fingerprint":"`+fingerprint+`"`) != 2 {
		t.Fatalf("CheckBillImport = %d %q; want both rows with the fingerprint %s", w.Code, w.Body.String(), fingerprint)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Changing one field of a row changes its fingerprint. Here only the price of the second row differs.
func TestCheckBillImportChangesTheFingerprintWhenOneFieldChanges(t *testing.T) {
	first := importStockSale(12, "10", "2")
	second := importStockSale(12, "10.50", "2")
	firstFingerprint := fingerprintOf(t, first)
	secondFingerprint := fingerprintOf(t, second)
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	expectStoreBranch(mock, 7)
	expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
	expectImportDuplicate(mock, firstFingerprint, nil)
	expectStoreBranch(mock, 7)
	expectImportDuplicate(mock, secondFingerprint, nil)

	w := runSaleRequest(t, h.CheckBillImport, "/bill/import/check", importBody(first, second), nil)
	body := w.Body.String()
	if w.Code != http.StatusOK || firstFingerprint == secondFingerprint ||
		!strings.Contains(body, `"request_fingerprint":"`+firstFingerprint+`"`) || !strings.Contains(body, `"request_fingerprint":"`+secondFingerprint+`"`) {
		t.Fatalf("CheckBillImport = %d %q; want each row with its own fingerprint", w.Code, body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A row that is not a sale request has no fingerprint, so its answer carries null.
func TestCheckBillImportAnswersNoFingerprintForARowThatIsNotASale(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")

	w := runSaleRequest(t, h.CheckBillImport, "/bill/import/check", importBody(`"not a sale"`), nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"request_fingerprint":null`) {
		t.Fatalf("CheckBillImport = %d %q; want 200 with request_fingerprint null", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The stock rule of the sale path applies in enforce mode and in warn mode, because the database refuses the deduction in both. In
// disable mode the stock is not read.
func TestCheckBillImportChecksTheStockOfEachLine(t *testing.T) {
	cases := map[string]struct {
		mode, stock, quantity string
		want                  string
	}{
		"enforce, short":  {mode: "enforce", stock: "1.000", quantity: "2", want: importStockShort},
		"warn, short":     {mode: "warn", stock: "1.000", quantity: "2", want: importStockShort},
		"enforce, enough": {mode: "enforce", stock: "3.000", quantity: "2", want: ""},
		"disable, short":  {mode: "disable", stock: "", quantity: "2", want: ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, mock := newSaleTestHandler(t)
			body := importStockSale(12, "10", tc.quantity)
			expectImportStart(mock, tc.mode)
			expectStoreBranch(mock, 7)
			expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
			if tc.mode != "disable" {
				expectImportProductStock(mock, 12, tc.stock)
			}
			expectImportDuplicate(mock, fingerprintOf(t, body), nil)

			rows := checkImport(t, h, importBody(body))
			requireStockRow(t, rows[0], tc.want)
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// requireStockRow fails unless the row is valid when no error is wanted, and invalid with the wanted error when one is.
func requireStockRow(t *testing.T, row importRowResult, want string) {
	t.Helper()
	if want == "" && !row.Valid {
		t.Fatalf("row = %+v; want a valid row", row)
	}
	if want != "" && (row.Valid || !hasImportCode(row.Errors, want)) {
		t.Fatalf("row = %+v; want an invalid row with the error %s", row, want)
	}
}

// The rows of one check take the stock in turn. The second row finds only what the first row left.
func TestCheckBillImportCountsTheStockThatEarlierRowsTake(t *testing.T) {
	first := importStockSale(12, "10", "2")
	second := importStockSale(12, "11", "2")
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "enforce")
	expectStoreBranch(mock, 7)
	expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
	expectImportProductStock(mock, 12, "3.000")
	expectImportDuplicate(mock, fingerprintOf(t, first), nil)
	expectStoreBranch(mock, 7)
	expectImportProductStock(mock, 12, "3.000")
	expectImportDuplicate(mock, fingerprintOf(t, second), nil)

	rows := checkImport(t, h, importBody(first, second))
	if !rows[0].Valid {
		t.Fatalf("first row = %+v; want a valid row", rows[0])
	}
	if rows[1].Valid || !hasImportCode(rows[1].Errors, importStockShort) || !strings.Contains(rows[1].Errors[0].Message, "has 1 left") {
		t.Fatalf("second row = %+v; want the stock error with 1 left after the first row", rows[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCheckBillImportRefusesABatchOverTheLimit(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	rows := strings.TrimSuffix(strings.Repeat(`1,`, maxImportRows+1), ",")

	w := runSaleRequest(t, h.CheckBillImport, "/bill/import/check", `{"rows":[`+rows+`]}`, nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "at most 500 rows") {
		t.Fatalf("CheckBillImport over the limit = %d %q; want 400 with the limit", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A batch of exactly the limit is checked. Each row here is not a sale, so the rows need no reads beyond the caller's stores and the
// stock mode.
func TestCheckBillImportTakesAFullBatch(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	rows := strings.TrimSuffix(strings.Repeat(`1,`, maxImportRows), ",")

	answer := checkImport(t, h, `{"rows":[`+rows+`]}`)
	if len(answer) != maxImportRows || answer[maxImportRows-1].Index != maxImportRows-1 {
		t.Fatalf("answer has %d rows; want %d, indexed from 0", len(answer), maxImportRows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A failed read fails the whole check. No partial answer is given, because an answer for a row needs its reads.
func TestCheckBillImportAnswersAFailedReadWithoutRows(t *testing.T) {
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	expectStoreBranch(mock, 7)
	mock.ExpectQuery(`COALESCE\(c\.vat_registration_number, ''\) AS seller_vat`).WillReturnError(errors.New("lock wait timeout"))

	w := runSaleRequest(t, h.CheckBillImport, "/bill/import/check", importBody(importSale()), nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("CheckBillImport with a failed read = %d; want 500", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A body that is not a list of rows is refused, as the sale path refuses a body it cannot bind.
func TestCheckBillImportRefusesABodyWithoutRows(t *testing.T) {
	for _, body := range []string{`{}`, `[]`, `not json`} {
		h, mock := newSaleTestHandler(t)
		w := runSaleRequest(t, h.CheckBillImport, "/bill/import/check", body, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("CheckBillImport(%q) = %d; want 400", body, w.Code)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

// storedScaleIssues reports a value that bill_product would store as zero. A negative price and a quantity that is not positive are
// refused by verifyFinalSale, so they are not reported here.
func TestStoredScaleIssuesReportTheValuesThatStoreAsZero(t *testing.T) {
	line := func(price, quantity string) model.BillProduct {
		return model.BillProduct{Price: decimal.RequireFromString(price), Quantity: decimal.RequireFromString(quantity)}
	}
	priceAt := func(n int) importIssue {
		return importIssue{Code: "line_price", Message: fmt.Sprintf("line %d: price must be at least 0.01", n)}
	}
	quantityAt := func(n int) importIssue {
		return importIssue{Code: "line_quantity", Message: fmt.Sprintf("line %d: quantity must be positive", n)}
	}
	maintenance := importIssue{Code: "line_price", Message: "maintenance_cost must be at least 0.01"}
	cases := map[string]struct {
		request *model.AddBillRequest
		want    []importIssue
	}{
		"a zero price":                                {&model.AddBillRequest{Products: []model.BillProduct{line("0", "1")}}, []importIssue{priceAt(1)}},
		"a sub-cent price on the second line":         {&model.AddBillRequest{Products: []model.BillProduct{line("10", "1"), line("0.004", "1")}}, []importIssue{priceAt(2)}},
		"a price of one cent":                         {&model.AddBillRequest{Products: []model.BillProduct{line("0.01", "1")}}, nil},
		"a quantity that stores as zero":              {&model.AddBillRequest{Products: []model.BillProduct{line("10", "0.0004")}}, []importIssue{quantityAt(1)}},
		"a quantity of one thousandth":                {&model.AddBillRequest{Products: []model.BillProduct{line("10", "0.001")}}, nil},
		"a sub-cent maintenance cost":                 {&model.AddBillRequest{MaintenanceCost: decimal.RequireFromString("0.004")}, []importIssue{maintenance}},
		"a maintenance cost of one cent":              {&model.AddBillRequest{MaintenanceCost: decimal.RequireFromString("0.01")}, nil},
		"a negative price is left to verifyFinalSale": {&model.AddBillRequest{Products: []model.BillProduct{line("-1", "1")}}, nil},
		"a zero quantity is left to verifyFinalSale":  {&model.AddBillRequest{Products: []model.BillProduct{line("10", "0")}}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := storedScaleIssues(tc.request); !slices.Equal(got, tc.want) {
				t.Fatalf("storedScaleIssues = %+v; want %+v", got, tc.want)
			}
		})
	}
}

// saleTotal is the total of the stored lines: each price and quantity at its column's scale, rounded per line, plus 15 percent VAT.
func TestSaleTotalMatchesTheStoredLines(t *testing.T) {
	lines := []model.BillProduct{
		{Price: decimal.RequireFromString("10"), Quantity: decimal.RequireFromString("1")},     // 10.00 and 1.50 VAT
		{Price: decimal.RequireFromString("10.333"), Quantity: decimal.RequireFromString("3")}, // 10.33 times 3 is 30.99, and 4.65 VAT
		{Price: decimal.RequireFromString("5.555"), Quantity: decimal.RequireFromString("1")},  // 5.56 and 0.83 VAT
	}
	if got, want := saleTotal(lines), decimal.RequireFromString("53.53"); !got.Equal(want) {
		t.Fatalf("saleTotal = %s; want %s", got, want)
	}
}

// A sale older than 24 hours is valid, with a warning. The check runs on the real clock, so this test keeps a margin of a minute. The exact
// boundary is tested on dateIssues with a fixed clock, in bill_dates_test.go.
func TestCheckBillImportWarnsOnASaleOlderThan24HoursAndKeepsItValid(t *testing.T) {
	body := withEffectiveDate(importSale(), time.Now().Add(-24*time.Hour-time.Minute).UTC().Format(time.RFC3339))
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	expectStoreBranch(mock, 7)
	expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
	expectImportDuplicate(mock, fingerprintOf(t, body), nil)

	rows := checkImport(t, h, importBody(body))
	row := rows[0]
	if !row.Valid || len(row.Errors) != 0 || !hasImportCode(row.Warnings, importDateOutsideWindow) {
		t.Fatalf("row = %+v; want a valid row with date_outside_reporting_window as its only finding", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A sale dated today is valid. The check reads the branch and the duplicate, as for any valid sale, and no date issue is reported.
func TestCheckBillImportAcceptsASaleDatedToday(t *testing.T) {
	body := withEffectiveDate(importSale(), dayAtNoonUTC(time.Now()))
	h, mock := newSaleTestHandler(t)
	expectImportStart(mock, "disable")
	expectStoreBranch(mock, 7)
	expectImportBranch(mock, "300000000000003", "egs-uuid-7", true)
	expectImportDuplicate(mock, fingerprintOf(t, body), nil)

	rows := checkImport(t, h, importBody(body))
	if !rows[0].Valid || len(rows[0].Errors) != 0 {
		t.Fatalf("row = %+v; want a valid row for a sale dated today", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
