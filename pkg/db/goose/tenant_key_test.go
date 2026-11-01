package goosemigrations

import (
	"context"
	"os"
	"testing"
)

// TestCompanyTenantKeyContract checks what 00007 adds to company. The column defaults to the empty
// string, which means not set, and it compares exactly, as zatca_master does. It needs a MySQL server.
func TestCompanyTenantKeyContract(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}

	db, provider := openFreshGooseDB(t, dsn, "goose_tenant_key")
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	result, err := db.Exec("INSERT INTO company (name, vat_number) VALUES ('acme', '300000000000003')")
	if err != nil {
		t.Fatalf("insert company: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("company id: %v", err)
	}

	var key string
	if err := db.QueryRow("SELECT tenant_key FROM company WHERE id = ?", id).Scan(&key); err != nil || key != "" {
		t.Fatalf("new company tenant_key = %q, %v; want the empty default", key, err)
	}
	if _, err := db.Exec("UPDATE company SET tenant_key = 'Acme' WHERE id = ?", id); err != nil {
		t.Fatalf("set tenant_key: %v", err)
	}
	matches := func(value string) int {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM company WHERE id = ? AND tenant_key = ?", id, value).Scan(&n); err != nil {
			t.Fatalf("compare tenant_key with %q: %v", value, err)
		}
		return n
	}
	exact, folded := matches("Acme"), matches("acme")
	if exact != 1 || folded != 0 {
		t.Fatalf("tenant_key comparison is not exact: Acme matched %d rows, acme matched %d; want 1 and 0", exact, folded)
	}
}
