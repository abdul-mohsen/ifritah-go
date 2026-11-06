package goosemigrations

import (
	"database/sql"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
)

// openTestDB opens database name on the server in dsn. An empty name opens the server without choosing a database.
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

// openFreshGooseDB recreates database name, opens it, and returns it with a goose provider for FS. The database is dropped
// when the test ends.
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
