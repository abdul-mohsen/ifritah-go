package goosemigrations

import (
	"context"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// baselineUpSQL returns the Up section of 00001_baseline.sql. Run on its own, it builds a database the way one was built before
// goose: tables only, and no version table.
func baselineUpSQL(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(FS, "00001_baseline.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	const upMarker, downMarker = "-- +goose Up", "-- +goose Down"
	up := strings.Index(text, upMarker)
	if up < 0 {
		t.Fatal("00001_baseline.sql has no Up section")
	}
	end := len(text)
	if down := strings.Index(text, downMarker); down > up {
		end = down
	}
	return text[up+len(upMarker) : end]
}

// TestBaselineDigestsMatchTheMigratedBaseline keeps baseline.digests current with 00001_baseline.sql on each MySQL version the goose
// tests run on. When it fails, regenerate the file with GOOSE_WRITE_DIGESTS=1 and TestWriteBaselineDigests.
func TestBaselineDigestsMatchTheMigratedBaseline(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_digests")
	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatalf("goose up to 00001: %v", err)
	}
	got, err := SchemaDigests(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	want, err := BaselineDigests()
	if err != nil {
		t.Fatal(err)
	}
	if diffs := CompareDigests(got, want); len(diffs) > 0 {
		t.Fatalf("baseline.digests does not match the migrated baseline; regenerate it with GOOSE_WRITE_DIGESTS=1:\n  %s",
			strings.Join(diffs, "\n  "))
	}
}

// TestWriteBaselineDigests regenerates baseline.digests from a database built by the baseline. It runs only when GOOSE_WRITE_DIGESTS
// is set, so an ordinary test run never rewrites the file.
func TestWriteBaselineDigests(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" || os.Getenv("GOOSE_WRITE_DIGESTS") == "" {
		t.Skip("set GOOSE_PARITY_DSN and GOOSE_WRITE_DIGESTS to regenerate baseline.digests")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_write_digests")
	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatalf("goose up to 00001: %v", err)
	}
	digests, err := SchemaDigests(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("baseline.digests", []byte(FormatDigests(digests)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A database built from the baseline without goose is adopted: the baseline is recorded as applied, and up applies the rest.
func TestAdoptRecordsABaselineDatabaseThenUpAppliesTheRest(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_adopt")
	mustExec(t, db, baselineUpSQL(t))
	if err := Adopt(ctx, db, provider); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after adopt: %v", err)
	}
	current, target, err := provider.GetVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if current != target || current < 2 {
		t.Fatalf("after adopt and up the database is at version %d of %d; want the last migration", current, target)
	}
}

// A database that drifted from the baseline is not adopted. The error names the table, and goose records nothing.
func TestAdoptRefusesADriftedDatabase(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_adopt_drift")
	mustExec(t, db, baselineUpSQL(t))
	mustExec(t, db, "ALTER TABLE company ADD COLUMN drift_check INT NULL")

	err := Adopt(ctx, db, provider)
	if err == nil || !strings.Contains(err.Error(), "table company") {
		t.Fatalf("adopt of a drifted database = %v; want an error that names table company", err)
	}
	var tables int
	scanOne(t, db, &tables, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'goose_db_version'")
	if tables != 0 {
		t.Fatal("a refused adopt left goose's version table behind")
	}
}

// A database that goose already manages is refused, so adopt cannot rewrite its history.
func TestAdoptRefusesADatabaseGooseAlreadyManages(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_adopt_managed")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	if err := Adopt(ctx, db, provider); err == nil || !strings.Contains(err.Error(), "run up instead") {
		t.Fatalf("adopt of a managed database = %v; want a refusal that says to run up", err)
	}
}

// An empty database is built with up, not adopted.
func TestAdoptRefusesAnEmptyDatabase(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}
	ctx := context.Background()
	db, provider := openFreshGooseDB(t, dsn, "goose_adopt_empty")
	if err := Adopt(ctx, db, provider); err == nil || !strings.Contains(err.Error(), "no tables") {
		t.Fatalf("adopt of an empty database = %v; want a refusal that says it has no tables", err)
	}
}
