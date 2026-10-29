-- 0010_zatca_egs_chain.sql
--
-- Tenant-local, branch/EGS-scoped ownership for the shared ZATCA ICV and
-- previous-invoice-hash chain. The tenant boundary is the database selected
-- by the caller; branch_id and egs_id are the row-level ownership boundary.

CREATE TABLE IF NOT EXISTS `zatca_egs_chain` (
  `branch_id`              INT UNSIGNED NOT NULL,
  `egs_id`                 VARCHAR(128) NOT NULL,
  `last_icv`               BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `previous_invoice_hash`  VARCHAR(255) DEFAULT NULL,
  `reservation_key`        VARCHAR(255) DEFAULT NULL,
  `reservation_token`      VARCHAR(128) DEFAULT NULL,
  `reserved_icv`           BIGINT UNSIGNED DEFAULT NULL,
  `reserved_previous_hash` VARCHAR(255) DEFAULT NULL,
  `reserved_at`            DATETIME DEFAULT NULL,
  `lease_expires_at`       DATETIME DEFAULT NULL,
  `updated_at`             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`branch_id`, `egs_id`),
  KEY `idx_zatca_egs_chain_reservation`
    (`branch_id`, `reservation_key`),
  CONSTRAINT `fk_zatca_egs_chain_branch` FOREIGN KEY (`branch_id`)
    REFERENCES `branches` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
