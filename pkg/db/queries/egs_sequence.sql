-- Canonical EGS sequence contract.
-- egs_sequence_id is the sole sequence identity; tenant, branch, VAT, and EGS
-- ownership are explicit predicates and never aliases for user/company/POS IDs.

-- name: EnsureEGSSequence :exec
INSERT INTO egs_sequence (
  egs_sequence_id, tenant_id, branch_id, vat_number, egs_id
)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE egs_sequence_id = egs_sequence_id;

-- name: GetEGSSequenceForUpdate :one
SELECT
  egs_sequence_id,
  tenant_id,
  branch_id,
  vat_number,
  egs_id,
  last_icv,
  previous_invoice_hash,
  reservation_key,
  reservation_token,
  reserved_icv,
  reserved_previous_invoice_hash,
  reserved_document_uuid,
  reserved_outbox_event_id,
  reserved_at,
  lease_expires_at,
  created_at,
  updated_at
FROM egs_sequence
WHERE egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND branch_id = sqlc.arg('branch_id')
  AND vat_number = sqlc.arg('vat_number')
  AND egs_id = sqlc.arg('egs_id')
FOR UPDATE;

-- name: ReserveEGSSequence :execresult
UPDATE egs_sequence
SET
  reservation_key = sqlc.arg('reservation_key'),
  reservation_token = sqlc.arg('reservation_token'),
  reserved_icv = last_icv + 1,
  reserved_previous_invoice_hash = previous_invoice_hash,
  reserved_document_uuid = sqlc.arg('document_uuid'),
  reserved_outbox_event_id = sqlc.arg('outbox_event_id'),
  reserved_at = CURRENT_TIMESTAMP,
  lease_expires_at = sqlc.arg('lease_expires_at')
WHERE egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND branch_id = sqlc.arg('branch_id')
  AND vat_number = sqlc.arg('vat_number')
  AND egs_id = sqlc.arg('egs_id')
  AND (
    reservation_key IS NULL
    OR lease_expires_at IS NULL
    OR lease_expires_at <= CURRENT_TIMESTAMP
  );

-- name: InsertZatcaEGSDocument :execresult
INSERT INTO zatca_egs_document (
  tenant_id, egs_sequence_id, branch_id, vat_number, egs_id,
  document_uuid, document_type, source_id, outbox_event_id,
  icv, pih, document_snapshot, state
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CommitEGSSequence :execresult
UPDATE egs_sequence AS s
SET
  last_icv = s.reserved_icv,
  previous_invoice_hash = sqlc.arg('previous_invoice_hash'),
  reservation_key = NULL,
  reservation_token = NULL,
  reserved_icv = NULL,
  reserved_previous_invoice_hash = NULL,
  reserved_document_uuid = NULL,
  reserved_outbox_event_id = NULL,
  reserved_at = NULL,
  lease_expires_at = NULL
WHERE s.egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND s.tenant_id = sqlc.arg('tenant_id')
  AND s.branch_id = sqlc.arg('branch_id')
  AND s.vat_number = sqlc.arg('vat_number')
  AND s.egs_id = sqlc.arg('egs_id')
  AND s.reservation_key = sqlc.arg('reservation_key')
  AND s.reservation_token = sqlc.arg('reservation_token')
  AND s.reserved_icv = sqlc.arg('reserved_icv')
  AND s.reserved_document_uuid = sqlc.arg('document_uuid')
  AND s.reserved_outbox_event_id = sqlc.arg('outbox_event_id')
  AND EXISTS (
    SELECT 1
    FROM zatca_egs_document AS d
    JOIN outbox AS o
      ON o.tenant_id = d.tenant_id
     AND o.event_id = d.outbox_event_id
     AND o.egs_sequence_id = d.egs_sequence_id
     AND o.invoice_uuid = d.document_uuid
     AND o.icv = d.icv
    WHERE d.tenant_id = s.tenant_id
      AND d.egs_sequence_id = s.egs_sequence_id
      AND d.branch_id = s.branch_id
      AND d.vat_number = s.vat_number
      AND d.egs_id = s.egs_id
      AND d.document_uuid = s.reserved_document_uuid
      AND d.outbox_event_id = s.reserved_outbox_event_id
      AND d.icv = s.reserved_icv
      AND d.state NOT IN ('rejected', 'ambiguous')
  );

-- name: ReleaseEGSSequence :execresult
UPDATE egs_sequence
SET
  reservation_key = NULL,
  reservation_token = NULL,
  reserved_icv = NULL,
  reserved_previous_invoice_hash = NULL,
  reserved_document_uuid = NULL,
  reserved_outbox_event_id = NULL,
  reserved_at = NULL,
  lease_expires_at = NULL
WHERE egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND branch_id = sqlc.arg('branch_id')
  AND vat_number = sqlc.arg('vat_number')
  AND egs_id = sqlc.arg('egs_id')
  AND reservation_key = sqlc.arg('reservation_key')
  AND reservation_token = sqlc.arg('reservation_token');

-- name: GetZatcaEGSDocumentByUUID :one
SELECT
  id,
  tenant_id,
  egs_sequence_id,
  branch_id,
  vat_number,
  egs_id,
  document_uuid,
  document_type,
  source_id,
  outbox_event_id,
  icv,
  pih,
  document_snapshot,
  state,
  created_at
FROM zatca_egs_document
WHERE tenant_id = sqlc.arg('tenant_id')
  AND document_uuid = sqlc.arg('document_uuid')
LIMIT 1;
