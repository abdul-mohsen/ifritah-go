-- 0008_zatca_submission_state.sql
--
-- Durable ZATCA submission state, provider outcomes, retry attempts, and
-- artifact references. The existing numeric status/response columns remain
-- for compatibility; the explicit outcome_state is the monitor source of
-- truth after this migration.

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'merchant_id') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `merchant_id` int NULL AFTER `bill_id`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'document_id') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `document_id` varchar(128) NULL AFTER `bill_id`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'outcome_state') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `outcome_state` varchar(32) NULL DEFAULT NULL AFTER `document_id`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'provider_code') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `provider_code` varchar(64) DEFAULT NULL AFTER `outcome_state`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'provider_warning_code') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `provider_warning_code` varchar(64) DEFAULT NULL AFTER `provider_code`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'provider_warning_message') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `provider_warning_message` varchar(1024) DEFAULT NULL AFTER `provider_warning_code`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'provider_reference') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `provider_reference` varchar(255) DEFAULT NULL AFTER `provider_warning_message`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'last_attempt_at') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `last_attempt_at` datetime DEFAULT NULL AFTER `retry_count`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'next_retry_at') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `next_retry_at` datetime DEFAULT NULL AFTER `last_attempt_at`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'outcome_at') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `outcome_at` datetime DEFAULT NULL AFTER `next_retry_at`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'request_xml_path') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `request_xml_path` varchar(500) DEFAULT NULL AFTER `outcome_at`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND column_name = 'cleared_xml_path') = 0,
  'ALTER TABLE `zatca_submission` ADD COLUMN `cleared_xml_path` varchar(500) DEFAULT NULL AFTER `request_xml_path`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

UPDATE `zatca_submission` zs
JOIN `bill` b ON b.`id` = zs.`bill_id`
SET
  zs.`merchant_id` = b.`merchant_id`,
  zs.`document_id` = COALESCE(NULLIF(b.`invoice_uuid`, ''), CONCAT('bill:', b.`id`))
WHERE zs.`merchant_id` IS NULL
   OR zs.`document_id` IS NULL
   OR zs.`document_id` = '';

UPDATE `zatca_submission`
SET `outcome_state` = CASE `status`
  WHEN 2 THEN 'accepted'
  WHEN 3 THEN 'rejected'
  WHEN 4 THEN 'accepted_with_warning'
  ELSE 'pending'
END
WHERE `outcome_state` IS NULL OR `outcome_state` = '';

ALTER TABLE `zatca_submission`
  MODIFY COLUMN `merchant_id` int NOT NULL,
  MODIFY COLUMN `document_id` varchar(128) NOT NULL,
  MODIFY COLUMN `outcome_state` varchar(32) NOT NULL DEFAULT 'pending';

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.table_constraints
   WHERE constraint_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND constraint_name = 'chk_zatca_submission_outcome_state'),
  'SELECT 1',
  'ALTER TABLE `zatca_submission` ADD CONSTRAINT `chk_zatca_submission_outcome_state` CHECK (`outcome_state` IN (''pending'',''accepted'',''accepted_with_warning'',''rejected'',''ambiguous'',''terminal_failure''))'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'uq_zatca_merchant_document'),
  'SELECT 1',
  'ALTER TABLE `zatca_submission` ADD UNIQUE KEY `uq_zatca_merchant_document` (`merchant_id`,`document_id`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'uq_zatca_bill_document'),
  'SELECT 1',
  'ALTER TABLE `zatca_submission` ADD UNIQUE KEY `uq_zatca_bill_document` (`bill_id`,`document_id`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'idx_zatca_merchant_state'),
  'SELECT 1',
  'ALTER TABLE `zatca_submission` ADD KEY `idx_zatca_merchant_state` (`merchant_id`,`outcome_state`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'idx_zatca_outcome_state'),
  'SELECT 1',
  'ALTER TABLE `zatca_submission` ADD KEY `idx_zatca_outcome_state` (`outcome_state`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'idx_zatca_provider_reference'),
  'SELECT 1',
  'ALTER TABLE `zatca_submission` ADD KEY `idx_zatca_provider_reference` (`provider_reference`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'zatca_submission'
     AND index_name = 'idx_zatca_retry'),
  'SELECT 1',
  'ALTER TABLE `zatca_submission` ADD KEY `idx_zatca_retry` (`next_retry_at`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

CREATE TABLE IF NOT EXISTS `zatca_submission_attempt` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `submission_id` int NOT NULL,
  `attempt_number` int unsigned NOT NULL,
  `attempt_state` varchar(32) NOT NULL DEFAULT 'pending',
  `provider_code` varchar(64) DEFAULT NULL,
  `provider_warning_code` varchar(64) DEFAULT NULL,
  `provider_warning_message` varchar(1024) DEFAULT NULL,
  `provider_reference` varchar(255) DEFAULT NULL,
  `error_code` varchar(64) DEFAULT NULL,
  `error_message` varchar(1024) DEFAULT NULL,
  `retry_count` int unsigned NOT NULL DEFAULT '0',
  `next_retry_at` datetime DEFAULT NULL,
  `request_xml_path` varchar(500) DEFAULT NULL,
  `cleared_xml_path` varchar(500) DEFAULT NULL,
  `started_at` datetime NOT NULL,
  `completed_at` datetime DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_zatca_attempt_number` (`submission_id`,`attempt_number`),
  KEY `idx_zatca_attempt_retry` (`next_retry_at`),
  KEY `idx_zatca_attempt_state` (`attempt_state`),
  CONSTRAINT `chk_zatca_attempt_state` CHECK ((`attempt_state` in ('pending','accepted','accepted_with_warning','rejected','ambiguous','terminal_failure'))),
  CONSTRAINT `fk_zatca_attempt_submission` FOREIGN KEY (`submission_id`) REFERENCES `zatca_submission` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `zatca_submission_outcome` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `submission_id` int NOT NULL,
  `attempt_id` bigint unsigned DEFAULT NULL,
  `outcome_state` varchar(32) NOT NULL,
  `provider_code` varchar(64) DEFAULT NULL,
  `provider_warning_code` varchar(64) DEFAULT NULL,
  `provider_warning_message` varchar(1024) DEFAULT NULL,
  `provider_reference` varchar(255) DEFAULT NULL,
  `observed_at` datetime NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_zatca_outcome_latest` (`submission_id`,`observed_at`,`id`),
  KEY `idx_zatca_outcome_provider_code` (`provider_code`),
  CONSTRAINT `chk_zatca_outcome_state` CHECK ((`outcome_state` in ('pending','accepted','accepted_with_warning','rejected','ambiguous','terminal_failure'))),
  CONSTRAINT `fk_zatca_outcome_submission` FOREIGN KEY (`submission_id`) REFERENCES `zatca_submission` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_zatca_outcome_attempt` FOREIGN KEY (`attempt_id`) REFERENCES `zatca_submission_attempt` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `zatca_submission_artifact` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `submission_id` int NOT NULL,
  `attempt_id` bigint unsigned DEFAULT NULL,
  `artifact_kind` varchar(32) NOT NULL,
  `artifact_path` varchar(500) DEFAULT NULL,
  `content_type` varchar(128) DEFAULT NULL,
  `sha256` char(64) DEFAULT NULL,
  `metadata` json DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_zatca_artifact_submission` (`submission_id`,`created_at`),
  KEY `idx_zatca_artifact_attempt` (`attempt_id`),
  CONSTRAINT `chk_zatca_artifact_kind` CHECK ((`artifact_kind` in ('request_xml','cleared_xml','response_xml','other'))),
  CONSTRAINT `fk_zatca_artifact_submission` FOREIGN KEY (`submission_id`) REFERENCES `zatca_submission` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_zatca_artifact_attempt` FOREIGN KEY (`attempt_id`) REFERENCES `zatca_submission_attempt` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
