// Package testdb gives the tests of one package a MySQL database with every goose migration
// applied. A package calls Main from TestMain, so the migrations run once for the whole package.
// Every test shares that database and its connection pool, and Open empties the tables a test
// writes when the test ends. Tests of one package must not run in parallel, because they share
// the database.
package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	goosemigrations "ifritah/web-service-gin/pkg/db/goose"
)

// shared is the database of the running test binary. It is nil when TEST_MYSQL_DSN is not set.
var shared *sql.DB

// Main creates database name on the server in TEST_MYSQL_DSN, migrates it to the latest goose
// version, runs the package's tests, and drops the database. It returns the exit code for
// os.Exit. The server must let the test create and drop databases. Without TEST_MYSQL_DSN, the
// tests that call Open are skipped.
func Main(m *testing.M, name string) int {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		return m.Run()
	}
	db, teardown, err := setUp(dsn, name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdb:", err)
		return 1
	}
	defer teardown()
	shared = db
	return m.Run()
}

// Open returns the shared database for one test. tables are the tables that the test writes,
// children before parents. Open empties them when the test ends, so the next test starts with
// empty tables. Open skips the test when Main has no database.
func Open(t *testing.T, tables ...string) *sql.DB {
	t.Helper()
	if shared == nil {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	t.Cleanup(func() {
		for _, table := range tables {
			if _, err := shared.Exec("DELETE FROM `" + table + "`"); err != nil {
				t.Errorf("empty %s after the test: %v", table, err)
			}
		}
	})
	return shared
}

// setUp recreates database name and migrates it. The returned teardown closes the pool and drops
// the database.
func setUp(dsn, name string) (*sql.DB, func(), error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, nil, err
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, nil, err
	}
	drop := "DROP DATABASE IF EXISTS `" + name + "`"
	dropDatabase := func() {
		_, _ = admin.Exec(drop)
		admin.Close()
	}
	if _, err := admin.Exec(drop); err != nil {
		dropDatabase()
		return nil, nil, err
	}
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		dropDatabase()
		return nil, nil, err
	}
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		dropDatabase()
		return nil, nil, err
	}
	provider, err := goose.NewProvider(goose.DialectMySQL, db, goosemigrations.FS)
	if err == nil {
		_, err = provider.Up(context.Background())
	}
	if err != nil {
		db.Close()
		dropDatabase()
		return nil, nil, fmt.Errorf("migrate %s: %w", name, err)
	}
	return db, func() {
		db.Close()
		dropDatabase()
	}, nil
}
