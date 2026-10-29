-- name: InsertZatcaEventOutbox :exec
INSERT INTO zatca_event_outbox (
  event_id,
  tenant_id,
  merchant_id,
  document_type,
  document_id,
  branch_id,
  idempotency_key,
  payload
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  updated_at = CURRENT_TIMESTAMP;

-- name: ListDueZatcaEventOutbox :many
SELECT
  id,
  event_id,
  tenant_id,
  merchant_id,
  document_type,
  document_id,
  branch_id,
  idempotency_key,
  payload,
  status,
  attempt_count,
  available_at,
  claimed_at,
  published_at,
  last_error,
  last_error_at,
  created_at,
  updated_at
FROM zatca_event_outbox
WHERE (
    (status IN ('pending', 'failed') AND available_at <= CURRENT_TIMESTAMP)
    OR (
      status = 'processing'
      AND claimed_at IS NOT NULL
      AND claimed_at < DATE_SUB(CURRENT_TIMESTAMP, INTERVAL 5 MINUTE)
    )
  )
ORDER BY id
LIMIT ?
FOR UPDATE;

-- name: MarkZatcaEventOutboxProcessing :exec
UPDATE zatca_event_outbox
SET status = 'processing',
    claimed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: MarkZatcaEventOutboxPublished :exec
UPDATE zatca_event_outbox
SET status = 'published',
    claimed_at = NULL,
    published_at = CURRENT_TIMESTAMP,
    last_error = NULL,
    last_error_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
  AND status = 'processing';

-- name: MarkZatcaEventOutboxFailed :exec
UPDATE zatca_event_outbox
SET status = 'failed',
    attempt_count = attempt_count + 1,
    available_at = ?,
    claimed_at = NULL,
    last_error = ?,
    last_error_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
  AND status = 'processing';
