package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/testdb"
)

// noteSaleTables are the tables that the tests in this file write, children first, so that testdb.Open can empty them after each test.
var noteSaleTables = []string{
	"outbox", "credit_note_line", "debit_note_line", "credit_note", "debit_note", "bill_product", "bill",
	"branch_sequence", "store", "branches", "user", "company",
}

// noteSaleFixture is one company with branch 7, store 1 and user 7, which a sale of branch 7 by user 7 needs. It returns a handler on
// the migrated database.
func noteSaleFixture(t *testing.T) (*handler, *sql.DB) {
	t.Helper()
	sqlDB := testdb.Open(t, noteSaleTables...)
	company := noteSaleExec(t, sqlDB, `INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')`)
	noteSaleExec(t, sqlDB, `INSERT INTO branches (id, name, company_id) VALUES (7, 'branch 7', ?)`, company)
	noteSaleExec(t, sqlDB, `INSERT INTO store (id, name, company_id, branch_id) VALUES (1, 'store 1', ?, 7)`, company)
	noteSaleExec(t, sqlDB, "INSERT INTO `user` (id, username, password, company_id) VALUES (7, 'cashier', 'unused', ?)", company)
	h := New(sqlDB, db.New(sqlDB), &ZatcaPublisher{tenantKey: "acme", dbName: "acme_db"})
	return &h, sqlDB
}

// noteSaleExec runs a write and returns its generated id.
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

// noteSaleInt runs a query that returns one integer.
func noteSaleInt(t *testing.T, sqlDB *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := sqlDB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// postSaleOK posts a sale that must be created, and returns the id of its bill as the response gives it.
func postSaleOK(t *testing.T, h *handler, body string) string {
	t.Helper()
	w := runSaleRequest(t, h.AddBill, "/bill", body, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("AddBill = %d %q; want 201", w.Code, w.Body.String())
	}
	return strings.TrimSpace(w.Body.String())
}

// freshRequestID returns a request id that no sale or note holds.
func freshRequestID(t *testing.T) string {
	t.Helper()
	id, err := identity.NewUUID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// saleWrites counts what a sale writes: the bill rows, the branch's last bill number, and the outbox events.
type saleWrites struct {
	bills, lastNumber, events int
}

func countSaleWrites(t *testing.T, sqlDB *sql.DB) saleWrites {
	t.Helper()
	return saleWrites{
		bills:      noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM bill"),
		lastNumber: noteSaleInt(t, sqlDB, "SELECT COALESCE(MAX(`last_value`), 0) FROM `branch_sequence` WHERE branch_id = 7 AND scope = 'bill'"),
		events:     noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM outbox"),
	}
}

// A note holds a request identity on the migrated schema, and a sale that carries it is refused. The refusal writes nothing: no bill row,
// no number from the branch's counter and no outbox event. The note keeps its identity. A sale with a new identity still goes through,
// and repeating it returns its bill, as before.
func TestNoteIdentityRefusesASaleOnTheMigratedSchema(t *testing.T) {
	cases := map[string]struct {
		requestID string
		table     string
		path      string
		handle    func(h *handler) gin.HandlerFunc
		body      func(billID string, lineID int) string
	}{
		"credit note": {
			requestID: creditRequestID,
			table:     "credit_note",
			path:      "/bill/credit",
			handle:    func(h *handler) gin.HandlerFunc { return h.CreditBill },
			body: func(billID string, lineID int) string {
				return fmt.Sprintf(`{"bill_id":%s,"branch_id":7,"pos_id":"pos-1","bill_request_id":"%s","reason":"returned","amount":"11.50","lines":[{"bill_product_id":%d,"quantity":"1","price":"10.00","vat":"15.00"}]}`,
					billID, creditRequestID, lineID)
			},
		},
		"debit note": {
			requestID: debitRequestID,
			table:     "debit_note",
			path:      "/bill/debit",
			handle:    func(h *handler) gin.HandlerFunc { return h.DebitBill },
			body: func(billID string, _ int) string {
				return fmt.Sprintf(`{"bill_id":%s,"branch_id":7,"pos_id":"pos-1","bill_request_id":"%s","reason":"towing","amount":"5.75","lines":[{"name":"Towing","quantity":"1","price":"5.00","vat":"15.00"}]}`,
					billID, debitRequestID)
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, sqlDB := noteSaleFixture(t)
			billID := postSaleOK(t, h, saleBody(1, "pos-1", saleRequestID))
			lineID := noteSaleInt(t, sqlDB, "SELECT id FROM bill_product WHERE bill_id = ?", billID)
			if w := runSaleRequest(t, tc.handle(h), tc.path, tc.body(billID, lineID), nil); w.Code != http.StatusCreated {
				t.Fatalf("%s = %d %q; want 201", tc.path, w.Code, w.Body.String())
			}

			fresh := freshRequestID(t)
			freshBill := postSaleOK(t, h, saleBody(1, "pos-1", fresh))
			if again := postSaleOK(t, h, saleBody(1, "pos-1", fresh)); again != freshBill {
				t.Fatalf("repeat of the sale = bill %s; want the original %s", again, freshBill)
			}

			before := countSaleWrites(t, sqlDB)
			requireNoteConflict(t, runSaleRequest(t, h.AddBill, "/bill", saleBody(1, "pos-1", tc.requestID), nil))
			if after := countSaleWrites(t, sqlDB); after != before {
				t.Fatalf("writes after the refused sale = %+v; want %+v", after, before)
			}
			if n := noteSaleInt(t, sqlDB, "SELECT COUNT(*) FROM bill WHERE pos_id = ? AND bill_request_id = ?", "pos-1", tc.requestID); n != 0 {
				t.Fatalf("bills with the refused identity = %d; want 0", n)
			}
			if n := noteSaleInt(t, sqlDB, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE pos_id = ? AND bill_request_id = ?", tc.table), "pos-1", tc.requestID); n != 1 {
				t.Fatalf("%s rows with the identity = %d; want 1", tc.table, n)
			}
		})
	}
}
