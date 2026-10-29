-- name: GetDebitNoteSourceContext :one
SELECT
  b.id AS bill_id,
  b.merchant_id,
  b.branch_id,
  b.sequence_number,
  CAST(COALESCE(
    NULLIF(bzc.csr_computer_number, ''),
    CONCAT(sqlc.arg('tenant_id'), ':branch:', b.branch_id)
  ) AS CHAR) AS egs_id
FROM bill b
LEFT JOIN branch_zatca_config bzc ON bzc.branch_id = b.branch_id
WHERE b.id = sqlc.arg('bill_id')
  AND b.merchant_id = sqlc.arg('merchant_id')
  AND b.branch_id IS NOT NULL
FOR UPDATE;

-- name: InsertDebitNote :execresult
INSERT INTO debit_note (
  bill_id,
  merchant_id,
  branch_id,
  sequence_number,
  egs_id,
  state,
  note
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: SetDebitNoteIdentity :exec
UPDATE debit_note
SET document_id = ?,
    stable_document_key = ?
WHERE id = ?
  AND merchant_id = ?
  AND document_id IS NULL
  AND stable_document_key IS NULL;

-- name: GetDebitNotePublicationContext :one
SELECT
  dn.id,
  dn.bill_id,
  dn.merchant_id,
  dn.branch_id,
  dn.sequence_number,
  dn.egs_id,
  dn.document_id,
  dn.stable_document_key
FROM debit_note dn
JOIN bill b ON b.id = dn.bill_id
WHERE dn.id = sqlc.arg('debit_note_id')
  AND dn.merchant_id = sqlc.arg('merchant_id')
  AND b.merchant_id = sqlc.arg('merchant_id')
  AND dn.branch_id = sqlc.arg('branch_id')
  AND dn.document_id IS NOT NULL
  AND dn.stable_document_key IS NOT NULL
LIMIT 1;

-- name: RegisterDebitZatcaSubmission :exec
INSERT INTO zatca_submission (
  bill_id,
  document_type,
  source_id,
  merchant_id,
  status,
  document_id,
  stable_document_key,
  sequence_number,
  egs_id,
  outcome_state
) VALUES (?, 'debit', ?, ?, 0, ?, ?, ?, ?, 'pending')
ON DUPLICATE KEY UPDATE
  id = LAST_INSERT_ID(id);
