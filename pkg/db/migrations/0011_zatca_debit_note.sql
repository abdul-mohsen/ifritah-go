-- 0011_zatca_debit_note.sql
--
-- Durable, tenant-scoped debit-note source rows. The row is created by the
-- backend transaction; daemon output columns remain nullable until a later
-- processing contract updates them.

CREATE TABLE IF NOT EXISTS `debit_note` (
  `id`                 bigint unsigned NOT NULL AUTO_INCREMENT,
  `bill_id`            bigint unsigned NOT NULL,
  `merchant_id`        int NOT NULL,
  `branch_id`          int unsigned NOT NULL,
  `sequence_number`    bigint unsigned DEFAULT NULL,
  `egs_id`             varchar(128) NOT NULL,
  `document_id`        varchar(128) DEFAULT NULL,
  `stable_document_key` varchar(255) DEFAULT NULL,
  `state`              int NOT NULL DEFAULT '1' COMMENT '1=pending,3=submitted',
  `note`               text NOT NULL COMMENT 'KSA-10 reason',
  `created_at`         datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `invoice_uuid`       char(36) DEFAULT NULL,
  `invoice_hash`       varchar(128) DEFAULT NULL,
  `invoice_qr`         mediumtext,
  `invoice_xml_path`   varchar(500) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_debit_note_merchant_bill` (`merchant_id`,`bill_id`),
  UNIQUE KEY `uq_debit_note_document` (`merchant_id`,`document_id`),
  UNIQUE KEY `uq_debit_note_stable_key` (`merchant_id`,`stable_document_key`),
  KEY `idx_debit_note_state` (`merchant_id`,`state`),
  KEY `idx_debit_note_context` (`merchant_id`,`branch_id`,`egs_id`,`sequence_number`),
  CONSTRAINT `fk_debit_note_bill` FOREIGN KEY (`bill_id`)
    REFERENCES `bill` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_debit_note_branch` FOREIGN KEY (`branch_id`)
    REFERENCES `branches` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Older backend dumps may already contain the daemon-shaped debit_note table
-- without tenant/context columns. Add the contract columns in place so this
-- migration is safe for both fresh and existing tenant databases.
SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND column_name = 'merchant_id') = 0,
  'ALTER TABLE `debit_note` ADD COLUMN `merchant_id` int NULL AFTER `bill_id`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND column_name = 'branch_id') = 0,
  'ALTER TABLE `debit_note` ADD COLUMN `branch_id` int unsigned NULL AFTER `merchant_id`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND column_name = 'sequence_number') = 0,
  'ALTER TABLE `debit_note` ADD COLUMN `sequence_number` bigint unsigned DEFAULT NULL AFTER `branch_id`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND column_name = 'egs_id') = 0,
  'ALTER TABLE `debit_note` ADD COLUMN `egs_id` varchar(128) DEFAULT NULL AFTER `sequence_number`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND column_name = 'document_id') = 0,
  'ALTER TABLE `debit_note` ADD COLUMN `document_id` varchar(128) DEFAULT NULL AFTER `egs_id`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND column_name = 'stable_document_key') = 0,
  'ALTER TABLE `debit_note` ADD COLUMN `stable_document_key` varchar(255) DEFAULT NULL AFTER `document_id`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

UPDATE `debit_note` dn
JOIN `bill` b ON b.`id` = dn.`bill_id`
LEFT JOIN `branch_zatca_config` bzc ON bzc.`branch_id` = b.`branch_id`
SET
  dn.`merchant_id` = COALESCE(dn.`merchant_id`, b.`merchant_id`),
  dn.`branch_id` = COALESCE(dn.`branch_id`, b.`branch_id`),
  dn.`sequence_number` = COALESCE(dn.`sequence_number`, b.`sequence_number`),
  dn.`egs_id` = COALESCE(NULLIF(dn.`egs_id`, ''), NULLIF(bzc.`csr_computer_number`, '')),
  dn.`document_id` = COALESCE(NULLIF(dn.`document_id`, ''), CONCAT('debit:', dn.`id`))
WHERE dn.`merchant_id` IS NULL
   OR dn.`branch_id` IS NULL
   OR dn.`sequence_number` IS NULL
   OR dn.`egs_id` IS NULL
   OR dn.`egs_id` = ''
   OR dn.`document_id` IS NULL
   OR dn.`document_id` = '';

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND index_name = 'uq_debit_note_merchant_bill'),
  'SELECT 1',
  'ALTER TABLE `debit_note` ADD UNIQUE KEY `uq_debit_note_merchant_bill` (`merchant_id`,`bill_id`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND index_name = 'uq_debit_note_document'),
  'SELECT 1',
  'ALTER TABLE `debit_note` ADD UNIQUE KEY `uq_debit_note_document` (`merchant_id`,`document_id`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND index_name = 'uq_debit_note_stable_key'),
  'SELECT 1',
  'ALTER TABLE `debit_note` ADD UNIQUE KEY `uq_debit_note_stable_key` (`merchant_id`,`stable_document_key`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND index_name = 'idx_debit_note_state'),
  'SELECT 1',
  'ALTER TABLE `debit_note` ADD KEY `idx_debit_note_state` (`merchant_id`,`state`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'debit_note'
     AND index_name = 'idx_debit_note_context'),
  'SELECT 1',
  'ALTER TABLE `debit_note` ADD KEY `idx_debit_note_context` (`merchant_id`,`branch_id`,`egs_id`,`sequence_number`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
