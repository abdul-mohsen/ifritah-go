-- +goose Up
-- Transactional outbox (B3). A business change writes its event in the same transaction, so the
-- event exists only if the change commits. pkg/outbox publishes the event after commit. Delivery
-- is at least once: a crash after publishing and before the result is recorded publishes the
-- event again. Consumers must deduplicate by event_id. JetStream also drops a repeat that carries
-- the same message ID, inside the stream's duplicate window.
CREATE TABLE outbox (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  document_key VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL,
  subject VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL,
  payload JSON NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  next_attempt_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  lease_owner VARCHAR(128) NULL,
  lease_expires_at DATETIME NULL,
  last_error VARCHAR(1024) NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  published_at DATETIME NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_outbox_event (event_id),
  KEY idx_outbox_claim (status, next_attempt_at, lease_expires_at, id),
  CONSTRAINT chk_outbox_status CHECK (status IN ('pending', 'in_flight', 'published', 'dead_letter'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE outbox;
