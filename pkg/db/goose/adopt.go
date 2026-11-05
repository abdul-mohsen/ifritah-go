package goosemigrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
)

// Adopt records the baseline as applied in a database that was built before goose managed it. The database must match the
// baseline digests exactly. If it does not, Adopt changes nothing and lists the differences, so drift is fixed by hand before
// the database is adopted. Run up afterwards to apply the migrations after the baseline.
func Adopt(ctx context.Context, db *sql.DB, provider *goose.Provider) error {
	rows, err := versionRows(ctx, db)
	if err != nil {
		return fmt.Errorf("adopt: %w", err)
	}
	if rows > 0 {
		return errors.New("adopt: goose already records migrations in this database; run up instead")
	}

	want, err := BaselineDigests()
	if err != nil {
		return err
	}
	got, err := SchemaDigests(ctx, db)
	if err != nil {
		return fmt.Errorf("adopt: %w", err)
	}
	if len(got) == 0 {
		return errors.New("adopt: the database has no tables; build an empty database with up instead")
	}
	if diffs := CompareDigests(got, want); len(diffs) > 0 {
		return fmt.Errorf("adopt: the database does not match 00001_baseline.sql, so nothing was recorded:\n  %s",
			strings.Join(diffs, "\n  "))
	}

	// GetVersions creates goose's version table, in goose's own layout, when it is missing.
	if _, _, err := provider.GetVersions(ctx); err != nil {
		return fmt.Errorf("adopt: create %s: %w", versionTable, err)
	}
	// Version 0 is goose's starting point, and version 1 is the baseline. Both are recorded in one statement.
	if _, err := db.ExecContext(ctx, "INSERT INTO goose_db_version (version_id, is_applied) VALUES (0, 1), (1, 1)"); err != nil {
		return fmt.Errorf("adopt: record the baseline: %w", err)
	}
	return nil
}

// versionRows returns how many rows goose's version table holds, or 0 when the table does not exist yet.
func versionRows(ctx context.Context, db *sql.DB) (int, error) {
	var tables int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`,
		versionTable).Scan(&tables); err != nil {
		return 0, err
	}
	if tables == 0 {
		return 0, nil
	}
	var rows int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM goose_db_version").Scan(&rows); err != nil {
		return 0, err
	}
	return rows, nil
}
