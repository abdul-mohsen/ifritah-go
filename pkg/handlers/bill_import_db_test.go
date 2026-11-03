package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/testdb"
)

// importTables are the tables that the database tests of the import check write, children first, so that testdb.Open can empty them
// after each test. The SAR currency row comes with the migrations, and it is not emptied.
var importTables = []string{
	"stock_movements", "outbox", "bill_payment", "bill_product", "bill", "branch_sequence", "settings", "product", "store",
	"branch_zatca_config", "branches", "user", "company",
}

// importFixture is one company with branch 7, which has an EGS id, store 1 on that branch, and user 7, who can sell from store 1.
// It returns a handler on the migrated database.
func importFixture(t *testing.T) (*handler, *sql.DB) {
	t.Helper()
	sqlDB := testdb.Open(t, importTables...)
	company := importExec(t, sqlDB, "INSERT INTO company (name, vat_number, vat_registration_number) VALUES ('acme', '300000000000003', '300000000000003')")
	importExec(t, sqlDB, "INSERT INTO branches (id, name, company_id) VALUES (7, 'branch 7', ?)", company)
	importExec(t, sqlDB, "INSERT INTO branch_zatca_config (branch_id, csr_computer_number, zatca_production_username, zatca_production_password) VALUES (7, 'egs-7', 'prod-user-7', 'prod-pass-7')")
	importExec(t, sqlDB, "INSERT INTO store (id, name, company_id, branch_id) VALUES (1, 'store 1', ?, 7)", company)
	importExec(t, sqlDB, "INSERT INTO `user` (id, username, password, company_id) VALUES (7, 'cashier', 'unused', ?)", company)
	h := New(sqlDB, db.New(sqlDB), &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"})
	return &h, sqlDB
}

// importExec runs a write and returns its generated id.
func importExec(t *testing.T, sqlDB *sql.DB, query string, args ...any) int64 {
	t.Helper()
	res, err := sqlDB.Exec(query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// importText runs a query that returns one value, as text.
func importText(t *testing.T, sqlDB *sql.DB, query string, args ...any) string {
	t.Helper()
	var s string
	if err := sqlDB.QueryRow(query, args...).Scan(&s); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return s
}

// importWrites counts what a sale writes and a check must not: the bill rows, the bill lines, the payments, the outbox events, the
// stock movements, and the numbers taken from the branch counters.
type importWrites struct {
	bills, lines, payments, events, movements, numbers string
}

func countImportWrites(t *testing.T, sqlDB *sql.DB) importWrites {
	t.Helper()
	return importWrites{
		bills:     importText(t, sqlDB, "SELECT COUNT(*) FROM bill"),
		lines:     importText(t, sqlDB, "SELECT COUNT(*) FROM bill_product"),
		payments:  importText(t, sqlDB, "SELECT COUNT(*) FROM bill_payment"),
		events:    importText(t, sqlDB, "SELECT COUNT(*) FROM outbox"),
		movements: importText(t, sqlDB, "SELECT COUNT(*) FROM stock_movements"),
		numbers:   importText(t, sqlDB, "SELECT COALESCE(SUM(`last_value`), 0) FROM `branch_sequence`"),
	}
}

// saveImportSale saves a final sale through the sale path, and returns its bill id.
func saveImportSale(t *testing.T, h *handler, body string) int64 {
	t.Helper()
	w := runSaleRequest(t, h.AddBill, "/bill", body, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("AddBill = %d %q; want 201", w.Code, w.Body.String())
	}
	id, err := strconv.ParseInt(strings.TrimSpace(w.Body.String()), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// importSaleIdentity is the point-of-sale identity of the sales in these tests.
const importSaleIdentity = `"pos_id":"pos-1","bill_request_id":"` + saleRequestID + `",`

// A row that matches a sale saved through the sale path is flagged with that bill's id, and a check writes nothing. The row's content
// has no identity, so the match is by content, as it is for a sale typed in the form.
func TestCheckBillImportFlagsTheSavedSaleOnTheMigratedSchema(t *testing.T) {
	h, sqlDB := importFixture(t)
	billID := saveImportSale(t, h, saleBody(1, "pos-1", saleRequestID))
	before := countImportWrites(t, sqlDB)

	changed := strings.Replace(importSale(), `"price":"10"`, `"price":"11"`, 1)
	rows := checkImport(t, h, importBody(importSale(), changed))

	if !rows[0].Valid || rows[0].DuplicateOf == nil || *rows[0].DuplicateOf != uint64(billID) || !hasImportCode(rows[0].Warnings, importDuplicateSale) {
		t.Fatalf("saved sale row = %+v; want a valid row that duplicates bill %d", rows[0], billID)
	}
	if !rows[1].Valid || rows[1].DuplicateOf != nil {
		t.Fatalf("changed row = %+v; want a valid row with no duplicate", rows[1])
	}
	if after := countImportWrites(t, sqlDB); after != before {
		t.Fatalf("writes after the check = %+v; want %+v", after, before)
	}
}

// A payment is refused above the bill's total, which the database computes from the lines. The check's total is the same: a payment
// equal to the stored total is accepted, and one cent more is refused. The lines have values with more decimals than their columns keep.
func TestCheckBillImportPaymentLimitIsTheStoredTotal(t *testing.T) {
	h, sqlDB := importFixture(t)
	const sale = `{"store_id":1,"state":1,"discount":"0","maintenance_cost":"5.555","payment_method":1,"branch_id":7,` +
		importSaleIdentity + `"products":[{"product_id":null,"name":"Brake pad","price":"10.333","quantity":"3","discount":"0"},` +
		`{"product_id":null,"name":"Oil","price":"10","quantity":"1","discount":"0"}],"manual_products":[]}`
	billID := saveImportSale(t, h, sale)
	total := importText(t, sqlDB, "SELECT total FROM bill WHERE id = ?", billID)
	plusCent := decimal.RequireFromString(total).Add(decimal.RequireFromString("0.01")).String()

	content := strings.Replace(sale, importSaleIdentity, "", 1)
	rows := checkImport(t, h, importBody(withPaidAmount(content, total), withPaidAmount(content, plusCent)))

	if !rows[0].Valid || len(rows[0].Errors) != 0 {
		t.Fatalf("a payment of the stored total %s = %+v; want a valid row", total, rows[0])
	}
	if rows[1].Valid || !hasImportCode(rows[1].Errors, "paid_amount_above_total") {
		t.Fatalf("a payment of %s = %+v; want paid_amount_above_total against the stored total %s", plusCent, rows[1], total)
	}
}

// The stock check agrees with the sale in enforce mode and in warn mode. A line the stock cannot cover is refused by the check, and the
// sale refuses it and writes nothing. A line the stock covers is valid, and the sale saves it.
func TestCheckBillImportStockAgreesWithTheSale(t *testing.T) {
	for _, mode := range []string{"enforce", "warn"} {
		t.Run(mode, func(t *testing.T) {
			h, sqlDB := importFixture(t)
			importExec(t, sqlDB, "INSERT INTO settings (setting_key, value) VALUES ('stock_enforcement', ?)", mode)
			short := importExec(t, sqlDB, "INSERT INTO product (store_id, price, quantity, name) VALUES (1, 10.00, 1.000, 'Brake pad')")
			enough := importExec(t, sqlDB, "INSERT INTO product (store_id, price, quantity, name) VALUES (1, 10.00, 3.000, 'Oil')")

			shortSale := importStockSale(int(short), "10", "2")
			before := countImportWrites(t, sqlDB)
			rows := checkImport(t, h, importBody(shortSale))
			if rows[0].Valid || !hasImportCode(rows[0].Errors, importStockShort) {
				t.Fatalf("short line = %+v; want the stock error", rows[0])
			}
			w := runSaleRequest(t, h.AddBill, "/bill", strings.Replace(shortSale, `"store_id":1,`, `"store_id":1,`+importSaleIdentity, 1), nil)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), importStockShort) {
				t.Fatalf("AddBill short = %d %q; want 400 with %s, as the check says", w.Code, w.Body.String(), importStockShort)
			}
			if after := countImportWrites(t, sqlDB); after != before {
				t.Fatalf("writes after a refused sale = %+v; want %+v", after, before)
			}

			enoughSale := importStockSale(int(enough), "10", "2")
			rows = checkImport(t, h, importBody(enoughSale))
			if !rows[0].Valid {
				t.Fatalf("enough line = %+v; want a valid row", rows[0])
			}
			saveImportSale(t, h, strings.Replace(enoughSale, `"store_id":1,`, `"store_id":1,`+importSaleIdentity, 1))
		})
	}
}

// The daemon runs a branch that is active and has a production username and password. The check refuses a row on a branch that the
// daemon does not run, and it accepts a row on one it runs. Each state is set on the migrated schema in turn, and then checked.
func TestCheckBillImportRefusesABranchTheDaemonDoesNotRunOnTheMigratedSchema(t *testing.T) {
	h, sqlDB := importFixture(t)
	steps := []struct {
		name    string
		exec    []string
		refused bool
	}{
		{name: "active, with credentials", refused: false},
		{name: "no production username", exec: []string{"UPDATE branch_zatca_config SET zatca_production_username = '' WHERE branch_id = 7"}, refused: true},
		{name: "no production password", exec: []string{
			"UPDATE branch_zatca_config SET zatca_production_username = 'prod-user-7', zatca_production_password = '' WHERE branch_id = 7",
		}, refused: true},
		{name: "not active, with credentials", exec: []string{
			"UPDATE branch_zatca_config SET zatca_production_password = 'prod-pass-7' WHERE branch_id = 7",
			"UPDATE branches SET is_active = 0 WHERE id = 7",
		}, refused: true},
		{name: "active again", exec: []string{"UPDATE branches SET is_active = 1 WHERE id = 7"}, refused: false},
		{name: "no ZATCA config row", exec: []string{"DELETE FROM branch_zatca_config WHERE branch_id = 7"}, refused: true},
	}
	for _, step := range steps {
		for _, query := range step.exec {
			importExec(t, sqlDB, query)
		}
		rows := checkImport(t, h, importBody(importSale()))
		if hasImportCode(rows[0].Errors, importBranchNotRegistered) != step.refused {
			t.Fatalf("%s: row = %+v; want branch_not_registered %v", step.name, rows[0], step.refused)
		}
		if !step.refused && !rows[0].Valid {
			t.Fatalf("%s: row = %+v; want a valid row on a branch the daemon runs", step.name, rows[0])
		}
	}
}

// The sale path saves the effective date as the UTC wall clock, to the second, and the daemon reads that text back. The check reads the
// same text. A sale with an offset is saved in UTC, and the saved text is the one the date rules read. A sale dated in the future is
// saved, because the sale path does not refuse it, and the check flags it.
func TestSavedEffectiveDateIsTheTextTheDaemonReads(t *testing.T) {
	h, sqlDB := importFixture(t)
	billID := saveImportSale(t, h, withEffectiveDate(saleBody(1, "pos-1", saleRequestID), "2026-11-03T10:00:00.6+03:00"))
	saved := importText(t, sqlDB, "SELECT CAST(effective_date AS CHAR) FROM bill WHERE id = ?", billID)
	if saved != "2026-11-03 07:00:01" {
		t.Fatalf("saved effective date = %q; want the UTC wall clock to the second, 2026-11-03 07:00:01", saved)
	}
	issue, err := parseDaemonDate(saved)
	if err != nil || issue.Format(dateLayout) != "2026-11-03" {
		t.Fatalf("the daemon reads %q as %v, %v; want the day 2026-11-03", saved, issue, err)
	}
	if issues := dateIssues(saved, time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)); len(issues) != 0 {
		t.Fatalf("date issues for %q = %+v; want none", saved, issues)
	}

	future := dayAtNoonUTC(time.Now().Add(36 * time.Hour))
	futureID, err := identity.NewUUID()
	if err != nil {
		t.Fatal(err)
	}
	saveImportSale(t, h, withEffectiveDate(saleBody(1, "pos-2", futureID), future))
	rows := checkImport(t, h, importBody(withEffectiveDate(importSale(), future)))
	if rows[0].Valid || !hasImportCode(rows[0].Errors, importDateInFuture) {
		t.Fatalf("future sale row = %+v; want an invalid row with date_in_future", rows[0])
	}
}
