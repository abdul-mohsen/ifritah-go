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

func tableSource(t *testing.T, schema, table string) string {
	t.Helper()

	start := strings.Index(schema, "CREATE TABLE `"+table+"`")
	if start < 0 {
		t.Fatalf("schema does not define table %q", table)
	}

	end := strings.Index(schema[start:], "\n--")
	if end < 0 {
		t.Fatalf("schema definition for table %q has no terminator", table)
	}
	return schema[start : start+end]
}

func requireSourceContains(t *testing.T, source, want string) {
	t.Helper()
	if !strings.Contains(source, want) {
		t.Fatalf("source is missing %q", want)
	}
}

func TestCanonicalIdentityOwnershipSchema(t *testing.T) {
	schema := readDatabaseSource(t, "schema", "schema.sql")

	for _, table := range []string{"bill", "outbox", "zatca_submission"} {
		requireSourceContains(t, tableSource(t, schema, table), "`tenant_id`")
	}
}

func TestCanonicalBillRequestIdentitySchema(t *testing.T) {
	schema := readDatabaseSource(t, "schema", "schema.sql")
	bill := tableSource(t, schema, "bill")

	for _, column := range []string{
		"`tenant_id`",
		"`pos_id`",
		"`bill_request_id`",
		"`request_fingerprint`",
		"`request_fingerprint_version`",
		"`invoice_uuid`",
	} {
		requireSourceContains(t, bill, column)
	}

	requireSourceContains(
		t,
		bill,
		"UNIQUE KEY `uq_bill_tenant_pos_request` (`tenant_id`,`pos_id`,`bill_request_id`)",
	)
	requireSourceContains(
		t,
		bill,
		"UNIQUE KEY `uq_bill_tenant_invoice_uuid` (`tenant_id`,`invoice_uuid`)",
	)
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
