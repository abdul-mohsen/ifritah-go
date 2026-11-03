package handlers

import (
	"os"
	"testing"

	"ifritah/web-service-gin/pkg/testdb"
)

// TestMain gives the package one migrated MySQL database when TEST_MYSQL_DSN is set. Without it, the database tests skip.
func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m, "handlers_test"))
}
