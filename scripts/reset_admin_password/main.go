// reset_admin_password sets the password for the local-dev admin user
// (`admin` / id 20) to a known bcrypt hash so the frontend peer-dev
// agent can smoke-test the typed-filter feature against the local
// docker MySQL (search-and-filters thread, blocker in msg #24).
//
// LOCAL-ONLY. Refuses to run against any DB whose name contains "prod".
//
// Usage:
//   go run ./scripts/reset_admin_password
//
// Result: user 'admin' password becomes 'admin123'.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pkgdb "ifritah/web-service-gin/pkg/db"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

const (
	adminUsername = "admin"
	adminPassword = "admin123"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: could not load .env: %v", err)
	}
	dbName := strings.ToLower(os.Getenv("DBNAME"))
	if strings.Contains(dbName, "prod") {
		log.Fatalf("refusing to touch production-looking DB %q", dbName)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), 10)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}
	// Go bcrypt emits $2a$ which is what the existing user rows use.

	dbConn := pkgdb.Connect()
	defer dbConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := dbConn.ExecContext(ctx,
		`UPDATE user SET password = ?, is_active = 1, is_deleted = 0
		  WHERE username = ?`,
		string(hash), adminUsername)
	if err != nil {
		log.Fatalf("UPDATE user: %v", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		log.Fatalf("no row matched username=%q — seed an admin user first", adminUsername)
	}
	fmt.Printf("ok: %s/%s now works against %s\n", adminUsername, adminPassword, dbName)
}
