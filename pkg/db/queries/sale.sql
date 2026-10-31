-- Canonical atomic sale command queries.

-- name: GetSaleOwnership :one
SELECT
  s.id,
  s.branch_id,
  s.company_id
FROM store AS s
JOIN branches AS b ON b.id = s.branch_id
JOIN `user` AS u ON u.id = sqlc.arg('user_id')
WHERE s.id = sqlc.arg('store_id')
  AND s.branch_id = sqlc.arg('branch_id')
  AND b.company_id = s.company_id
  AND u.company_id = s.company_id
LIMIT 1
FOR UPDATE;

-- name: GetSaleProductForUpdate :one
SELECT
  p.id,
  p.store_id,
  p.quantity
FROM product AS p
WHERE p.id = sqlc.arg('product_id')
  AND p.store_id = sqlc.arg('store_id')
  AND p.is_deleted = FALSE
LIMIT 1
FOR UPDATE;

-- name: InsertBillPayment :execresult
INSERT INTO bill_payment (
  bill_id, date, paid_at, amount, currency_id, payment_method, recorded_by, note
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertSaleOutbox :execresult
INSERT INTO outbox (
  tenant_id, event_id, aggregate_type, aggregate_id, pos_id,
  bill_request_id, invoice_uuid, egs_sequence_id, payload,
  payload_fingerprint, content_hash
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
