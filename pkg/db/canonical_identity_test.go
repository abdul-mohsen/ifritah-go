package db

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readDatabaseSource(t *testing.T, path ...string) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	parts := append([]string{filepath.Dir(filename)}, path...)
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatalf("read database source %v: %v", path, err)
	}
	return string(data)
}

func requireSourceContains(t *testing.T, source, want string) {
	t.Helper()
	if !strings.Contains(source, want) {
		t.Fatalf("source is missing %q", want)
	}
}

func TestCanonicalIdentityOwnershipSchema(t *testing.T) {
	migration := readDatabaseSource(t, "migrations", "0007_canonical_bill_identity.sql")

	if strings.Contains(migration, "PREPARE") || strings.Contains(migration, "information_schema") {
		t.Fatal("canonical identity migration must remain static")
	}

	for _, contract := range []string{
		"CREATE TABLE IF NOT EXISTS `tenant`",
		"ALTER TABLE `bill`",
		"CREATE TABLE IF NOT EXISTS `outbox`",
		"ALTER TABLE `zatca_submission`",
	} {
		requireSourceContains(t, migration, contract)
	}
}

func TestCanonicalBillRequestIdentitySchema(t *testing.T) {
	migration := readDatabaseSource(t, "migrations", "0007_canonical_bill_identity.sql")

	for _, column := range []string{
		"ADD COLUMN `tenant_id` bigint unsigned DEFAULT NULL",
		"ADD COLUMN `pos_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL",
		"ADD COLUMN `bill_request_id` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL",
		"ADD COLUMN `request_fingerprint` char(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL",
		"ADD COLUMN `request_fingerprint_version` smallint unsigned NOT NULL DEFAULT '1'",
	} {
		requireSourceContains(t, migration, column)
	}

	requireSourceContains(
		t,
		migration,
		"UNIQUE KEY `uq_bill_tenant_pos_request` (`tenant_id`,`pos_id`,`bill_request_id`)",
	)
	requireSourceContains(
		t,
		migration,
		"UNIQUE KEY `uq_bill_tenant_invoice_uuid` (`tenant_id`,`invoice_uuid`)",
	)
}

func TestExistingCreateBillCanOmitC004IdentityFields(t *testing.T) {
	migration := readDatabaseSource(t, "migrations", "0007_canonical_bill_identity.sql")

	for _, column := range []string{
		"ADD COLUMN `tenant_id` bigint unsigned DEFAULT NULL",
		"ADD COLUMN `pos_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL",
		"ADD COLUMN `bill_request_id` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL",
		"ADD COLUMN `request_fingerprint` char(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL",
	} {
		requireSourceContains(t, migration, column)
	}
}

func TestCanonicalIdentityIsMigrationOwned(t *testing.T) {
	schema := readDatabaseSource(t, "schema", "schema.sql")

	for _, directChange := range []string{
		"CREATE TABLE `tenant`",
		"CREATE TABLE `outbox`",
		"`tenant_id` bigint unsigned DEFAULT NULL",
		"Stable invoice UUID; distinct from bill_request_id",
	} {
		if strings.Contains(schema, directChange) {
			t.Fatalf("canonical identity change %q must be migration-owned", directChange)
		}
	}
}

func TestCanonicalBillRequestResolutionContract(t *testing.T) {
	queries := readDatabaseSource(t, "queries", "bill_identity.sql")

	requireSourceContains(t, queries, "-- name: ResolveBillRequest :one")
	requireSourceContains(t, queries, "-- name: GetBillByInvoiceUUID :one")
	requireSourceContains(t, queries, "b.invoice_uuid = sqlc.arg('invoice_uuid')")
	for _, predicate := range []string{
		"b.tenant_id = sqlc.arg('tenant_id')",
		"b.pos_id = sqlc.arg('pos_id')",
		"b.bill_request_id = sqlc.arg('bill_request_id')",
		"b.request_fingerprint = sqlc.arg('request_fingerprint')",
	} {
		requireSourceContains(t, queries, predicate)
	}
	requireSourceContains(t, queries, "'replay'")
	requireSourceContains(t, queries, "'conflict'")
	requireSourceContains(t, queries, "AS request_resolution")
}

func TestCanonicalTenantSeparationContract(t *testing.T) {
	queries := readDatabaseSource(t, "queries", "bill_identity.sql")

	if strings.Contains(queries, "company_id = sqlc.arg('tenant_id')") ||
		strings.Contains(queries, "merchant_id = sqlc.arg('tenant_id')") {
		t.Fatal("canonical tenant identity must not alias company_id or merchant_id")
	}
	requireSourceContains(t, queries, "WHERE b.tenant_id = sqlc.arg('tenant_id')")
	requireSourceContains(t, queries, "WHERE o.tenant_id = sqlc.arg('tenant_id')")
	requireSourceContains(t, queries, "WHERE zs.tenant_id = sqlc.arg('tenant_id')")
}
