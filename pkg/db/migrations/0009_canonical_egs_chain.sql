-- Canonical tenant/branch/VAT/EGS ownership and transactional ICV/PIH chain.
-- Reservations are provisional. The committed ICV advances only when the
-- immutable document and its outbox event are inserted in the same transaction.

CREATE TABLE IF NOT EXISTS `egs_sequence` (
  `egs_sequence_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `branch_id` int unsigned NOT NULL,
  `vat_number` varchar(15) COLLATE utf8mb4_unicode_ci NOT NULL,
  `egs_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `last_icv` bigint unsigned NOT NULL DEFAULT 0,
  `previous_invoice_hash` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reservation_key` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reservation_token` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reserved_icv` bigint unsigned DEFAULT NULL,
  `reserved_previous_invoice_hash` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reserved_document_uuid` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reserved_outbox_event_id` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reserved_at` datetime DEFAULT NULL,
  `lease_expires_at` datetime DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`egs_sequence_id`),
  UNIQUE KEY `uq_egs_sequence_owner` (`tenant_id`,`branch_id`,`vat_number`,`egs_id`),
  KEY `idx_egs_sequence_tenant_branch` (`tenant_id`,`branch_id`),
  KEY `idx_egs_sequence_reservation` (`egs_sequence_id`,`reservation_key`),
  CONSTRAINT `fk_egs_sequence_tenant` FOREIGN KEY (`tenant_id`) REFERENCES `tenant` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk_egs_sequence_branch` FOREIGN KEY (`branch_id`) REFERENCES `branches` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `chk_egs_sequence_identity` CHECK (
    char_length(trim(`egs_sequence_id`)) > 0
    AND char_length(trim(`vat_number`)) > 0
    AND char_length(trim(`egs_id`)) > 0
  ),
  CONSTRAINT `chk_egs_sequence_reservation` CHECK (
    (`reservation_key` IS NULL AND `reservation_token` IS NULL AND `reserved_icv` IS NULL
      AND `reserved_previous_invoice_hash` IS NULL AND `reserved_document_uuid` IS NULL
      AND `reserved_outbox_event_id` IS NULL)
    OR
    (`reservation_key` IS NOT NULL AND `reservation_token` IS NOT NULL AND `reserved_icv` IS NOT NULL
      AND `reserved_document_uuid` IS NOT NULL AND `reserved_outbox_event_id` IS NOT NULL)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `zatca_egs_document` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `egs_sequence_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `branch_id` int unsigned NOT NULL,
  `vat_number` varchar(15) COLLATE utf8mb4_unicode_ci NOT NULL,
  `egs_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `document_uuid` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `document_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_id` bigint unsigned NOT NULL,
  `outbox_event_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `icv` bigint unsigned NOT NULL,
  `pih` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `document_snapshot` json NOT NULL,
  `state` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'queued',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_zatca_egs_document_uuid` (`tenant_id`,`document_uuid`),
  UNIQUE KEY `uq_zatca_egs_document_icv` (`egs_sequence_id`,`icv`),
  UNIQUE KEY `uq_zatca_egs_document_outbox` (`tenant_id`,`outbox_event_id`),
  KEY `idx_zatca_egs_document_state` (`tenant_id`,`egs_sequence_id`,`state`,`id`),
  CONSTRAINT `fk_zatca_egs_document_tenant` FOREIGN KEY (`tenant_id`) REFERENCES `tenant` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk_zatca_egs_document_sequence` FOREIGN KEY (`egs_sequence_id`) REFERENCES `egs_sequence` (`egs_sequence_id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk_zatca_egs_document_branch` FOREIGN KEY (`branch_id`) REFERENCES `branches` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `chk_zatca_egs_document_state` CHECK (`state` IN ('queued','submitted','accepted','warning','rejected','ambiguous'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'icv') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `icv` BIGINT UNSIGNED NULL AFTER `egs_sequence_id`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND column_name = 'pih') = 0,
  'ALTER TABLE `outbox` ADD COLUMN `pih` VARCHAR(255) NULL AFTER `icv`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND index_name = 'uq_outbox_tenant_document_uuid') = 0,
  'ALTER TABLE `outbox` ADD UNIQUE KEY `uq_outbox_tenant_document_uuid` (`tenant_id`,`invoice_uuid`)',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'outbox'
     AND index_name = 'uq_outbox_egs_icv') = 0,
  'ALTER TABLE `outbox` ADD UNIQUE KEY `uq_outbox_egs_icv` (`tenant_id`,`egs_sequence_id`,`icv`)',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
