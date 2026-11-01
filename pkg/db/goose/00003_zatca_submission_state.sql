-- +goose Up
-- Durable ZATCA submission state (B2). zatca_submission keeps its numeric status and response
-- columns for compatibility. From now on outcome_state is the source of truth. Attempts,
-- outcomes and artifacts are separate tables. The claim table lets the daemon process each
-- request once. Columns are appended, because sqlc appends them too.
ALTER TABLE zatca_submission
  ADD COLUMN outcome_state VARCHAR(32) NOT NULL DEFAULT 'pending',
  ADD COLUMN provider_code VARCHAR(64) NULL,
  ADD COLUMN provider_warning_code VARCHAR(64) NULL,
  ADD COLUMN provider_warning_message VARCHAR(1024) NULL,
  ADD COLUMN provider_reference VARCHAR(255) NULL,
  ADD COLUMN last_attempt_at DATETIME NULL,
  ADD COLUMN next_retry_at DATETIME NULL,
  ADD COLUMN outcome_at DATETIME NULL,
  ADD COLUMN request_xml_path VARCHAR(500) NULL,
  ADD COLUMN cleared_xml_path VARCHAR(500) NULL,
  ADD KEY idx_zatca_outcome_state (outcome_state),
  ADD KEY idx_zatca_retry (next_retry_at),
  ADD CONSTRAINT chk_zatca_submission_outcome_state CHECK (outcome_state IN ('pending', 'accepted', 'accepted_with_warning', 'rejected', 'ambiguous', 'terminal_failure'));

-- Existing rows: the outcome follows the numeric status (2 accepted, 3 rejected, 4 warning).
UPDATE zatca_submission
SET outcome_state = CASE status
  WHEN 2 THEN 'accepted'
  WHEN 3 THEN 'rejected'
  WHEN 4 THEN 'accepted_with_warning'
  ELSE 'pending'
END;

CREATE TABLE zatca_submission_attempt (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  submission_id INT NOT NULL,
  attempt_number INT UNSIGNED NOT NULL,
  attempt_state VARCHAR(32) NOT NULL DEFAULT 'pending',
  provider_code VARCHAR(64) NULL,
  provider_warning_code VARCHAR(64) NULL,
  provider_warning_message VARCHAR(1024) NULL,
  provider_reference VARCHAR(255) NULL,
  error_code VARCHAR(64) NULL,
  error_message VARCHAR(1024) NULL,
  started_at DATETIME NOT NULL,
  completed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_zatca_attempt_number (submission_id, attempt_number),
  CONSTRAINT chk_zatca_attempt_state CHECK (attempt_state IN ('pending', 'accepted', 'accepted_with_warning', 'rejected', 'ambiguous', 'terminal_failure')),
  CONSTRAINT fk_zatca_attempt_submission FOREIGN KEY (submission_id) REFERENCES zatca_submission (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Outcome and artifact rows are append-only. Only sqlc queries write to this database, and
-- TestAppendOnlyTablesHaveNoUpdateOrDelete fails if a query updates or deletes them. A database
-- trigger would enforce the same rule, but sqlc cannot parse CREATE TRIGGER.
CREATE TABLE zatca_submission_outcome (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  submission_id INT NOT NULL,
  attempt_id BIGINT UNSIGNED NULL,
  outcome_state VARCHAR(32) NOT NULL,
  provider_code VARCHAR(64) NULL,
  provider_warning_code VARCHAR(64) NULL,
  provider_warning_message VARCHAR(1024) NULL,
  provider_reference VARCHAR(255) NULL,
  observed_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_zatca_outcome_latest (submission_id, observed_at, id),
  CONSTRAINT chk_zatca_outcome_state CHECK (outcome_state IN ('pending', 'accepted', 'accepted_with_warning', 'rejected', 'ambiguous', 'terminal_failure')),
  CONSTRAINT fk_zatca_outcome_submission FOREIGN KEY (submission_id) REFERENCES zatca_submission (id) ON DELETE CASCADE,
  CONSTRAINT fk_zatca_outcome_attempt FOREIGN KEY (attempt_id) REFERENCES zatca_submission_attempt (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE zatca_submission_artifact (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  submission_id INT NOT NULL,
  attempt_id BIGINT UNSIGNED NULL,
  artifact_kind VARCHAR(32) NOT NULL,
  artifact_path VARCHAR(500) NULL,
  content_type VARCHAR(128) NULL,
  sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  metadata JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_zatca_artifact_submission (submission_id, created_at),
  CONSTRAINT chk_zatca_artifact_kind CHECK (artifact_kind IN ('request_xml', 'cleared_xml', 'response_xml', 'other')),
  CONSTRAINT chk_zatca_artifact_sha256 CHECK (sha256 IS NULL OR sha256 REGEXP '^[0-9a-f]{64}$'),
  CONSTRAINT fk_zatca_artifact_submission FOREIGN KEY (submission_id) REFERENCES zatca_submission (id) ON DELETE CASCADE,
  CONSTRAINT fk_zatca_artifact_attempt FOREIGN KEY (attempt_id) REFERENCES zatca_submission_attempt (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- The claim table is owned here, not by the daemon. One request (pos_id, bill_request_id) is
-- claimed once in the tenant database, and each outbox event id is claimed once.
CREATE TABLE zatca_consumer_claim (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  pos_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL,
  bill_request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_id VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL,
  egs_sequence_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(128) NOT NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'processing',
  lease_expires_at DATETIME NULL,
  completed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_zatca_claim_request (pos_id, bill_request_id),
  UNIQUE KEY uq_zatca_claim_event (event_id),
  KEY idx_zatca_claim_lease (state, lease_expires_at),
  CONSTRAINT chk_zatca_claim_state CHECK (state IN ('processing', 'completed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE zatca_consumer_claim;
DROP TABLE zatca_submission_artifact;
DROP TABLE zatca_submission_outcome;
DROP TABLE zatca_submission_attempt;
ALTER TABLE zatca_submission
  DROP CHECK chk_zatca_submission_outcome_state,
  DROP INDEX idx_zatca_retry,
  DROP INDEX idx_zatca_outcome_state,
  DROP COLUMN cleared_xml_path,
  DROP COLUMN request_xml_path,
  DROP COLUMN outcome_at,
  DROP COLUMN next_retry_at,
  DROP COLUMN last_attempt_at,
  DROP COLUMN provider_reference,
  DROP COLUMN provider_warning_message,
  DROP COLUMN provider_warning_code,
  DROP COLUMN provider_code,
  DROP COLUMN outcome_state;
