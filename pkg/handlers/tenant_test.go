package handlers

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/DATA-DOG/go-sqlmock"
)

const tenantKeyQuery = "GetTenantKey"

var writeStatement = regexp.MustCompile(`(?i)\b(insert|update|replace|delete)\b`)

func newTenantKeyQueries(t *testing.T) (*db.Queries, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db.New(sqlDB), mock
}

func TestLoadTenantKeyReturnsTheCompanyKey(t *testing.T) {
	queries, mock := newTenantKeyQueries(t)
	mock.ExpectQuery("SELECT tenant_key FROM company").
		WillReturnRows(sqlmock.NewRows([]string{"tenant_key"}).AddRow("acme"))

	key, err := LoadTenantKey(context.Background(), queries)
	if err != nil || key != "acme" {
		t.Fatalf("LoadTenantKey = %q, %v; want acme", key, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The server refuses to start without a usable key, and each error names company.tenant_key.
func TestLoadTenantKeyRefusesAMissingOrInvalidKey(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		noRows bool
	}{
		{name: "no company row", noRows: true},
		{name: "empty", key: ""},
		{name: "contains a colon", key: "acme:1"},
		{name: "65 characters", key: strings.Repeat("k", 65)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			queries, mock := newTenantKeyQueries(t)
			rows := sqlmock.NewRows([]string{"tenant_key"})
			if !tc.noRows {
				rows.AddRow(tc.key)
			}
			mock.ExpectQuery("SELECT tenant_key FROM company").WillReturnRows(rows)

			if _, err := LoadTenantKey(context.Background(), queries); err == nil || !strings.Contains(err.Error(), "company.tenant_key") {
				t.Fatalf("LoadTenantKey = %v; want an error that names company.tenant_key", err)
			}
		})
	}
}

// Deployment owns company.tenant_key, so no query may write it. Only GetTenantKey may name it, and
// it must read it.
func TestQueriesNeverWriteTheTenantKey(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "db", "queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("query files not found: %v", err)
	}
	found := false
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range strings.Split(string(body), "-- name:")[1:] {
			header, text, _ := strings.Cut(block, "\n")
			name, _, _ := strings.Cut(strings.TrimSpace(header), " ")
			if name == tenantKeyQuery {
				found = true
			}
			if !strings.Contains(strings.ToLower(text), "tenant_key") {
				continue
			}
			if name != tenantKeyQuery || writeStatement.MatchString(text) {
				t.Errorf("%s: query %s names tenant_key and is not a read of it; deployment owns that column", filepath.Base(file), name)
			}
		}
	}
	if !found {
		t.Fatalf("%s is not in the query files", tenantKeyQuery)
	}
}
