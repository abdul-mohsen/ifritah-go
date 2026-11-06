package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/testdb"

	"github.com/shopspring/decimal"
)

func noteSaleExec(t *testing.T, sqlDB *sql.DB, query string, args ...any) int64 {
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

func noteSaleInt(t *testing.T, sqlDB *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := sqlDB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func postSaleOK(t *testing.T, h *handler, body string) string {
	t.Helper()
	w := runSaleRequest(t, h.AddBill, "/bill", body, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("AddBill = %d %q; want 201", w.Code, w.Body.String())
	}
	return strings.TrimSpace(w.Body.String())
}

func freshRequestID(t *testing.T) string {
	t.Helper()
	id, err := identity.NewUUID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

var saleProductOwnershipTables = []string{
	"stock_movements",
	"credit_note_line",
	"debit_note_line",
	"bill_payment",
	"bill_product",
	"credit_note",
	"debit_note",
	"zatca_submission",
	"outbox",
	"bill",
	"product",
	"settings",
	"branch_sequence",
	"store",
	"branches",
	"user",
	"company",
}

func saleProductOwnershipFixture(t *testing.T) (*handler, *sql.DB) {
	t.Helper()
	sqlDB := testdb.Open(t, saleProductOwnershipTables...)
	company := noteSaleExec(t, sqlDB, `INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')`)
	noteSaleExec(t, sqlDB, `INSERT INTO branches (id, name, company_id) VALUES (7, 'branch 7', ?), (8, 'branch 8', ?)`, company, company)
	noteSaleExec(t, sqlDB, `INSERT INTO store (id, name, company_id, branch_id) VALUES (1, 'store 1', ?, 7), (2, 'store 2', ?, 8)`, company, company)
	noteSaleExec(t, sqlDB, "INSERT INTO `user` (id, username, password, company_id) VALUES (7, 'cashier', 'unused', ?)", company)
	h := New(sqlDB, db.New(sqlDB), &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"})
	return &h, sqlDB
}

func addSaleProduct(t *testing.T, sqlDB *sql.DB, productID uint64, storeID int32, quantity string, deleted bool) {
	t.Helper()
	_, err := sqlDB.Exec(
		`INSERT INTO product (id, store_id, price, quantity, is_deleted, name) VALUES (?, ?, '10.00', ?, ?, 'catalog product')`,
		productID, storeID, quantity, deleted,
	)
	if err != nil {
		t.Fatalf("insert product %d: %v", productID, err)
	}
}

func enableSaleStock(t *testing.T, sqlDB *sql.DB) {
	t.Helper()
	noteSaleExec(t, sqlDB, `INSERT INTO settings (setting_key, value) VALUES ('stock_enforcement', 'enforce')`)
}

func saleProductOwnershipBody(requestID string, productIDs []uint64, includeManual bool) string {
	lines := make([]string, 0, len(productIDs))
	for _, productID := range productIDs {
		lines = append(lines, fmt.Sprintf(`{"product_id":%d,"name":"catalog product","price":"10","quantity":"1","discount":"0"}`, productID))
	}
	manualLines := "[]"
	if includeManual {
		manualLines = `[{"product_id":999999999,"name":"manual line","price":"2","quantity":"1","discount":"0"}]`
	}
	return fmt.Sprintf(
		`{"store_id":1,"state":1,"discount":"0","maintenance_cost":"0","payment_method":1,"branch_id":7,"pos_id":"pos-1","bill_request_id":"%s","products":[%s],"manual_products":%s}`,
		requestID, strings.Join(lines, ","), manualLines,
	)
}

func saleProductQuantity(t *testing.T, sqlDB *sql.DB, productID uint64) decimal.Decimal {
	t.Helper()
	var quantity decimal.Decimal
	if err := sqlDB.QueryRow(`SELECT quantity FROM product WHERE id = ?`, productID).Scan(&quantity); err != nil {
		t.Fatalf("read product %d quantity: %v", productID, err)
	}
	return quantity
}

func TestSaleAllowsSameStoreCatalogAndManualLinesAndUsesSaleStoreForMovement(t *testing.T) {
	h, sqlDB := saleProductOwnershipFixture(t)
	addSaleProduct(t, sqlDB, 1001, 1, "5.000", false)
	enableSaleStock(t, sqlDB)

	billID := postSaleOK(t, h, saleProductOwnershipBody(freshRequestID(t), []uint64{1001}, true))

	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM bill_product WHERE bill_id = ?", billID); got != 2 {
		t.Fatalf("bill product rows = %d; want catalog and manual lines", got)
	}
	if got := saleProductQuantity(t, sqlDB, 1001); !got.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("product quantity = %s; want 4", got)
	}

	var movementStore int32
	var movementQuantity decimal.Decimal
	if err := sqlDB.QueryRow(
		`SELECT store_id, quantity FROM stock_movements WHERE product_id = ? AND movement_type = 'sale'`,
		1001,
	).Scan(&movementStore, &movementQuantity); err != nil {
		t.Fatalf("read sale movement: %v", err)
	}
	if movementStore != 1 {
		t.Fatalf("sale movement store = %d; want selected store 1", movementStore)
	}
	if !movementQuantity.Equal(decimal.NewFromInt(-1)) {
		t.Fatalf("sale movement quantity = %s; want -1", movementQuantity)
	}
}

func TestSaleRejectsCrossBranchProductWithoutWrites(t *testing.T) {
	h, sqlDB := saleProductOwnershipFixture(t)
	addSaleProduct(t, sqlDB, 1002, 2, "5.000", false)
	enableSaleStock(t, sqlDB)

	w := runSaleRequest(t, h.AddBill, "/bill", saleProductOwnershipBody(freshRequestID(t), []uint64{1002}, false), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "does not belong to store") {
		t.Fatalf("cross-branch product sale = %d %q; want 400 about product store", w.Code, w.Body.String())
	}
	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM bill"); got != 0 {
		t.Fatalf("bill rows after cross-branch rejection = %d; want 0", got)
	}
	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM bill_product"); got != 0 {
		t.Fatalf("bill product rows after cross-branch rejection = %d; want 0", got)
	}
	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM stock_movements"); got != 0 {
		t.Fatalf("stock movements after cross-branch rejection = %d; want 0", got)
	}
	if got := saleProductQuantity(t, sqlDB, 1002); !got.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("cross-branch product quantity = %s; want 5", got)
	}
}

func TestSaleRejectsMissingProductWithoutWrites(t *testing.T) {
	h, sqlDB := saleProductOwnershipFixture(t)

	w := runSaleRequest(t, h.AddBill, "/bill", saleProductOwnershipBody(freshRequestID(t), []uint64{1003}, false), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "not found") {
		t.Fatalf("missing product sale = %d %q; want 400 about missing product", w.Code, w.Body.String())
	}
	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM bill"); got != 0 {
		t.Fatalf("bill rows after missing product rejection = %d; want 0", got)
	}
}

func TestSaleRejectsDeletedProductWithoutWrites(t *testing.T) {
	h, sqlDB := saleProductOwnershipFixture(t)
	addSaleProduct(t, sqlDB, 1004, 1, "5.000", true)

	w := runSaleRequest(t, h.AddBill, "/bill", saleProductOwnershipBody(freshRequestID(t), []uint64{1004}, false), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "deleted") {
		t.Fatalf("deleted product sale = %d %q; want 400 about deleted product", w.Code, w.Body.String())
	}
	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM bill"); got != 0 {
		t.Fatalf("bill rows after deleted product rejection = %d; want 0", got)
	}
}

func TestSaleRollsBackMixedSameStoreAndCrossStoreLines(t *testing.T) {
	h, sqlDB := saleProductOwnershipFixture(t)
	addSaleProduct(t, sqlDB, 1005, 1, "5.000", false)
	addSaleProduct(t, sqlDB, 1006, 2, "5.000", false)
	enableSaleStock(t, sqlDB)

	w := runSaleRequest(t, h.AddBill, "/bill", saleProductOwnershipBody(freshRequestID(t), []uint64{1005, 1006}, false), nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "does not belong to store") {
		t.Fatalf("mixed product sale = %d %q; want 400 about product store", w.Code, w.Body.String())
	}
	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM bill"); got != 0 {
		t.Fatalf("bill rows after mixed rejection = %d; want 0", got)
	}
	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM bill_product"); got != 0 {
		t.Fatalf("bill product rows after mixed rejection = %d; want 0", got)
	}
	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM stock_movements"); got != 0 {
		t.Fatalf("stock movements after mixed rejection = %d; want 0", got)
	}
	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM branch_sequence"); got != 0 {
		t.Fatalf("branch sequence rows after mixed rejection = %d; want 0", got)
	}
	if got := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM outbox"); got != 0 {
		t.Fatalf("outbox rows after mixed rejection = %d; want 0", got)
	}
	if got := saleProductQuantity(t, sqlDB, 1005); !got.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("same-store product quantity after mixed rejection = %s; want 5", got)
	}
	if got := saleProductQuantity(t, sqlDB, 1006); !got.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("cross-store product quantity after mixed rejection = %s; want 5", got)
	}
}
