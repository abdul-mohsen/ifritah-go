package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
)

func newStoreHandler(t *testing.T) (*handler, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	h := New(sqlDB, db.New(sqlDB), nil)
	return &h, mock
}

func storeRequest(t *testing.T, role string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v2/store", strings.NewReader(""))
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "store-user", Role: role})
	c.Set("userId", int64(7))
	c.Set("user_role", role)
	return c, recorder
}

func expectStoreScopeQuery(mock sqlmock.Sqlmock, role string, rows *sqlmock.Rows) {
	mock.ExpectQuery(`SELECT s\.id, s\.addressId, s\.name`).
		WithArgs(int32(7), role, role, role, role, role).
		WillReturnRows(rows)
}

func TestGetStoresRoleVisibility(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		storeIDs []int
	}{
		{name: "admin sees company stores", role: RoleAdmin, storeIDs: []int{1, 2}},
		{name: "owner sees company stores", role: "owner", storeIDs: []int{1, 2}},
		{name: "manager sees managed branch stores", role: RoleManager, storeIDs: []int{2}},
		{name: "employee sees assigned company stores", role: RoleEmployee, storeIDs: []int{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, mock := newStoreHandler(t)
			rows := sqlmock.NewRows([]string{"id", "addressId", "name"})
			for _, id := range tt.storeIDs {
				rows.AddRow(int32(id), nil, "Store "+string(rune('A'+id-1)))
			}
			expectStoreScopeQuery(mock, tt.role, rows)

			c, recorder := storeRequest(t, tt.role)
			h.GetStores(c)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			var response pagination.Envelope[Store]
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(response.Items) != len(tt.storeIDs) {
				t.Fatalf("stores = %#v, want %v stores", response.Items, len(tt.storeIDs))
			}
			for i, wantID := range tt.storeIDs {
				if response.Items[i].Id != wantID {
					t.Errorf("store[%d].id = %d, want %d", i, response.Items[i].Id, wantID)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGetStoresPropagatesStoreScopeQueryError(t *testing.T) {
	h, mock := newStoreHandler(t)
	mock.ExpectQuery(`SELECT s\.id, s\.addressId, s\.name`).
		WithArgs(int32(7), RoleManager, RoleManager, RoleManager, RoleManager, RoleManager).
		WillReturnError(errors.New("store scope unavailable"))

	c, recorder := storeRequest(t, RoleManager)
	h.GetStores(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("query error returned success-shaped empty response: %s", recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetStoresPropagatesLegacyStoreScopeQueryError(t *testing.T) {
	h, mock := newStoreHandler(t)
	mock.ExpectQuery(`SELECT s\.id, s\.addressId, s\.name`).
		WithArgs(int32(7), nil, nil, nil, nil, nil).
		WillReturnError(errors.New("legacy store scope unavailable"))

	c, recorder := storeRequest(t, "")
	h.GetStores(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("query error returned success-shaped empty response: %s", recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetStoresPropagatesStoreScopeScanError(t *testing.T) {
	h, mock := newStoreHandler(t)
	rows := sqlmock.NewRows([]string{"id", "addressId", "name"}).
		AddRow("not-an-id", nil, "Broken store")
	expectStoreScopeQuery(mock, RoleEmployee, rows)

	c, recorder := storeRequest(t, RoleEmployee)
	h.GetStores(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("scan error returned success-shaped empty response: %s", recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
