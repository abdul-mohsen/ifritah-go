package goosemigrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// baselineDigests holds one "table digest" line per table, for the database that 00001_baseline.sql builds. The goose
// tests keep it current, and Adopt compares a database built before goose with it.
//
//go:embed baseline.digests
var baselineDigests string

// versionTable is the table in which goose records the migrations it has applied.
const versionTable = "goose_db_version"

// SchemaDigests returns one digest per base table of the current database, except goose's version table. A digest covers the
// columns (name, type, nullability), the indexes, the foreign keys with their columns and rules, and the names of the check
// constraints. It leaves out defaults, collations, and column order, which MySQL reports differently from one version to another.
// It reads information_schema only.
func SchemaDigests(ctx context.Context, db *sql.DB) (map[string]string, error) {
	lines := map[string][]string{}
	if err := addTables(ctx, db, lines); err != nil {
		return nil, err
	}
	for _, add := range []func(context.Context, *sql.DB, map[string][]string) error{addColumns, addIndexes, addForeignKeys, addChecks} {
		if err := add(ctx, db, lines); err != nil {
			return nil, err
		}
	}
	digests := make(map[string]string, len(lines))
	for table, parts := range lines {
		sort.Strings(parts)
		sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
		digests[table] = hex.EncodeToString(sum[:])
	}
	return digests, nil
}

// BaselineDigests returns the digests that a database built by 00001_baseline.sql has.
func BaselineDigests() (map[string]string, error) {
	digests := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(baselineDigests), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("baseline.digests: malformed line %q", line)
		}
		digests[fields[0]] = fields[1]
	}
	return digests, nil
}

// FormatDigests renders digests as baseline.digests does: one "table digest" line per table, sorted by table name.
func FormatDigests(digests map[string]string) string {
	var b strings.Builder
	for _, table := range sortedNames(digests) {
		fmt.Fprintf(&b, "%s %s\n", table, digests[table])
	}
	return b.String()
}

// CompareDigests lists how a database differs from the baseline: the tables it is missing, the tables it has extra, and the tables
// whose definition differs.
func CompareDigests(got, want map[string]string) []string {
	var diffs []string
	for _, table := range sortedNames(want) {
		have, ok := got[table]
		switch {
		case !ok:
			diffs = append(diffs, "missing table "+table)
		case have != want[table]:
			diffs = append(diffs, "table "+table+" differs from 00001_baseline.sql")
		}
	}
	for _, table := range sortedNames(got) {
		if _, ok := want[table]; !ok {
			diffs = append(diffs, "extra table "+table)
		}
	}
	return diffs
}

func sortedNames(digests map[string]string) []string {
	names := make([]string, 0, len(digests))
	for name := range digests {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// queryRows runs query with args, and calls each for every row.
func queryRows(ctx context.Context, db *sql.DB, query string, args []any, each func(*sql.Rows) error) error {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("schema digest: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := each(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func addTables(ctx context.Context, db *sql.DB, lines map[string][]string) error {
	query := `SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE' AND TABLE_NAME <> ?`
	return queryRows(ctx, db, query, []any{versionTable}, func(r *sql.Rows) error {
		var table string
		if err := r.Scan(&table); err != nil {
			return err
		}
		lines[table] = nil
		return nil
	})
}

func addColumns(ctx context.Context, db *sql.DB, lines map[string][]string) error {
	query := `SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE()`
	return queryRows(ctx, db, query, nil, func(r *sql.Rows) error {
		var table, name, typ, nullable string
		if err := r.Scan(&table, &name, &typ, &nullable); err != nil {
			return err
		}
		if _, ok := lines[table]; ok {
			lines[table] = append(lines[table], fmt.Sprintf("column %s %s nullable=%s", name, typ, nullable))
		}
		return nil
	})
}

// indexLine collects the columns of one index, in index order.
type indexLine struct {
	table, name, kind string
	unique            bool
	columns           []string
}

func addIndexes(ctx context.Context, db *sql.DB, lines map[string][]string) error {
	indexes := map[string]*indexLine{}
	query := `SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, INDEX_TYPE, COLUMN_NAME FROM information_schema.STATISTICS ` +
		`WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`
	err := queryRows(ctx, db, query, nil, func(r *sql.Rows) error {
		var table, name, kind, column string
		var nonUnique int
		if err := r.Scan(&table, &name, &nonUnique, &kind, &column); err != nil {
			return err
		}
		if _, ok := lines[table]; !ok {
			return nil
		}
		key := table + "\x00" + name
		ix := indexes[key]
		if ix == nil {
			ix = &indexLine{table: table, name: name, kind: kind, unique: nonUnique == 0}
			indexes[key] = ix
		}
		ix.columns = append(ix.columns, column)
		return nil
	})
	if err != nil {
		return err
	}
	for _, ix := range indexes {
		lines[ix.table] = append(lines[ix.table], fmt.Sprintf("index %s unique=%t type=%s columns=%s",
			ix.name, ix.unique, ix.kind, strings.Join(ix.columns, ",")))
	}
	return nil
}

// foreignKeyLine collects the columns of one foreign key, in key order.
type foreignKeyLine struct {
	table, name, refTable, onUpdate, onDelete string
	columns, refColumns                       []string
}

func addForeignKeys(ctx context.Context, db *sql.DB, lines map[string][]string) error {
	keys := map[string]*foreignKeyLine{}
	query := `SELECT k.TABLE_NAME, k.CONSTRAINT_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME, ` +
		`r.UPDATE_RULE, r.DELETE_RULE FROM information_schema.KEY_COLUMN_USAGE k ` +
		`JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON r.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA ` +
		`AND r.CONSTRAINT_NAME = k.CONSTRAINT_NAME AND r.TABLE_NAME = k.TABLE_NAME ` +
		`WHERE k.TABLE_SCHEMA = DATABASE() AND k.REFERENCED_TABLE_NAME IS NOT NULL ` +
		`ORDER BY k.TABLE_NAME, k.CONSTRAINT_NAME, k.ORDINAL_POSITION`
	err := queryRows(ctx, db, query, nil, func(r *sql.Rows) error {
		var table, name, column, refTable, refColumn, onUpdate, onDelete string
		if err := r.Scan(&table, &name, &column, &refTable, &refColumn, &onUpdate, &onDelete); err != nil {
			return err
		}
		if _, ok := lines[table]; !ok {
			return nil
		}
		key := table + "\x00" + name
		fk := keys[key]
		if fk == nil {
			fk = &foreignKeyLine{table: table, name: name, refTable: refTable, onUpdate: onUpdate, onDelete: onDelete}
			keys[key] = fk
		}
		fk.columns = append(fk.columns, column)
		fk.refColumns = append(fk.refColumns, refColumn)
		return nil
	})
	if err != nil {
		return err
	}
	for _, fk := range keys {
		lines[fk.table] = append(lines[fk.table], fmt.Sprintf("foreign key %s (%s) references %s (%s) update=%s delete=%s",
			fk.name, strings.Join(fk.columns, ","), fk.refTable, strings.Join(fk.refColumns, ","), fk.onUpdate, fk.onDelete))
	}
	return nil
}

func addChecks(ctx context.Context, db *sql.DB, lines map[string][]string) error {
	query := `SELECT TABLE_NAME, CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS ` +
		`WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_TYPE = 'CHECK'`
	return queryRows(ctx, db, query, nil, func(r *sql.Rows) error {
		var table, name string
		if err := r.Scan(&table, &name); err != nil {
			return err
		}
		if _, ok := lines[table]; ok {
			lines[table] = append(lines[table], "check "+name)
		}
		return nil
	})
}
