-- 0008_transactional_outbox.sql
-- Complete the canonical transactional outbox contract created by migration
-- 0007. The static DDL is intentionally ordered after 0007 so sqlc can build
-- its schema model without executing dynamic SQL.

CREATE TABLE IF NOT EXISTS `outbox` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `event_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT (uuid()),
  `aggregate_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `aggregate_id` bigint unsigned NOT NULL,
  `pos_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `bill_request_id` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `invoice_uuid` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `payload` json NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_outbox_tenant_event` (`tenant_id`,`event_id`),
  KEY `idx_outbox_tenant_pending` (`tenant_id`,`created_at`,`id`),
  CONSTRAINT `fk_outbox_tenant` FOREIGN KEY (`tenant_id`) REFERENCES `tenant` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk_outbox_invoice_identity` FOREIGN KEY (`tenant_id`,`invoice_uuid`) REFERENCES `bill` (`tenant_id`,`invoice_uuid`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `outbox`
  ADD COLUMN `egs_sequence_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL AFTER `invoice_uuid`,
  ADD COLUMN `payload_fingerprint` char(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL AFTER `payload`,
  ADD COLUMN `content_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL AFTER `payload_fingerprint`,
  ADD COLUMN `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' AFTER `content_hash`,
  ADD COLUMN `attempt_count` int unsigned NOT NULL DEFAULT '0' AFTER `status`,
  ADD COLUMN `next_attempt_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `attempt_count`,
  ADD COLUMN `lease_owner` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL AFTER `next_attempt_at`,
  ADD COLUMN `lease_expires_at` datetime DEFAULT NULL AFTER `lease_owner`,
  ADD COLUMN `last_error` text COLLATE utf8mb4_unicode_ci DEFAULT NULL AFTER `lease_expires_at`,
  ADD COLUMN `published_at` datetime DEFAULT NULL AFTER `last_error`,
  ADD COLUMN `dead_lettered_at` datetime DEFAULT NULL AFTER `published_at`,
  ADD COLUMN `manual_review_at` datetime DEFAULT NULL AFTER `dead_lettered_at`,
  ADD COLUMN `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`,
  ADD UNIQUE KEY `uq_outbox_tenant_pos_request` (`tenant_id`,`pos_id`,`bill_request_id`),
  ADD KEY `idx_outbox_claim` (`status`,`next_attempt_at`,`lease_expires_at`,`id`),
  DROP INDEX `idx_outbox_tenant_pending`,
  ADD KEY `idx_outbox_tenant_pending` (`tenant_id`,`status`,`next_attempt_at`,`id`);
