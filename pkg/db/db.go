package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

func Connect() *sql.DB {
	loc, err := time.LoadLocation(getEnv("DB_TIMEZONE", "Asia/Riyadh"))
	if err != nil {
		log.Fatalf("invalid DB_TIMEZONE: %v", err)
	}

	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DBUSER")
	cfg.Passwd = os.Getenv("PASSWORD")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("HOST")
	cfg.DBName = os.Getenv("DBNAME")
	cfg.AllowNativePasswords = true
	cfg.ParseTime = true
	cfg.Loc = loc
	cfg.Collation = "utf8mb4_unicode_ci"
	cfg.InterpolateParams = true   // single round-trip queries
	cfg.Timeout = 10 * time.Second // dial timeout
	cfg.ReadTimeout = 30 * time.Second
	cfg.WriteTimeout = 30 * time.Second
	cfg.Params = map[string]string{
		"time_zone": "'+03:00'", // server-side tz for NOW()
	}
	if os.Getenv("DB_TLS") == "true" {
		cfg.TLSConfig = "true"
	}

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatalf("sql.Open: %v", err)
	}

	// ── Pool tuning: keep these SHORTER than MySQL's wait_timeout ──
	db.SetMaxOpenConns(getEnvInt("DB_MAX_OPEN_CONNS", 50))
	db.SetMaxIdleConns(getEnvInt("DB_MAX_IDLE_CONNS", 10))
	db.SetConnMaxLifetime(time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_SEC", 180)) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(getEnvInt("DB_CONN_MAX_IDLE_SEC", 60)) * time.Second)

	// Bounded ping with retry
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}
	fmt.Println("DB connected:", cfg.Addr, cfg.DBName)
	return db
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
func CloseConnection(db *sql.DB) {
	defer db.Close()
}
