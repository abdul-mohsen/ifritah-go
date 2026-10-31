package db

import (
	"strings"
	"testing"
)

func TestTransactionalOutboxMigrationIsStaticAndComplete(t *testing.T) {
	migration := readDatabaseSource(t, "migrations", "0008_transactional_outbox.sql")

	for _, forbidden := range []string{
		"information_schema",
		"PREPARE",
		"EXECUTE",
		"DEALLOCATE",
		"SET @",
	} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("transactional outbox migration must not use dynamic SQL %q", forbidden)
		}
	}

	for _, contract := range []string{
		"CREATE TABLE IF NOT EXISTS `outbox`",
		"ADD COLUMN `egs_sequence_id`",
		"ADD COLUMN `payload_fingerprint`",
		"ADD COLUMN `content_hash`",
		"ADD COLUMN `status`",
		"ADD COLUMN `attempt_count`",
		"ADD COLUMN `next_attempt_at`",
		"ADD COLUMN `lease_owner`",
		"ADD COLUMN `lease_expires_at`",
		"ADD COLUMN `last_error`",
		"ADD COLUMN `published_at`",
		"ADD COLUMN `dead_lettered_at`",
		"ADD COLUMN `manual_review_at`",
		"ADD COLUMN `updated_at`",
		"ADD UNIQUE KEY `uq_outbox_tenant_pos_request`",
		"ADD KEY `idx_outbox_claim`",
		"DROP INDEX `idx_outbox_tenant_pending`",
		"ADD KEY `idx_outbox_tenant_pending`",
		"CONSTRAINT `fk_outbox_tenant`",
		"CONSTRAINT `fk_outbox_invoice_identity`",
	} {
		requireSourceContains(t, migration, contract)
	}
}

func TestTransactionalOutboxMigrationIsSeededAfterCanonicalIdentity(t *testing.T) {
	compose := readDatabaseSource(t, "..", "..", "docker-compose.yml")
	canonical := strings.Index(
		compose,
		"./pkg/db/migrations/0007_canonical_bill_identity.sql:/docker-entrypoint-initdb.d/15-0007-canonical-bill-identity.sql:ro",
	)
	outbox := strings.Index(
		compose,
		"./pkg/db/migrations/0008_transactional_outbox.sql:/docker-entrypoint-initdb.d/16-0008-transactional-outbox.sql:ro",
	)
	if canonical < 0 || outbox < 0 || outbox <= canonical {
		t.Fatal("docker-compose.yml must seed migration 0008 after migration 0007")
	}
}
