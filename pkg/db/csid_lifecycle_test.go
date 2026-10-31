package db

import "testing"

func TestCSIDLifecycleSourceContract(t *testing.T) {
	schema := readDatabaseSource(t, "schema", "schema.sql")
	table := tableSource(t, schema, "zatca_egs_credential")
	migration := readDatabaseSource(t, "migrations", "0010_canonical_csid_lifecycle.sql")
	queries := readDatabaseSource(t, "queries", "csid_lifecycle.sql")

	for _, column := range []string{
		"`tenant_id`",
		"`branch_id`",
		"`egs_sequence_id`",
		"`credential_type`",
		"`certificate_serial`",
		"`certificate_expires_at`",
		"`status`",
		"`renewal_state`",
		"`revocation_state`",
		"`renewal_requested_at`",
		"`revocation_requested_at`",
		"`revoked_at`",
	} {
		requireSourceContains(t, table, column)
	}
	for _, state := range []string{"'pending'", "'active'", "'expired'", "'revoked'"} {
		requireSourceContains(t, table, state)
	}
	for _, contract := range []string{
		"UNIQUE KEY `uq_zatca_egs_credential_owner`",
		"FOREIGN KEY (`tenant_id`,`branch_id`,`egs_sequence_id`)",
		"REFERENCES `egs_sequence` (`tenant_id`,`branch_id`,`egs_sequence_id`)",
		"(`status` <> 'revoked' OR `revocation_state` = 'revoked')",
		"(`revocation_state` <> 'revoked' OR `status` = 'revoked')",
	} {
		requireSourceContains(t, table, contract)
	}

	for _, query := range []string{
		"-- name: GetZatcaEGSCredential :one",
		"-- name: ListZatcaEGSCredentialsByBranch :many",
		"-- name: ListZatcaEGSCredentialsForLifecycle :many",
		"-- name: CreateZatcaEGSCredential :execresult",
		"-- name: RequestZatcaEGSCredentialRenewal :execresult",
		"-- name: RecordZatcaEGSCredentialIssued :execresult",
		"-- name: MarkZatcaEGSCredentialRenewalFailed :execresult",
		"-- name: MarkZatcaEGSCredentialExpired :execresult",
		"-- name: RequestZatcaEGSCredentialRevocation :execresult",
		"-- name: MarkZatcaEGSCredentialRevoked :execresult",
	} {
		requireSourceContains(t, queries, query)
	}
	for _, predicate := range []string{
		"c.tenant_id = sqlc.arg('tenant_id')",
		"c.branch_id = sqlc.arg('branch_id')",
		"c.egs_sequence_id = sqlc.arg('egs_sequence_id')",
		"c.credential_type = sqlc.arg('credential_type')",
	} {
		requireSourceContains(t, queries, predicate)
	}
	requireSourceContains(t, migration, "CREATE TABLE IF NOT EXISTS `zatca_egs_credential`")
	requireSourceContains(t, migration, "uq_egs_sequence_tenant_branch_id")
}
