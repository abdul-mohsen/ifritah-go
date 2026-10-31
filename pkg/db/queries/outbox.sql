-- Transactional outbox claim, retry, and operator-state commands.

-- name: GetOutboxEventBySaleIdentity :one
SELECT o.*
FROM outbox AS o
WHERE o.tenant_id = sqlc.arg('tenant_id')
  AND o.pos_id = sqlc.arg('pos_id')
  AND o.bill_request_id = sqlc.arg('bill_request_id')
LIMIT 1;

-- name: ClaimOutboxEvent :execresult
UPDATE outbox
SET
  status = 'in_flight',
  lease_owner = sqlc.arg('lease_owner'),
  lease_expires_at = sqlc.arg('lease_expires_at'),
  attempt_count = attempt_count + 1,
  updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = sqlc.arg('tenant_id')
  AND event_id = sqlc.arg('event_id')
  AND (
    (
      status IN ('pending', 'retryable')
      AND (next_attempt_at IS NULL OR next_attempt_at <= CURRENT_TIMESTAMP)
    )
    OR (
      status = 'in_flight'
      AND lease_expires_at IS NOT NULL
      AND lease_expires_at <= CURRENT_TIMESTAMP
    )
  )
;

-- name: MarkOutboxPublished :exec
UPDATE outbox
SET
  status = 'published',
  published_at = CURRENT_TIMESTAMP,
  lease_owner = NULL,
  lease_expires_at = NULL,
  last_error = NULL,
  updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = sqlc.arg('tenant_id')
  AND event_id = sqlc.arg('event_id')
  AND status = 'in_flight'
  AND lease_owner = sqlc.arg('lease_owner');

-- name: MarkOutboxPublishFailed :exec
UPDATE outbox
SET
  status = sqlc.arg('status'),
  next_attempt_at = sqlc.arg('next_attempt_at'),
  last_error = sqlc.arg('last_error'),
  dead_lettered_at = sqlc.arg('dead_lettered_at'),
  manual_review_at = sqlc.arg('manual_review_at'),
  lease_owner = NULL,
  lease_expires_at = NULL,
  updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = sqlc.arg('tenant_id')
  AND event_id = sqlc.arg('event_id')
  AND status = 'in_flight'
  AND lease_owner = sqlc.arg('lease_owner');

-- name: MarkOutboxManualReview :exec
UPDATE outbox
SET
  status = 'manual_review',
  manual_review_at = CURRENT_TIMESTAMP,
  lease_owner = NULL,
  lease_expires_at = NULL,
  updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = sqlc.arg('tenant_id')
  AND event_id = sqlc.arg('event_id')
  AND status IN ('retryable', 'dead_letter', 'in_flight');
