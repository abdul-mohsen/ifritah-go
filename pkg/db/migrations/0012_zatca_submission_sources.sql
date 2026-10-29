-- 0012_zatca_submission_sources.sql
--
-- Extend T003 submissions from bill-only identity to generic document/source
-- identity while retaining bill_id as the parent compatibility reference.

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'document_type') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `document_type` varchar(16) NULL DEFAULT ''bill'' AFTER `bill_id`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'source_id') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `source_id` bigint unsigned NULL AFTER `document_type`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'stable_document_key') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `stable_document_key` varchar(255) DEFAULT NULL AFTER `document_id`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'sequence_number') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `sequence_number` bigint unsigned DEFAULT NULL AFTER `stable_document_key`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'egs_id') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `egs_id` varchar(128) DEFAULT NULL AFTER `sequence_number`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

UPDATE `zatca_submission`
SET `document_type` = 'bill'
WHERE `document_type` IS NULL OR `document_type` = '';

UPDATE `zatca_submission`
SET `source_id` = `bill_id`
WHERE `source_id` IS NULL OR `source_id` = 0;

UPDATE `zatca_submission` zs
JOIN `bill` b ON b.`id` = zs.`bill_id`
LEFT JOIN `branch_zatca_config` bzc ON bzc.`branch_id` = b.`branch_id`
SET
  zs.`sequence_number` = COALESCE(zs.`sequence_number`, b.`sequence_number`),
  zs.`egs_id` = COALESCE(NULLIF(zs.`egs_id`, ''), NULLIF(bzc.`csr_computer_number`, ''))
WHERE zs.`sequence_number` IS NULL
   OR zs.`egs_id` IS NULL
   OR zs.`egs_id` = '';

ALTER TABLE `zatca_submission`
  MODIFY COLUMN `document_type` varchar(16) NOT NULL DEFAULT 'bill',
  MODIFY COLUMN `source_id` bigint unsigned NOT NULL;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'uq_zatca_bill'),
  'ALTER TABLE `zatca_submission` DROP INDEX `uq_zatca_bill`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'uq_zatca_source'),
  'SELECT 1',
  'ALTER TABLE `zatca_submission` ADD UNIQUE KEY `uq_zatca_source` (`merchant_id`,`document_type`,`source_id`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'idx_zatca_source'),
  'SELECT 1',
  'ALTER TABLE `zatca_submission` ADD KEY `idx_zatca_source` (`merchant_id`,`document_type`,`source_id`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.table_constraints
   WHERE constraint_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND constraint_name = 'chk_zatca_submission_document_type'),
  'SELECT 1',
  'ALTER TABLE `zatca_submission` ADD CONSTRAINT `chk_zatca_submission_document_type` CHECK (`document_type` IN (''bill'',''credit'',''debit''))'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
