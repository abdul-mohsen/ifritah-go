-- Canonical CSID lifecycle metadata is owned by a tenant, branch, and EGS
-- sequence. Secrets and provider calls remain outside this contract.

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name = 'egs_sequence'
     AND index_name = 'uq_egs_sequence_tenant_branch_id') = 0,
  'ALTER TABLE `egs_sequence` ADD UNIQUE KEY `uq_egs_sequence_tenant_branch_id` (`tenant_id`,`branch_id`,`egs_sequence_id`)',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

CREATE TABLE IF NOT EXISTS `zatca_egs_credential` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `branch_id` int unsigned NOT NULL,
  `egs_sequence_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `credential_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'production',
  `certificate_serial` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `certificate_expires_at` datetime DEFAULT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `renewal_state` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'none',
  `revocation_state` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'none',
  `renewal_requested_at` datetime DEFAULT NULL,
  `renewed_at` datetime DEFAULT NULL,
  `revocation_requested_at` datetime DEFAULT NULL,
  `revoked_at` datetime DEFAULT NULL,
  `revocation_reason` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `last_error` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_zatca_egs_credential_owner` (`tenant_id`,`branch_id`,`egs_sequence_id`,`credential_type`),
  UNIQUE KEY `uq_zatca_egs_credential_serial` (`tenant_id`,`certificate_serial`),
  KEY `idx_zatca_egs_credential_lifecycle` (`tenant_id`,`branch_id`,`status`,`certificate_expires_at`),
  KEY `idx_zatca_egs_credential_renewal` (`tenant_id`,`renewal_state`,`renewal_requested_at`),
  KEY `idx_zatca_egs_credential_revocation` (`tenant_id`,`revocation_state`,`revocation_requested_at`),
  CONSTRAINT `fk_zatca_egs_credential_tenant`
    FOREIGN KEY (`tenant_id`) REFERENCES `tenant` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk_zatca_egs_credential_branch`
    FOREIGN KEY (`branch_id`) REFERENCES `branches` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk_zatca_egs_credential_egs`
    FOREIGN KEY (`tenant_id`,`branch_id`,`egs_sequence_id`)
    REFERENCES `egs_sequence` (`tenant_id`,`branch_id`,`egs_sequence_id`)
    ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `chk_zatca_egs_credential_type`
    CHECK (`credential_type` IN ('compliance','production')),
  CONSTRAINT `chk_zatca_egs_credential_status`
    CHECK (`status` IN ('pending','active','expired','revoked')),
  CONSTRAINT `chk_zatca_egs_credential_renewal`
    CHECK (`renewal_state` IN ('none','pending','completed','failed')),
  CONSTRAINT `chk_zatca_egs_credential_revocation`
    CHECK (`revocation_state` IN ('none','pending','revoked')),
  CONSTRAINT `chk_zatca_egs_credential_material`
    CHECK (
      `status` = 'pending'
      OR (
        `certificate_serial` IS NOT NULL
        AND char_length(trim(`certificate_serial`)) > 0
        AND `certificate_expires_at` IS NOT NULL
      )
    ),
  CONSTRAINT `chk_zatca_egs_credential_revoked`
    CHECK (
      (`status` <> 'revoked' OR `revocation_state` = 'revoked')
      AND (`revocation_state` <> 'revoked' OR `status` = 'revoked')
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
