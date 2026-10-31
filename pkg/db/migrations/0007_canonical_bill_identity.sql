-- 0007_canonical_bill_identity.sql
-- Canonical tenant/POS/request identity contract for bills, outbox events, and
-- ZATCA submissions. Existing bill and submission rows keep nullable identity
-- columns until C004 populates and enforces them for new sales.
--
-- This migration is ordered before the future outbox/EGS migrations. The
-- migration runner applies each numbered migration once.

CREATE TABLE IF NOT EXISTS `tenant` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_key` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_tenant_key` (`tenant_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `bill`
  ADD COLUMN `tenant_id` bigint unsigned DEFAULT NULL COMMENT 'Required canonical tenant identity for the C004 sale command',
  ADD COLUMN `pos_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Required POS identity for the C004 sale command',
  ADD COLUMN `bill_request_id` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Required caller request identity for the C004 sale command',
  ADD COLUMN `request_fingerprint` char(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT 'Required canonical request fingerprint for the C004 sale command',
  ADD COLUMN `request_fingerprint_version` smallint unsigned NOT NULL DEFAULT '1',
  ADD KEY `idx_bill_tenant_pos` (`tenant_id`,`pos_id`),
  ADD UNIQUE KEY `uq_bill_tenant_pos_request` (`tenant_id`,`pos_id`,`bill_request_id`),
  ADD UNIQUE KEY `uq_bill_tenant_invoice_uuid` (`tenant_id`,`invoice_uuid`),
  ADD UNIQUE KEY `uq_bill_tenant_pos_id` (`tenant_id`,`pos_id`,`id`),
  ADD CONSTRAINT `fk_bill_tenant` FOREIGN KEY (`tenant_id`) REFERENCES `tenant` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  ADD CONSTRAINT `chk_bill_request_fingerprint` CHECK (regexp_like(`request_fingerprint`, _utf8mb4'^[0-9a-f]{64}$')),
  ADD CONSTRAINT `chk_bill_request_fingerprint_version` CHECK ((`request_fingerprint_version` > 0));

CREATE TABLE IF NOT EXISTS `outbox` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `event_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT (uuid()),
  `aggregate_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `aggregate_id` bigint unsigned NOT NULL,
  `pos_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `bill_request_id` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `invoice_uuid` char(36) COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `payload` json NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_outbox_tenant_event` (`tenant_id`,`event_id`),
  KEY `idx_outbox_tenant_pending` (`tenant_id`,`created_at`,`id`),
  CONSTRAINT `fk_outbox_tenant` FOREIGN KEY (`tenant_id`) REFERENCES `tenant` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk_outbox_invoice_identity` FOREIGN KEY (`tenant_id`,`invoice_uuid`) REFERENCES `bill` (`tenant_id`,`invoice_uuid`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `zatca_submission`
  ADD KEY `idx_zatca_bill` (`bill_id`);

ALTER TABLE `zatca_submission`
  DROP INDEX `uq_zatca_bill`;

ALTER TABLE `zatca_submission`
  ADD COLUMN `tenant_id` bigint unsigned DEFAULT NULL AFTER `bill_id`,
  ADD COLUMN `pos_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL AFTER `tenant_id`,
  ADD COLUMN `bill_request_id` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL AFTER `pos_id`,
  ADD COLUMN `invoice_uuid` char(36) COLLATE utf8mb4_0900_ai_ci DEFAULT NULL AFTER `bill_request_id`,
  ADD UNIQUE KEY `uq_zatca_tenant_bill` (`tenant_id`,`bill_id`),
  ADD UNIQUE KEY `uq_zatca_tenant_pos_request` (`tenant_id`,`pos_id`,`bill_request_id`),
  ADD CONSTRAINT `fk_zatca_tenant` FOREIGN KEY (`tenant_id`) REFERENCES `tenant` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  ADD CONSTRAINT `fk_zatca_bill_identity` FOREIGN KEY (`tenant_id`,`pos_id`,`bill_request_id`) REFERENCES `bill` (`tenant_id`,`pos_id`,`bill_request_id`) ON DELETE CASCADE,
  ADD CONSTRAINT `fk_zatca_invoice_identity` FOREIGN KEY (`tenant_id`,`invoice_uuid`) REFERENCES `bill` (`tenant_id`,`invoice_uuid`) ON DELETE CASCADE,
  ADD CONSTRAINT `fk_zatca_bill_id` FOREIGN KEY (`tenant_id`,`pos_id`,`bill_id`) REFERENCES `bill` (`tenant_id`,`pos_id`,`id`) ON DELETE CASCADE;
