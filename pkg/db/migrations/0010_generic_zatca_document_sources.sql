-- Generic ZATCA source identity for bills, credit notes, and debit notes.
-- Existing bill rows are backfilled before the legacy bill-only columns are
-- removed from durable ZATCA records.

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'credit_note'
     AND column_name = 'request_id') = 0,
  'ALTER TABLE `credit_note`
     ADD COLUMN `request_id` CHAR(36) NOT NULL DEFAULT (uuid()) AFTER `bill_id`,
     ADD UNIQUE KEY `uq_credit_note_request_id` (`request_id`)',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND column_name = 'request_id') = 0,
  'ALTER TABLE `debit_note`
     ADD COLUMN `request_id` CHAR(36) NOT NULL DEFAULT (uuid()) AFTER `bill_id`,
     ADD UNIQUE KEY `uq_debit_note_request_id` (`request_id`)',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'document_type') = 0,
  'ALTER TABLE `outbox`
     ADD COLUMN `document_type` VARCHAR(32) NULL AFTER `aggregate_id`,
     ADD COLUMN `source_id` BIGINT UNSIGNED NULL AFTER `document_type`,
     ADD COLUMN `request_id` CHAR(36) NULL AFTER `pos_id`,
     ADD COLUMN `original_document_type` VARCHAR(32) NULL AFTER `invoice_uuid`,
     ADD COLUMN `original_source_id` BIGINT UNSIGNED NULL AFTER `original_document_type`,
     ADD COLUMN `original_invoice_uuid` CHAR(36) NULL AFTER `original_source_id`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'bill_request_id') > 0,
  'UPDATE `outbox`
   SET
     `document_type` = COALESCE(NULLIF(`document_type`, ''''), NULLIF(`aggregate_type`, ''''), ''bill''),
     `source_id` = COALESCE(`source_id`, `aggregate_id`),
     `request_id` = COALESCE(`request_id`, `bill_request_id`)
   WHERE `document_type` IS NULL OR `source_id` IS NULL OR `request_id` IS NULL',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.table_constraints
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND constraint_name = 'fk_outbox_invoice_identity') > 0,
  'ALTER TABLE `outbox` DROP FOREIGN KEY `fk_outbox_invoice_identity`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND index_name = 'uq_outbox_tenant_pos_request') > 0,
  'ALTER TABLE `outbox` DROP INDEX `uq_outbox_tenant_pos_request`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'bill_request_id') > 0,
  'ALTER TABLE `outbox` DROP COLUMN `bill_request_id`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND index_name = 'uq_outbox_tenant_document_source') = 0,
  'ALTER TABLE `outbox`
     MODIFY COLUMN `document_type` VARCHAR(32) NOT NULL,
     MODIFY COLUMN `source_id` BIGINT UNSIGNED NOT NULL,
     ADD UNIQUE KEY `uq_outbox_tenant_document_source` (`tenant_id`,`document_type`,`source_id`),
     ADD UNIQUE KEY `uq_outbox_tenant_pos_request` (`tenant_id`,`pos_id`,`request_id`),
     ADD KEY `idx_outbox_tenant_source` (`tenant_id`,`document_type`,`source_id`),
     ADD CONSTRAINT `chk_outbox_document_type`
       CHECK (`document_type` IN (''bill'',''credit'',''debit''))',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_egs_document'
     AND column_name = 'pos_id') = 0,
  'ALTER TABLE `zatca_egs_document`
     ADD COLUMN `pos_id` VARCHAR(64) NULL AFTER `tenant_id`,
     ADD COLUMN `request_id` CHAR(36) NULL AFTER `pos_id`,
     ADD COLUMN `original_document_type` VARCHAR(32) NULL AFTER `source_id`,
     ADD COLUMN `original_source_id` BIGINT UNSIGNED NULL AFTER `original_document_type`,
     ADD COLUMN `original_invoice_uuid` CHAR(36) NULL AFTER `original_source_id`,
     ADD KEY `idx_zatca_egs_document_source` (`tenant_id`,`document_type`,`source_id`)',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'document_type') = 0,
  'ALTER TABLE `zatca_submission`
     ADD COLUMN `document_type` VARCHAR(32) NULL AFTER `tenant_id`,
     ADD COLUMN `source_id` BIGINT UNSIGNED NULL AFTER `document_type`,
     ADD COLUMN `request_id` CHAR(36) NULL AFTER `pos_id`,
     ADD COLUMN `original_document_type` VARCHAR(32) NULL AFTER `invoice_uuid`,
     ADD COLUMN `original_source_id` BIGINT UNSIGNED NULL AFTER `original_document_type`,
     ADD COLUMN `original_invoice_uuid` CHAR(36) NULL AFTER `original_source_id`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'bill_id') > 0,
  'UPDATE `zatca_submission`
   SET
     `document_type` = COALESCE(NULLIF(`document_type`, ''''), ''bill''),
     `source_id` = COALESCE(`source_id`, `bill_id`),
     `request_id` = COALESCE(`request_id`, `bill_request_id`)
   WHERE `document_type` IS NULL OR `source_id` IS NULL OR `request_id` IS NULL',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.table_constraints
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND constraint_name = 'fk_zatca_bill_identity') > 0,
  'ALTER TABLE `zatca_submission` DROP FOREIGN KEY `fk_zatca_bill_identity`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.table_constraints
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND constraint_name = 'fk_zatca_invoice_identity') > 0,
  'ALTER TABLE `zatca_submission` DROP FOREIGN KEY `fk_zatca_invoice_identity`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.table_constraints
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND constraint_name = 'fk_zatca_bill_id') > 0,
  'ALTER TABLE `zatca_submission` DROP FOREIGN KEY `fk_zatca_bill_id`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'uq_zatca_tenant_bill') > 0,
  'ALTER TABLE `zatca_submission` DROP INDEX `uq_zatca_tenant_bill`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'uq_zatca_tenant_pos_request') > 0,
  'ALTER TABLE `zatca_submission` DROP INDEX `uq_zatca_tenant_pos_request`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'bill_id') > 0,
  'ALTER TABLE `zatca_submission` DROP COLUMN `bill_id`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'bill_request_id') > 0,
  'ALTER TABLE `zatca_submission` DROP COLUMN `bill_request_id`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'uq_zatca_tenant_document_source') = 0,
  'ALTER TABLE `zatca_submission`
     MODIFY COLUMN `document_type` VARCHAR(32) NOT NULL,
     MODIFY COLUMN `source_id` BIGINT UNSIGNED NOT NULL,
     MODIFY COLUMN `request_id` CHAR(36) NOT NULL,
     ADD UNIQUE KEY `uq_zatca_tenant_document_source` (`tenant_id`,`document_type`,`source_id`),
     ADD UNIQUE KEY `uq_zatca_tenant_pos_request` (`tenant_id`,`pos_id`,`request_id`),
     ADD UNIQUE KEY `uq_zatca_tenant_invoice_uuid` (`tenant_id`,`invoice_uuid`),
     ADD KEY `idx_zatca_source` (`tenant_id`,`document_type`,`source_id`),
     ADD CONSTRAINT `chk_zatca_document_type`
       CHECK (`document_type` IN (''bill'',''credit'',''debit''))',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

CREATE OR REPLACE VIEW `zatca_document_source` AS
SELECT
  b.tenant_id AS tenant_id,
  'bill' AS document_type,
  b.id AS source_id,
  b.pos_id AS pos_id,
  b.bill_request_id AS request_id,
  b.invoice_uuid AS invoice_uuid,
  b.branch_id AS branch_id,
  b.sequence_number AS sequence_number,
  CAST(NULL AS CHAR(32)) AS original_document_type,
  CAST(NULL AS UNSIGNED) AS original_source_id,
  CAST(NULL AS CHAR(36)) AS original_invoice_uuid
FROM bill b
WHERE b.tenant_id IS NOT NULL
UNION ALL
SELECT
  b.tenant_id AS tenant_id,
  'credit' AS document_type,
  cn.id AS source_id,
  b.pos_id AS pos_id,
  cn.request_id AS request_id,
  cn.invoice_uuid AS invoice_uuid,
  b.branch_id AS branch_id,
  b.sequence_number AS sequence_number,
  'bill' AS original_document_type,
  b.id AS original_source_id,
  b.invoice_uuid AS original_invoice_uuid
FROM credit_note cn
JOIN bill b ON b.id = cn.bill_id
WHERE b.tenant_id IS NOT NULL
UNION ALL
SELECT
  b.tenant_id AS tenant_id,
  'debit' AS document_type,
  dn.id AS source_id,
  b.pos_id AS pos_id,
  dn.request_id AS request_id,
  dn.invoice_uuid AS invoice_uuid,
  b.branch_id AS branch_id,
  b.sequence_number AS sequence_number,
  'bill' AS original_document_type,
  b.id AS original_source_id,
  b.invoice_uuid AS original_invoice_uuid
FROM debit_note dn
JOIN bill b ON b.id = dn.bill_id
WHERE b.tenant_id IS NOT NULL;
