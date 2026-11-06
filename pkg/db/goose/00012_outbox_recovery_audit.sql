-- +goose Up
-- An operator recovery never edits the business row or event payload. This append-only table
-- records who deliberately requeued a dead-letter event, why, and which delivery state was
-- replaced. The event ID remains the consumer's stable deduplication key.
CREATE TABLE outbox_recovery_audit (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  outbox_id BIGINT UNSIGNED NOT NULL,
  event_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  idempotency_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  operator_user_id INT NULL,
  operator_username VARCHAR(45) NOT NULL,
  reason VARCHAR(1024) NOT NULL,
  previous_status VARCHAR(16) NOT NULL,
  previous_attempt_count INT UNSIGNED NOT NULL,
  previous_last_error VARCHAR(1024) NULL,
  recovered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_outbox_recovery_idempotency (idempotency_key),
  KEY idx_outbox_recovery_event (outbox_id, recovered_at, id),
  CONSTRAINT fk_outbox_recovery_event
    FOREIGN KEY (outbox_id) REFERENCES outbox (id),
  CONSTRAINT fk_outbox_recovery_operator
    FOREIGN KEY (operator_user_id) REFERENCES user (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE outbox_recovery_audit;
