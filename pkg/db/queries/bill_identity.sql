-- Canonical bill identity is scoped by the master tenant and POS identity.
-- A matching fingerprint is a replay; a different fingerprint is a conflict.

-- name: ResolveBillRequest :one
SELECT
  b.id,
  b.tenant_id,
  b.pos_id,
  b.bill_request_id,
  b.request_fingerprint,
  b.request_fingerprint_version,
  b.invoice_uuid,
  CASE
    WHEN b.request_fingerprint = sqlc.arg('request_fingerprint')
     AND b.request_fingerprint_version = sqlc.arg('request_fingerprint_version')
      THEN 'replay'
    ELSE 'conflict'
  END AS request_resolution
FROM bill AS b
WHERE b.tenant_id = sqlc.arg('tenant_id')
  AND b.pos_id = sqlc.arg('pos_id')
  AND b.bill_request_id = sqlc.arg('bill_request_id')
LIMIT 1;

-- name: GetBillByInvoiceUUID :one
SELECT
  b.id,
  b.tenant_id,
  b.pos_id,
  b.bill_request_id,
  b.request_fingerprint,
  b.request_fingerprint_version,
  b.invoice_uuid
FROM bill AS b
WHERE b.tenant_id = sqlc.arg('tenant_id')
  AND b.invoice_uuid = sqlc.arg('invoice_uuid')
LIMIT 1;

-- name: GetOutboxEventByTenant :one
SELECT o.*
FROM outbox AS o
WHERE o.tenant_id = sqlc.arg('tenant_id')
  AND o.event_id = sqlc.arg('event_id')
LIMIT 1;

-- name: GetZatcaSubmissionByBillRequest :one
SELECT
  zs.id,
  zs.tenant_id,
  zs.pos_id,
  zs.bill_id,
  zs.bill_request_id,
  zs.invoice_uuid,
  zs.status,
  zs.rejection_code,
  zs.rejection_msg,
  zs.submitted_at,
  zs.cleared_at,
  zs.response_xml,
  zs.retry_count,
  zs.created_at,
  zs.updated_at
FROM zatca_submission AS zs
WHERE zs.tenant_id = sqlc.arg('tenant_id')
  AND zs.pos_id = sqlc.arg('pos_id')
  AND zs.bill_request_id = sqlc.arg('bill_request_id')
LIMIT 1;
