-- T031: durable signed-UBL import evidence and review workflow.
--
-- Import state is deliberately independent from zatca_submission and the
-- transactional outbox. Posting an import records intake workflow completion;
-- it does not create an outbound provider submission.

CREATE TABLE IF NOT EXISTS `zatca_import_job` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `merchant_id` int NOT NULL,
  `branch_id` int unsigned DEFAULT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'review',
  `workflow_version` bigint unsigned NOT NULL DEFAULT '1',
  `source_content_hash` char(64) NOT NULL,
  `idempotency_key` varchar(128) DEFAULT NULL,
  `reviewed_by` int DEFAULT NULL,
  `reviewed_at` datetime DEFAULT NULL,
  `posted_by` int DEFAULT NULL,
  `posted_at` datetime DEFAULT NULL,
  `post_reference` varchar(255) DEFAULT NULL,
  `rejected_by` int DEFAULT NULL,
  `rejected_at` datetime DEFAULT NULL,
  `rejected_reason` varchar(1024) DEFAULT NULL,
  `created_by` int NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_zatca_import_content` (`merchant_id`,`source_content_hash`),
  UNIQUE KEY `uq_zatca_import_idempotency` (`merchant_id`,`idempotency_key`),
  KEY `idx_zatca_import_scope_status` (`merchant_id`,`status`,`created_at`,`id`),
  KEY `idx_zatca_import_branch` (`merchant_id`,`branch_id`,`status`),
  CONSTRAINT `chk_zatca_import_status`
    CHECK (`status` IN ('review','approved','posted','rejected')),
  CONSTRAINT `chk_zatca_import_hash`
    CHECK (CHAR_LENGTH(`source_content_hash`) = 64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `zatca_import_evidence` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `import_job_id` bigint unsigned NOT NULL,
  `merchant_id` int NOT NULL,
  `source_reference` varchar(255) NOT NULL,
  `source_original_name` varchar(255) DEFAULT NULL,
  `source_mime_type` varchar(100) NOT NULL DEFAULT 'application/xml',
  `source_provenance` varchar(128) NOT NULL DEFAULT 'supplier_xml',
  `source_content_hash` char(64) NOT NULL,
  `source_size` bigint unsigned NOT NULL,
  `original_bytes` mediumblob DEFAULT NULL,
  `canonical_data` json DEFAULT NULL,
  `parsed_data` json DEFAULT NULL,
  `signature_data` json DEFAULT NULL,
  `validation_data` json DEFAULT NULL,
  `result_data` json DEFAULT NULL,
  `failure_class` varchar(32) DEFAULT NULL,
  `failure_message` varchar(1024) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_zatca_import_evidence_job` (`import_job_id`),
  KEY `idx_zatca_import_evidence_scope` (`merchant_id`,`source_content_hash`),
  CONSTRAINT `fk_zatca_import_evidence_job`
    FOREIGN KEY (`import_job_id`) REFERENCES `zatca_import_job` (`id`)
    ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `chk_zatca_import_evidence_hash`
    CHECK (CHAR_LENGTH(`source_content_hash`) = 64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `zatca_import_validation_error` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `import_job_id` bigint unsigned NOT NULL,
  `merchant_id` int NOT NULL,
  `severity` varchar(16) NOT NULL,
  `error_code` varchar(64) NOT NULL,
  `field_key` varchar(128) DEFAULT NULL,
  `message` varchar(1024) NOT NULL,
  `details` json DEFAULT NULL,
  `created_by` int NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_zatca_import_validation_scope`
    (`merchant_id`,`import_job_id`,`severity`,`created_at`,`id`),
  CONSTRAINT `fk_zatca_import_validation_job`
    FOREIGN KEY (`import_job_id`) REFERENCES `zatca_import_job` (`id`)
    ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `chk_zatca_import_validation_severity`
    CHECK (`severity` IN ('warning','error'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `zatca_import_review` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `import_job_id` bigint unsigned NOT NULL,
  `merchant_id` int NOT NULL,
  `decision` varchar(32) NOT NULL,
  `reason` varchar(1024) DEFAULT NULL,
  `expected_workflow_version` bigint unsigned DEFAULT NULL,
  `observed_workflow_version` bigint unsigned NOT NULL,
  `created_by` int NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_zatca_import_review_scope`
    (`merchant_id`,`import_job_id`,`created_at`,`id`),
  CONSTRAINT `fk_zatca_import_review_job`
    FOREIGN KEY (`import_job_id`) REFERENCES `zatca_import_job` (`id`)
    ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `chk_zatca_import_review_decision`
    CHECK (`decision` IN ('approve','review','request_changes'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `zatca_import_status_history` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `import_job_id` bigint unsigned NOT NULL,
  `merchant_id` int NOT NULL,
  `from_status` varchar(16) DEFAULT NULL,
  `to_status` varchar(16) NOT NULL,
  `transition_key` varchar(191) NOT NULL,
  `workflow_version` bigint unsigned NOT NULL,
  `reason` varchar(1024) DEFAULT NULL,
  `created_by` int NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_zatca_import_transition` (`import_job_id`,`transition_key`),
  KEY `idx_zatca_import_history_scope`
    (`merchant_id`,`import_job_id`,`created_at`,`id`),
  CONSTRAINT `fk_zatca_import_history_job`
    FOREIGN KEY (`import_job_id`) REFERENCES `zatca_import_job` (`id`)
    ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `chk_zatca_import_history_to_status`
    CHECK (`to_status` IN ('review','approved','posted','rejected')),
  CONSTRAINT `chk_zatca_import_history_from_status`
    CHECK (`from_status` IS NULL OR
           `from_status` IN ('review','approved','posted','rejected'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
