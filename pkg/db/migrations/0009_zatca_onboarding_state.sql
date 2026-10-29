-- 0009_zatca_onboarding_state.sql
--
-- Durable branch onboarding state and attempt history. The branch row remains
-- the current-state/API boundary; the child table preserves every attempt.
-- The conditional DDL keeps this migration safe to replay on existing
-- tenant databases that already contain the older bookkeeping columns.

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'branch_zatca_config'
     AND column_name = 'onboard_step') = 0,
  'ALTER TABLE `branch_zatca_config` ADD COLUMN `onboard_step` varchar(32) NOT NULL DEFAULT ''none'' AFTER `onboard_state`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'branch_zatca_config'
     AND column_name = 'onboard_attempt') = 0,
  'ALTER TABLE `branch_zatca_config` ADD COLUMN `onboard_attempt` bigint unsigned NOT NULL DEFAULT 0 AFTER `onboard_step`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'branch_zatca_config'
     AND column_name = 'onboard_retry_count') = 0,
  'ALTER TABLE `branch_zatca_config` ADD COLUMN `onboard_retry_count` int unsigned NOT NULL DEFAULT 0 AFTER `onboard_attempt`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'branch_zatca_config'
     AND column_name = 'onboard_error_code') = 0,
  'ALTER TABLE `branch_zatca_config` ADD COLUMN `onboard_error_code` varchar(64) DEFAULT NULL AFTER `onboard_retry_count`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'branch_zatca_config'
     AND column_name = 'onboard_error') = 0,
  'ALTER TABLE `branch_zatca_config` ADD COLUMN `onboard_error` text DEFAULT NULL AFTER `onboard_error_code`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'branch_zatca_config'
     AND column_name = 'onboard_queued_at') = 0,
  'ALTER TABLE `branch_zatca_config` ADD COLUMN `onboard_queued_at` datetime DEFAULT NULL AFTER `onboard_error`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'branch_zatca_config'
     AND column_name = 'onboard_started_at') = 0,
  'ALTER TABLE `branch_zatca_config` ADD COLUMN `onboard_started_at` datetime DEFAULT NULL AFTER `onboard_queued_at`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'branch_zatca_config'
     AND column_name = 'onboard_completed_at') = 0,
  'ALTER TABLE `branch_zatca_config` ADD COLUMN `onboard_completed_at` datetime DEFAULT NULL AFTER `onboard_started_at`',
  'SELECT 1'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

UPDATE `branch_zatca_config`
SET
  `onboard_step` = CASE `onboard_state`
    WHEN 'queued' THEN 'queued'
    WHEN 'csr' THEN 'csr'
    WHEN 'compliance' THEN 'compliance'
    WHEN 'invoices' THEN 'invoices'
    WHEN 'production' THEN 'production'
    WHEN 'done' THEN 'done'
    WHEN 'failed' THEN 'failed'
    ELSE 'none'
  END,
  `onboard_error` = COALESCE(`onboard_error`, `last_error`),
  `onboard_started_at` = CASE
    WHEN `onboard_state` IN ('csr','compliance','invoices','production','done','failed')
      THEN COALESCE(`onboard_started_at`, `last_attempt_at`)
    ELSE `onboard_started_at`
  END,
  `onboard_completed_at` = COALESCE(`onboard_completed_at`, `zatca_onboarded_at`)
WHERE `onboard_step` = 'none'
   OR (`onboard_error` IS NULL AND `last_error` IS NOT NULL)
   OR (`onboard_completed_at` IS NULL AND `zatca_onboarded_at` IS NOT NULL);

SET @s := IF(
  EXISTS(SELECT 1 FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'branch_zatca_config'
     AND index_name = 'idx_bzc_onboard_state'),
  'SELECT 1',
  'ALTER TABLE `branch_zatca_config` ADD KEY `idx_bzc_onboard_state` (`onboard_state`,`onboard_attempt`)'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

CREATE TABLE IF NOT EXISTS `zatca_onboarding_attempt` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` int unsigned NOT NULL,
  `attempt_number` bigint unsigned NOT NULL,
  `state` varchar(32) NOT NULL DEFAULT 'queued',
  `step` varchar(32) NOT NULL DEFAULT 'queued',
  `retry_count` int unsigned NOT NULL DEFAULT '0',
  `error_code` varchar(64) DEFAULT NULL,
  `error_message` text DEFAULT NULL,
  `queued_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `started_at` datetime DEFAULT NULL,
  `completed_at` datetime DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_zatca_onboarding_attempt` (`branch_id`,`attempt_number`),
  KEY `idx_zatca_onboarding_state` (`branch_id`,`state`,`attempt_number`),
  KEY `idx_zatca_onboarding_retry` (`state`,`retry_count`),
  CONSTRAINT `chk_zatca_onboarding_state` CHECK (`state` IN ('queued','compliance','production','done','failed','not_started','csr','invoices')),
  CONSTRAINT `chk_zatca_onboarding_step` CHECK (`step` IN ('none','queued','csr','compliance','invoices','production','done','failed')),
  CONSTRAINT `fk_zatca_onboarding_branch` FOREIGN KEY (`branch_id`) REFERENCES `branches` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
