-- Transactional outbox (B3). pkg/outbox writes events in the business transaction and drains
-- them after commit. Each lease is owned by one worker, and a write only applies to the lease
-- holder's row.

-- name: InsertOutboxEvent :exec
INSERT INTO outbox (event_id, document_key, subject, payload, next_attempt_at)
VALUES (sqlc.arg('event_id'), sqlc.arg('document_key'), sqlc.arg('subject'), sqlc.arg('payload'), sqlc.arg('next_attempt_at'));

-- name: ListDueOutboxEvents :many
SELECT id, event_id, subject, payload, attempt_count
FROM outbox
WHERE (status = 'pending' AND next_attempt_at <= sqlc.arg('due_at'))
   OR (status = 'in_flight' AND lease_expires_at <= sqlc.arg('expired_at'))
ORDER BY id
LIMIT ?
FOR UPDATE SKIP LOCKED;

-- name: LeaseOutboxEvent :exec
UPDATE outbox
SET status = 'in_flight',
    lease_owner = sqlc.arg('lease_owner'),
    lease_expires_at = sqlc.arg('lease_expires_at'),
    attempt_count = attempt_count + 1
WHERE id = sqlc.arg('id');

-- name: MarkOutboxPublished :exec
UPDATE outbox
SET status = 'published',
    published_at = sqlc.arg('published_at'),
    lease_owner = NULL,
    lease_expires_at = NULL,
    last_error = NULL
WHERE id = sqlc.arg('id') AND lease_owner = sqlc.arg('lease_owner');

-- name: MarkOutboxRetry :exec
UPDATE outbox
SET status = 'pending',
    next_attempt_at = sqlc.arg('next_attempt_at'),
    last_error = sqlc.arg('last_error'),
    lease_owner = NULL,
    lease_expires_at = NULL
WHERE id = sqlc.arg('id') AND lease_owner = sqlc.arg('lease_owner');

-- name: MarkOutboxDeadLetter :exec
UPDATE outbox
SET status = 'dead_letter',
    last_error = sqlc.arg('last_error'),
    lease_owner = NULL,
    lease_expires_at = NULL
WHERE id = sqlc.arg('id') AND lease_owner = sqlc.arg('lease_owner');
