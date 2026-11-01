// Package testdb gives tests a fresh MySQL database with every goose migration applied.
package testdb

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	goosemigrations "ifritah/web-service-gin/pkg/db/goose"
)

// Open recreates database name on the server in TEST_MYSQL_DSN, migrates it to the latest goose
// version, and returns it. The database is dropped when the test ends. TEST_MYSQL_DSN must point
// at a server that the test may create and drop databases on. Open skips the test when the
// variable is not set.
func Open(t *testing.T, name string) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + name + "`"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS `" + name + "`") })

	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	provider, err := goose.NewProvider(goose.DialectMySQL, db, goosemigrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	return db
}
