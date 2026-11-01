package goosemigrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
)

// TestBaselineMatchesLegacySchema applies the legacy SQL files and the goose baseline (version 1)
// to two fresh databases and requires identical table definitions. Later migrations are not
// part of this comparison. It needs a MySQL server: GOOSE_PARITY_DSN must set multiStatements=true,
// and the account must be allowed to create and drop goose_parity_legacy and goose_parity_baseline.
func TestBaselineMatchesLegacySchema(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}

	admin := openTestDB(t, dsn, "")
	defer admin.Close()
	const legacyName = "goose_parity_legacy"
	const baselineName = "goose_parity_baseline"
	for _, name := range []string{legacyName, baselineName} {
		mustExec(t, admin, "DROP DATABASE IF EXISTS `"+name+"`")
		mustExec(t, admin, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	}

	legacy := openTestDB(t, dsn, legacyName)
	defer legacy.Close()
	applyLegacySQL(t, legacy)

	baseline := openTestDB(t, dsn, baselineName)
	defer baseline.Close()
	provider, err := goose.NewProvider(goose.DialectMySQL, baseline, FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(context.Background(), BaselineVersion); err != nil {
		t.Fatalf("goose up to the baseline: %v", err)
	}

	compareTableDefinitions(t, legacy, baseline)
}

// applyLegacySQL replays the order used by the existing replay check: schema.sql, then the
// numbered migrations, then the runtime migrations, each sorted by file name.
func applyLegacySQL(t *testing.T, db *sql.DB) {
	t.Helper()
	files := []string{filepath.Join("..", "schema", "schema.sql")}
	files = append(files, sortedSQLFiles(t, filepath.Join("..", "migrations"))...)
	files = append(files, sortedSQLFiles(t, filepath.Join("..", "runtime_migrations"))...)
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatalf("apply %s: %v", file, err)
		}
	}
}

func sortedSQLFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// compareTableDefinitions compares SHOW CREATE TABLE output for every base table. It ignores
// goose_db_version, which only the baseline database has.
func compareTableDefinitions(t *testing.T, legacy, baseline *sql.DB) {
	t.Helper()
	want := tableDefinitions(t, legacy)
	got := tableDefinitions(t, baseline)
	for name, definition := range want {
		other, ok := got[name]
		switch {
		case !ok:
			t.Errorf("baseline is missing table %s", name)
		case other != definition:
			t.Errorf("table %s differs\nlegacy:\n%s\nbaseline:\n%s", name, definition, other)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("baseline has the extra table %s", name)
		}
	}
}

func tableDefinitions(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query("SHOW FULL TABLES WHERE Table_type = 'BASE TABLE'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name, tableType string
		if err := rows.Scan(&name, &tableType); err != nil {
			t.Fatal(err)
		}
		if name != "goose_db_version" {
			names = append(names, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	definitions := make(map[string]string, len(names))
	for _, name := range names {
		var table, definition string
		query := fmt.Sprintf("SHOW CREATE TABLE `%s`", name)
		if err := db.QueryRow(query).Scan(&table, &definition); err != nil {
			t.Fatal(err)
		}
		// SHOW CREATE TABLE names the charset only when a column sets it explicitly. The legacy
		// schema inherits utf8mb4 from the table default, while the dumped baseline states it.
		// The two are equivalent, so the redundant clause is dropped before comparing.
		definitions[name] = strings.ReplaceAll(definition, " CHARACTER SET utf8mb4 COLLATE ", " COLLATE ")
	}
	return definitions
}

func openTestDB(t *testing.T, dsn, name string) *sql.DB {
	t.Helper()
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	return db
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// openFreshGooseDB recreates database name, opens it, and returns it with a goose provider for
// FS. The database is dropped when the test ends.
func openFreshGooseDB(t *testing.T, dsn, name string) (*sql.DB, *goose.Provider) {
	t.Helper()
	admin := openTestDB(t, dsn, "")
	t.Cleanup(func() { admin.Close() })
	mustExec(t, admin, "DROP DATABASE IF EXISTS `"+name+"`")
	mustExec(t, admin, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	t.Cleanup(func() { mustExec(t, admin, "DROP DATABASE IF EXISTS `"+name+"`") })

	db := openTestDB(t, dsn, name)
	t.Cleanup(func() { db.Close() })
	provider, err := goose.NewProvider(goose.DialectMySQL, db, FS)
	if err != nil {
		t.Fatal(err)
	}
	return db, provider
}
