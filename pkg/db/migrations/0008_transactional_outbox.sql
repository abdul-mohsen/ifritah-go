-- Durable transactional outbox lifecycle for canonical atomic-sale events.
-- The dynamic statements keep sqlc's schema parser from applying the
-- migration twice when it validates the checked-in schema dump.

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'egs_sequence_id') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `egs_sequence_id` VARCHAR(64) NULL AFTER `invoice_uuid`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'payload_fingerprint') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `payload_fingerprint` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER `payload`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'content_hash') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `content_hash` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER `payload_fingerprint`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'status') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `status` VARCHAR(24) NOT NULL DEFAULT ''pending'' AFTER `content_hash`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'attempt_count') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `attempt_count` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `status`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'next_attempt_at') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `next_attempt_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `attempt_count`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'lease_owner') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `lease_owner` VARCHAR(128) NULL AFTER `next_attempt_at`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'lease_expires_at') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `lease_expires_at` DATETIME NULL AFTER `lease_owner`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'last_error') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `last_error` TEXT NULL AFTER `lease_expires_at`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'published_at') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `published_at` DATETIME NULL AFTER `last_error`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'dead_lettered_at') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `dead_lettered_at` DATETIME NULL AFTER `published_at`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'manual_review_at') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `manual_review_at` DATETIME NULL AFTER `dead_lettered_at`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'updated_at') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND index_name = 'idx_outbox_claim') = 0,
  'ALTER TABLE `outbox` ADD KEY `idx_outbox_claim` (`status`,`next_attempt_at`,`lease_expires_at`,`id`)',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND index_name = 'uq_outbox_tenant_pos_request') = 0,
  'ALTER TABLE `outbox` ADD UNIQUE KEY `uq_outbox_tenant_pos_request` (`tenant_id`,`pos_id`,`bill_request_id`)',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND index_name = 'idx_outbox_tenant_pending') > 0,
  'ALTER TABLE `outbox` DROP INDEX `idx_outbox_tenant_pending`, ADD KEY `idx_outbox_tenant_pending` (`tenant_id`,`status`,`next_attempt_at`,`id`)',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
