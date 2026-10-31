// Command migrate runs the goose migrations in pkg/db/goose against one tenant database.
//
// The connection uses the backend's environment names: HOST (host:port), DBUSER, PASSWORD and
// DBNAME. The database must already exist.
//
//	migrate up         apply every pending migration
//	migrate down       roll back one migration; refuses to go below the baseline
//	migrate roundtrip  roll back every migration above the baseline, then apply them again
//	migrate status     list every migration and whether it is applied
//	migrate version    print the current database version
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	goosemigrations "ifritah/web-service-gin/pkg/db/goose"
)

func main() {
	if len(os.Args) != 2 {
		usage()
	}
	db, err := openDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectMySQL, db, goosemigrations.FS)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	switch os.Args[1] {
	case "up":
		err = up(ctx, provider)
	case "down":
		err = down(ctx, provider)
	case "roundtrip":
		err = roundtrip(ctx, provider)
	case "status":
		err = status(ctx, provider)
	case "version":
		err = version(ctx, provider)
	default:
		usage()
	}
	if err != nil {
		log.Fatal(err)
	}
}

func up(ctx context.Context, provider *goose.Provider) error {
	results, err := provider.Up(ctx)
	if err != nil {
		return err
	}
	for _, result := range results {
		fmt.Printf("applied %s in %s\n", result.Source.Path, result.Duration)
	}
	fmt.Printf("%d migration(s) applied\n", len(results))
	return nil
}

func down(ctx context.Context, provider *goose.Provider) error {
	current, err := provider.GetDBVersion(ctx)
	if err != nil {
		return err
	}
	if current <= goosemigrations.BaselineVersion {
		return fmt.Errorf("refusing to roll back: the database is at baseline version %d", current)
	}
	result, err := provider.Down(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("rolled back %s\n", result.Source.Path)
	return nil
}

func roundtrip(ctx context.Context, provider *goose.Provider) error {
	for {
		current, err := provider.GetDBVersion(ctx)
		if err != nil {
			return err
		}
		if current <= goosemigrations.BaselineVersion {
			break
		}
		result, err := provider.Down(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("rolled back %s\n", result.Source.Path)
	}
	return up(ctx, provider)
}

func status(ctx context.Context, provider *goose.Provider) error {
	statuses, err := provider.Status(ctx)
	if err != nil {
		return err
	}
	for _, s := range statuses {
		state := "pending"
		if s.State == goose.StateApplied {
			state = "applied"
		}
		fmt.Printf("%-40s %s\n", s.Source.Path, state)
	}
	return nil
}

func version(ctx context.Context, provider *goose.Provider) error {
	current, err := provider.GetDBVersion(ctx)
	if err != nil {
		return err
	}
	fmt.Println(current)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: migrate up|down|roundtrip|status|version")
	os.Exit(2)
}

func openDB() (*sql.DB, error) {
	host, name := os.Getenv("HOST"), os.Getenv("DBNAME")
	if host == "" || name == "" {
		return nil, errors.New("HOST and DBNAME must be set")
	}
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DBUSER")
	cfg.Passwd = os.Getenv("PASSWORD")
	cfg.Net = "tcp"
	cfg.Addr = host
	cfg.DBName = name
	cfg.ParseTime = true
	cfg.Timeout = 10 * time.Second

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to %s: %w", host, err)
	}
	return db, nil
}
