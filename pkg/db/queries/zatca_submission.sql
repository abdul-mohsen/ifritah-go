-- Durable, source-agnostic ZATCA submission and outcome contract.

-- name: UpsertZatcaSubmission :execresult
INSERT INTO zatca_submission (
  tenant_id,
  document_type,
  source_id,
  pos_id,
  request_id,
  invoice_uuid,
  original_document_type,
  original_source_id,
  original_invoice_uuid,
  status,
  rejection_code,
  rejection_msg,
  submitted_at,
  cleared_at,
  response_xml,
  retry_count
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  rejection_code = VALUES(rejection_code),
  rejection_msg = VALUES(rejection_msg),
  submitted_at = VALUES(submitted_at),
  cleared_at = VALUES(cleared_at),
  response_xml = VALUES(response_xml),
  retry_count = VALUES(retry_count),
  updated_at = CURRENT_TIMESTAMP;

-- name: RecordZatcaSubmissionOutcome :exec
UPDATE zatca_submission
SET
  status = sqlc.arg('status'),
  rejection_code = sqlc.arg('rejection_code'),
  rejection_msg = sqlc.arg('rejection_msg'),
  submitted_at = sqlc.arg('submitted_at'),
  cleared_at = sqlc.arg('cleared_at'),
  response_xml = sqlc.arg('response_xml'),
  retry_count = sqlc.arg('retry_count'),
  updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = sqlc.arg('tenant_id')
  AND document_type = sqlc.arg('document_type')
  AND source_id = sqlc.arg('source_id');

-- name: GetZatcaSubmissionByDocument :one
SELECT
  id,
  tenant_id,
  document_type,
  source_id,
  pos_id,
  request_id,
  invoice_uuid,
  original_document_type,
  original_source_id,
  original_invoice_uuid,
  status,
  rejection_code,
  rejection_msg,
  submitted_at,
  cleared_at,
  response_xml,
  retry_count,
  created_at,
  updated_at
FROM zatca_submission
WHERE tenant_id = sqlc.arg('tenant_id')
  AND document_type = sqlc.arg('document_type')
  AND source_id = sqlc.arg('source_id')
LIMIT 1;

-- name: GetZatcaSubmissionByRequest :one
SELECT
  id,
  tenant_id,
  document_type,
  source_id,
  pos_id,
  request_id,
  invoice_uuid,
  original_document_type,
  original_source_id,
  original_invoice_uuid,
  status,
  rejection_code,
  rejection_msg,
  submitted_at,
  cleared_at,
  response_xml,
  retry_count,
  created_at,
  updated_at
FROM zatca_submission
WHERE tenant_id = sqlc.arg('tenant_id')
  AND pos_id = sqlc.arg('pos_id')
  AND request_id = sqlc.arg('request_id')
LIMIT 1;

-- name: GetZatcaSubmissionByInvoiceUUID :one
SELECT
  id,
  tenant_id,
  document_type,
  source_id,
  pos_id,
  request_id,
  invoice_uuid,
  original_document_type,
  original_source_id,
  original_invoice_uuid,
  status,
  rejection_code,
  rejection_msg,
  submitted_at,
  cleared_at,
  response_xml,
  retry_count,
  created_at,
  updated_at
FROM zatca_submission
WHERE tenant_id = sqlc.arg('tenant_id')
  AND invoice_uuid = sqlc.arg('invoice_uuid')
LIMIT 1;
