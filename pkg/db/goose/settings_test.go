package goosemigrations

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

var expectedDefaultSettings = map[string]string{
	"whatsapp_enabled":             "false",
	"whatsapp_business_account_id": "",
	"whatsapp_phone_number_id":     "",
	"whatsapp_access_token":        "",
	"whatsapp_api_version":         "v18.0",
	"whatsapp_invoice_message":     "Invoice PDF is attached.",
	"default_markup_percentage":    "20",
}

const defaultSettingsQuery = `SELECT setting_key, COALESCE(value, '')
	FROM settings
	WHERE setting_key IN (
		'whatsapp_enabled',
		'whatsapp_business_account_id',
		'whatsapp_phone_number_id',
		'whatsapp_access_token',
		'whatsapp_api_version',
		'whatsapp_invoice_message',
		'default_markup_percentage'
	)
	ORDER BY setting_key`

func TestFreshDatabaseSeedsDefaultSettings(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}

	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_default_settings")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	assertDefaultSettings(t, ctx, db, expectedDefaultSettings)
}

func TestDefaultSettingsMigrationPreservesExistingValuesAndReplays(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}

	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_default_settings_replay")
	if _, err := provider.UpTo(ctx, 10); err != nil {
		t.Fatalf("goose up to version 10: %v", err)
	}

	mustExec(t, db, `INSERT INTO settings (setting_key, value) VALUES
		('whatsapp_api_version', 'v19.0'),
		('default_markup_percentage', '35')`)

	expected := cloneSettings(expectedDefaultSettings)
	expected["whatsapp_api_version"] = "v19.0"
	expected["default_markup_percentage"] = "35"

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up to version 11: %v", err)
	}
	assertDefaultSettings(t, ctx, db, expected)

	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("goose down from version 11: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose replay version 11: %v", err)
	}
	assertDefaultSettings(t, ctx, db, expected)
}

func assertDefaultSettings(t *testing.T, ctx context.Context, db *sql.DB, expected map[string]string) {
	t.Helper()

	rows, err := db.QueryContext(ctx, defaultSettingsQuery)
	if err != nil {
		t.Fatalf("query default settings: %v", err)
	}
	defer rows.Close()

	got := make(map[string]string, len(expected))
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			t.Fatalf("scan default setting: %v", err)
		}
		got[key] = value
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate default settings: %v", err)
	}

	if len(got) != len(expected) {
		t.Fatalf("default setting count = %d, want %d; got %#v", len(got), len(expected), got)
	}
	for key, want := range expected {
		if got[key] != want {
			t.Errorf("%s = %q, want %q", key, got[key], want)
		}
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM settings WHERE setting_key IN (
		'whatsapp_enabled',
		'whatsapp_business_account_id',
		'whatsapp_phone_number_id',
		'whatsapp_access_token',
		'whatsapp_api_version',
		'whatsapp_invoice_message',
		'default_markup_percentage'
	)`).Scan(&count); err != nil {
		t.Fatalf("count default settings: %v", err)
	}
	if count != len(expected) {
		t.Fatalf("default setting rows = %d, want %d", count, len(expected))
	}
}

func cloneSettings(settings map[string]string) map[string]string {
	clone := make(map[string]string, len(settings))
	for key, value := range settings {
		clone[key] = value
	}
	return clone
}
