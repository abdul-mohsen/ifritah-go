package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func newSupplierTestHandler(t *testing.T) (*handler, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	h := New(sqlDB, db.New(sqlDB), nil)
	return &h, mock, func() { _ = sqlDB.Close() }
}

func runSupplierSearchRequest(t *testing.T, handlerFunc gin.HandlerFunc, query string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/supplier/search?q="+query, nil)
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "testuser"})

	handlerFunc(c)
	return w
}

func TestSearchSuppliers(t *testing.T) {
	testCases := []struct {
		name      string
		query     string
		rows      [][]interface{}
		wantCount int
		wantError bool
	}{
		{
			name:  "empty query returns all",
			query: "",
			rows: [][]interface{}{
				{int64(1), "Electronics Supplier", "ELEC-001"},
				{int64(2), "Parts Co", "PARTS-001"},
			},
			wantCount: 2,
		},
		{
			name:  "substring match electronics",
			query: "elec",
			rows: [][]interface{}{
				{int64(1), "Electronics Supplier", "ELEC-001"},
			},
			wantCount: 1,
		},
		{
			name:  "code substring match",
			query: "PARTS",
			rows: [][]interface{}{
				{int64(2), "Parts Co", "PARTS-001"},
			},
			wantCount: 1,
		},
		{
			name:      "no matches",
			query:     "nonexistent",
			rows:      [][]interface{}{},
			wantCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, mock, cleanup := newSupplierTestHandler(t)
			defer cleanup()

			// Mock GetCompanyIdByUser (just match the SELECT part)
			companyID := int32(100)
			mock.ExpectQuery("SELECT company_id FROM user where id = \\?").
				WithArgs(int32(7)).
				WillReturnRows(sqlmock.NewRows([]string{"company_id"}).AddRow(companyID))

			// Mock SearchSuppliers (match SQL pattern without comment)
			rowsBuilder := sqlmock.NewRows([]string{"id", "name", "code"})
			for _, row := range tc.rows {
				rowsBuilder.AddRow(row[0], row[1], row[2])
			}
			mock.ExpectQuery("SELECT id, name, number as code FROM supplier").
				WithArgs(companyID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int32(100)).
				WillReturnRows(rowsBuilder)

			w := runSupplierSearchRequest(t, h.SearchSuppliers, tc.query)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
			}

			var results []model.SearchSupplierResponse
			if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}

			if len(results) != tc.wantCount {
				t.Fatalf("got %d results, want %d; results=%+v", len(results), tc.wantCount, results)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet SQL expectations: %v", err)
			}
		})
	}
}
