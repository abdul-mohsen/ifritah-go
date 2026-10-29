-- T031 import-lane persistence. These queries never touch outbound
-- zatca_submission or zatca_event_outbox.

-- name: InsertZatcaImportJob :execresult
INSERT INTO zatca_import_job (
  merchant_id,
  branch_id,
  status,
  workflow_version,
  source_content_hash,
  idempotency_key,
  created_by
) VALUES (
  sqlc.arg('merchant_id'),
  sqlc.narg('branch_id'),
  'review',
  1,
  sqlc.arg('source_content_hash'),
  sqlc.narg('idempotency_key'),
  sqlc.arg('created_by')
);

-- name: InsertZatcaImportEvidence :exec
INSERT INTO zatca_import_evidence (
  import_job_id,
  merchant_id,
  source_reference,
  source_original_name,
  source_mime_type,
  source_provenance,
  source_content_hash,
  source_size,
  original_bytes,
  canonical_data,
  parsed_data,
  signature_data,
  validation_data,
  result_data,
  failure_class,
  failure_message
) VALUES (
  sqlc.arg('import_job_id'),
  sqlc.arg('merchant_id'),
  sqlc.arg('source_reference'),
  sqlc.narg('source_original_name'),
  sqlc.arg('source_mime_type'),
  sqlc.arg('source_provenance'),
  sqlc.arg('source_content_hash'),
  sqlc.arg('source_size'),
  sqlc.narg('original_bytes'),
  sqlc.narg('canonical_data'),
  sqlc.narg('parsed_data'),
  sqlc.narg('signature_data'),
  sqlc.narg('validation_data'),
  sqlc.narg('result_data'),
  sqlc.narg('failure_class'),
  sqlc.narg('failure_message')
);

-- name: InsertZatcaImportValidationError :exec
INSERT INTO zatca_import_validation_error (
  import_job_id,
  merchant_id,
  severity,
  error_code,
  field_key,
  message,
  details,
  created_by
) VALUES (
  sqlc.arg('import_job_id'),
  sqlc.arg('merchant_id'),
  sqlc.arg('severity'),
  sqlc.arg('error_code'),
  sqlc.narg('field_key'),
  sqlc.arg('message'),
  sqlc.narg('details'),
  sqlc.arg('created_by')
);

-- name: InsertZatcaImportStatusHistory :exec
INSERT INTO zatca_import_status_history (
  import_job_id,
  merchant_id,
  from_status,
  to_status,
  transition_key,
  workflow_version,
  reason,
  created_by
) VALUES (
  sqlc.arg('import_job_id'),
  sqlc.arg('merchant_id'),
  sqlc.narg('from_status'),
  sqlc.arg('to_status'),
  sqlc.arg('transition_key'),
  sqlc.arg('workflow_version'),
  sqlc.narg('reason'),
  sqlc.arg('created_by')
);

-- name: InsertZatcaImportReview :exec
INSERT INTO zatca_import_review (
  import_job_id,
  merchant_id,
  decision,
  reason,
  expected_workflow_version,
  observed_workflow_version,
  created_by
) VALUES (
  sqlc.arg('import_job_id'),
  sqlc.arg('merchant_id'),
  sqlc.arg('decision'),
  sqlc.narg('reason'),
  sqlc.narg('expected_workflow_version'),
  sqlc.arg('observed_workflow_version'),
  sqlc.arg('created_by')
);

-- name: GetZatcaImportJobForTenant :one
SELECT
  id,
  merchant_id,
  branch_id,
  status,
  workflow_version,
  source_content_hash,
  reviewed_by,
  reviewed_at,
  posted_by,
  posted_at,
  post_reference,
  rejected_by,
  rejected_at,
  rejected_reason,
  created_by,
  created_at,
  updated_at
FROM zatca_import_job
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id')
LIMIT 1;

-- name: GetZatcaImportJobForUpdate :one
SELECT
  id,
  merchant_id,
  branch_id,
  status,
  workflow_version,
  source_content_hash,
  reviewed_by,
  reviewed_at,
  posted_by,
  posted_at,
  post_reference,
  rejected_by,
  rejected_at,
  rejected_reason,
  created_by,
  created_at,
  updated_at
FROM zatca_import_job
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id')
LIMIT 1
FOR UPDATE;

-- name: GetZatcaImportJobByContentHash :one
SELECT
  id,
  merchant_id,
  branch_id,
  status,
  workflow_version,
  source_content_hash,
  reviewed_by,
  reviewed_at,
  posted_by,
  posted_at,
  post_reference,
  rejected_by,
  rejected_at,
  rejected_reason,
  created_by,
  created_at,
  updated_at
FROM zatca_import_job
WHERE merchant_id = sqlc.arg('merchant_id')
  AND source_content_hash = sqlc.arg('source_content_hash')
LIMIT 1;

-- name: GetZatcaImportJobByIdempotencyKey :one
SELECT
  id,
  merchant_id,
  branch_id,
  status,
  workflow_version,
  source_content_hash,
  reviewed_by,
  reviewed_at,
  posted_by,
  posted_at,
  post_reference,
  rejected_by,
  rejected_at,
  rejected_reason,
  created_by,
  created_at,
  updated_at
FROM zatca_import_job
WHERE merchant_id = sqlc.arg('merchant_id')
  AND idempotency_key = sqlc.arg('idempotency_key')
LIMIT 1;

-- name: ListZatcaImportJobs :many
SELECT
  id,
  merchant_id,
  branch_id,
  status,
  workflow_version,
  source_content_hash,
  reviewed_by,
  reviewed_at,
  posted_by,
  posted_at,
  post_reference,
  rejected_by,
  rejected_at,
  rejected_reason,
  created_by,
  created_at,
  updated_at
FROM zatca_import_job
WHERE merchant_id = sqlc.arg('merchant_id')
  AND (sqlc.arg('status_filter') = '' OR status = sqlc.arg('status_filter'))
ORDER BY created_at DESC, id DESC
LIMIT ?;

-- name: GetZatcaImportEvidence :one
SELECT
  id,
  import_job_id,
  merchant_id,
  source_reference,
  source_original_name,
  source_mime_type,
  source_provenance,
  source_content_hash,
  source_size,
  original_bytes,
  COALESCE(canonical_data, JSON_OBJECT()) AS canonical_data,
  COALESCE(parsed_data, JSON_OBJECT()) AS parsed_data,
  COALESCE(signature_data, JSON_OBJECT()) AS signature_data,
  COALESCE(validation_data, JSON_OBJECT()) AS validation_data,
  COALESCE(result_data, JSON_OBJECT()) AS result_data,
  failure_class,
  failure_message,
  created_at
FROM zatca_import_evidence
WHERE import_job_id = sqlc.arg('import_job_id')
  AND merchant_id = sqlc.arg('merchant_id')
LIMIT 1;

-- name: ListZatcaImportValidationErrors :many
SELECT
  id,
  import_job_id,
  merchant_id,
  severity,
  error_code,
  field_key,
  message,
  COALESCE(details, JSON_OBJECT()) AS details,
  created_by,
  created_at
FROM zatca_import_validation_error
WHERE import_job_id = sqlc.arg('import_job_id')
  AND merchant_id = sqlc.arg('merchant_id')
ORDER BY created_at ASC, id ASC;

-- name: ListZatcaImportReviews :many
SELECT
  id,
  import_job_id,
  merchant_id,
  decision,
  reason,
  expected_workflow_version,
  observed_workflow_version,
  created_by,
  created_at
FROM zatca_import_review
WHERE import_job_id = sqlc.arg('import_job_id')
  AND merchant_id = sqlc.arg('merchant_id')
ORDER BY created_at ASC, id ASC;

-- name: ListZatcaImportStatusHistory :many
SELECT
  id,
  import_job_id,
  merchant_id,
  from_status,
  to_status,
  transition_key,
  workflow_version,
  reason,
  created_by,
  created_at
FROM zatca_import_status_history
WHERE import_job_id = sqlc.arg('import_job_id')
  AND merchant_id = sqlc.arg('merchant_id')
ORDER BY created_at ASC, id ASC;

-- name: TransitionZatcaImportToApproved :execresult
UPDATE zatca_import_job
SET status = 'approved',
    workflow_version = workflow_version + 1,
    reviewed_by = sqlc.arg('reviewed_by'),
    reviewed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id')
  AND status = 'review'
  AND workflow_version = sqlc.arg('expected_workflow_version');

-- name: TouchZatcaImportReview :execresult
UPDATE zatca_import_job
SET workflow_version = workflow_version + 1,
    reviewed_by = sqlc.arg('reviewed_by'),
    reviewed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id')
  AND status IN ('review', 'approved')
  AND workflow_version = sqlc.arg('expected_workflow_version');

-- name: TransitionZatcaImportToPosted :execresult
UPDATE zatca_import_job
SET status = 'posted',
    workflow_version = workflow_version + 1,
    posted_by = sqlc.arg('posted_by'),
    posted_at = CURRENT_TIMESTAMP,
    post_reference = sqlc.narg('post_reference'),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id')
  AND status = 'approved'
  AND workflow_version = sqlc.arg('expected_workflow_version');

-- name: TransitionZatcaImportToRejected :execresult
UPDATE zatca_import_job
SET status = 'rejected',
    workflow_version = workflow_version + 1,
    rejected_by = sqlc.arg('rejected_by'),
    rejected_at = CURRENT_TIMESTAMP,
    rejected_reason = sqlc.arg('rejected_reason'),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id')
  AND status IN ('review', 'approved')
  AND workflow_version = sqlc.arg('expected_workflow_version');
