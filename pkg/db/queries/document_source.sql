-- Canonical source mapping shared by bills, credit notes, and debit notes.
-- The view owns the polymorphic mapping; callers must always scope by tenant.

-- name: GetZatcaDocumentSource :one
SELECT
  tenant_id,
  document_type,
  source_id,
  pos_id,
  request_id,
  invoice_uuid,
  branch_id,
  sequence_number,
  original_document_type,
  original_source_id,
  original_invoice_uuid
FROM zatca_document_source
WHERE tenant_id = sqlc.arg('tenant_id')
  AND document_type = sqlc.arg('document_type')
  AND source_id = sqlc.arg('source_id')
LIMIT 1;

-- name: GetZatcaDocumentSourceByInvoiceUUID :one
SELECT
  tenant_id,
  document_type,
  source_id,
  pos_id,
  request_id,
  invoice_uuid,
  branch_id,
  sequence_number,
  original_document_type,
  original_source_id,
  original_invoice_uuid
FROM zatca_document_source
WHERE tenant_id = sqlc.arg('tenant_id')
  AND invoice_uuid = sqlc.arg('invoice_uuid')
LIMIT 1;

-- name: GetZatcaDocumentSourceByRequest :one
SELECT
  tenant_id,
  document_type,
  source_id,
  pos_id,
  request_id,
  invoice_uuid,
  branch_id,
  sequence_number,
  original_document_type,
  original_source_id,
  original_invoice_uuid
FROM zatca_document_source
WHERE tenant_id = sqlc.arg('tenant_id')
  AND pos_id = sqlc.arg('pos_id')
  AND request_id = sqlc.arg('request_id')
LIMIT 1;
