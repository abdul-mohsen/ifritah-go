-- Operator recovery of dead-lettered outbox events. Recovery changes delivery state only;
-- business rows and the event payload remain untouched.

-- name: ListDeadLetterOutboxEvents :many
SELECT
  id,
  event_id,
  document_key,
  subject,
  payload,
  status,
  attempt_count,
  next_attempt_at,
  lease_owner,
  lease_expires_at,
  last_error,
  created_at,
  published_at,
  updated_at
FROM outbox
WHERE status = 'dead_letter'
ORDER BY updated_at DESC, id DESC
LIMIT ?;

-- name: GetOutboxEventByEventID :one
SELECT
  id,
  event_id,
  document_key,
  subject,
  payload,
  status,
  attempt_count,
  next_attempt_at,
  lease_owner,
  lease_expires_at,
  last_error,
  created_at,
  published_at,
  updated_at
FROM outbox
WHERE event_id = ?
LIMIT 1;

-- name: GetOutboxEventForRecovery :one
SELECT
  id,
  event_id,
  document_key,
  subject,
  payload,
  status,
  attempt_count,
  next_attempt_at,
  lease_owner,
  lease_expires_at,
  last_error,
  created_at,
  published_at,
  updated_at
FROM outbox
WHERE event_id = ?
LIMIT 1
FOR UPDATE;

-- name: RecoverOutboxEvent :execresult
UPDATE outbox
SET status = 'pending',
    attempt_count = 0,
    next_attempt_at = ?,
    lease_owner = NULL,
    lease_expires_at = NULL
WHERE id = ? AND status = 'dead_letter';

-- name: GetOutboxRecoveryAuditByIdempotencyKey :one
SELECT
  id,
  outbox_id,
  event_id,
  idempotency_key,
  operator_user_id,
  operator_username,
  reason,
  previous_status,
  previous_attempt_count,
  previous_last_error,
  recovered_at
FROM outbox_recovery_audit
WHERE idempotency_key = ?
LIMIT 1;

-- name: InsertOutboxRecoveryAudit :execresult
INSERT INTO outbox_recovery_audit (
  outbox_id,
  event_id,
  idempotency_key,
  operator_user_id,
  operator_username,
  reason,
  previous_status,
  previous_attempt_count,
  previous_last_error,
  recovered_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListOutboxRecoveryAudit :many
SELECT
  id,
  outbox_id,
  event_id,
  idempotency_key,
  operator_user_id,
  operator_username,
  reason,
  previous_status,
  previous_attempt_count,
  previous_last_error,
  recovered_at
FROM outbox_recovery_audit
WHERE event_id = ?
ORDER BY recovered_at DESC, id DESC;
