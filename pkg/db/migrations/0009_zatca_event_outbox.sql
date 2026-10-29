-- 0009_zatca_event_outbox.sql
--
-- Durable queue records for ZATCA document and onboarding events. Business
-- mutations insert an outbox row in the same transaction; a post-commit
-- publisher owns the status/retry transitions.

CREATE TABLE IF NOT EXISTS `zatca_event_outbox` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `event_id` varchar(128) NOT NULL,
  `tenant_id` varchar(128) NOT NULL,
  `merchant_id` int NOT NULL,
  `document_type` varchar(16) NOT NULL,
  `document_id` bigint unsigned NOT NULL,
  `branch_id` int unsigned NOT NULL,
  `idempotency_key` varchar(255) NOT NULL,
  `payload` json NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'pending',
  `attempt_count` int unsigned NOT NULL DEFAULT '0',
  `available_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `claimed_at` datetime DEFAULT NULL,
  `published_at` datetime DEFAULT NULL,
  `last_error` varchar(1024) DEFAULT NULL,
  `last_error_at` datetime DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_zatca_outbox_event` (`event_id`),
  UNIQUE KEY `uq_zatca_outbox_idempotency` (`idempotency_key`),
  KEY `idx_zatca_outbox_due` (`status`,`available_at`,`id`),
  KEY `idx_zatca_outbox_tenant_document` (`tenant_id`,`merchant_id`,`document_type`,`document_id`),
  KEY `idx_zatca_outbox_branch` (`branch_id`,`status`),
  CONSTRAINT `chk_zatca_outbox_document_type` CHECK (`document_type` IN ('bill','credit','debit','onboard')),
  CONSTRAINT `chk_zatca_outbox_status` CHECK (`status` IN ('pending','processing','published','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
