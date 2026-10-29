-- name: GetZatcaMonitorStats :one
SELECT
  COUNT(*) AS total,
  COUNT(CASE WHEN COALESCE(NULLIF(latest_outcome.outcome_state, ''), NULLIF(zs.outcome_state, ''), 'pending') = 'accepted' THEN 1 END) AS accepted,
  COUNT(CASE WHEN COALESCE(NULLIF(latest_outcome.outcome_state, ''), NULLIF(zs.outcome_state, ''), 'pending') = 'accepted_with_warning' THEN 1 END) AS accepted_with_warning,
  COUNT(CASE WHEN COALESCE(NULLIF(latest_outcome.outcome_state, ''), NULLIF(zs.outcome_state, ''), 'pending') = 'rejected' THEN 1 END) AS rejected,
  COUNT(CASE WHEN COALESCE(NULLIF(latest_outcome.outcome_state, ''), NULLIF(zs.outcome_state, ''), 'pending') = 'ambiguous' THEN 1 END) AS ambiguous,
  COUNT(CASE WHEN COALESCE(NULLIF(latest_outcome.outcome_state, ''), NULLIF(zs.outcome_state, ''), 'pending') = 'terminal_failure' THEN 1 END) AS terminal_failure,
  COUNT(CASE WHEN COALESCE(NULLIF(latest_outcome.outcome_state, ''), NULLIF(zs.outcome_state, ''), 'pending') = 'pending' THEN 1 END) AS pending
FROM zatca_submission zs
JOIN bill bl ON bl.id = zs.bill_id
LEFT JOIN zatca_submission_outcome latest_outcome
  ON latest_outcome.id = (
    SELECT so.id
    FROM zatca_submission_outcome so
    WHERE so.submission_id = zs.id
    ORDER BY so.observed_at DESC, so.id DESC
    LIMIT 1
  )
WHERE bl.merchant_id = sqlc.arg('merchant_id')
  AND zs.merchant_id = sqlc.arg('merchant_id');

-- name: ListZatcaMonitorBranches :many
SELECT
  b.id AS branch_id,
  b.name AS branch_name,
  bzc.zatca_status AS zatca_status,
  bzc.zatca_compliance_certificate AS certificate,
  (SELECT COUNT(*)
     FROM zatca_submission zs
     JOIN bill bl ON bl.id = zs.bill_id
     WHERE bl.branch_id = b.id
       AND bl.merchant_id = sqlc.arg('merchant_id')
       AND zs.merchant_id = sqlc.arg('merchant_id')
       AND DATE(zs.submitted_at) = CURDATE()) AS today_count,
  (SELECT COUNT(*)
     FROM zatca_submission zs
     JOIN bill bl ON bl.id = zs.bill_id
     WHERE bl.branch_id = b.id
       AND bl.merchant_id = sqlc.arg('merchant_id')
       AND zs.merchant_id = sqlc.arg('merchant_id')
       AND zs.submitted_at >= NOW() - INTERVAL 7 DAY) AS week_total,
  (SELECT COUNT(*)
     FROM zatca_submission zs
     JOIN bill bl ON bl.id = zs.bill_id
     LEFT JOIN zatca_submission_outcome latest_outcome
       ON latest_outcome.id = (
         SELECT so.id
         FROM zatca_submission_outcome so
         WHERE so.submission_id = zs.id
         ORDER BY so.observed_at DESC, so.id DESC
         LIMIT 1
       )
     WHERE bl.branch_id = b.id
       AND bl.merchant_id = sqlc.arg('merchant_id')
       AND zs.merchant_id = sqlc.arg('merchant_id')
       AND zs.submitted_at >= NOW() - INTERVAL 7 DAY
       AND COALESCE(NULLIF(latest_outcome.outcome_state, ''), NULLIF(zs.outcome_state, ''), 'pending')
           IN ('accepted', 'accepted_with_warning')) AS week_accepted,
  (SELECT MAX(zs.submitted_at)
     FROM zatca_submission zs
     JOIN bill bl ON bl.id = zs.bill_id
     WHERE bl.branch_id = b.id
       AND bl.merchant_id = sqlc.arg('merchant_id')
       AND zs.merchant_id = sqlc.arg('merchant_id')) AS last_submission_at
FROM branches b
JOIN branch_zatca_config bzc ON bzc.branch_id = b.id
WHERE EXISTS (
  SELECT 1
  FROM bill tenant_bill
  WHERE tenant_bill.branch_id = b.id
    AND tenant_bill.merchant_id = sqlc.arg('merchant_id')
)
ORDER BY b.id ASC;

-- name: ListZatcaMonitorSubmissions :many
-- Sentinel filters keep the SQL static for code-scanners (Sonar S2077):
--   pending_flag = 'y' includes explicit pending state.
--   state_filter = '' disables the exact explicit-state match.
--   status_code  = -1 disables the legacy numeric-status match.
--   branch_id    = 0 disables the branch filter.
SELECT
  bl.id AS invoice_id,
  bl.sequence_number,
  bl.branch_id,
  COALESCE(b.name, '') AS branch_name,
  zs.document_id,
  COALESCE(latest_outcome.outcome_state, zs.outcome_state, 'pending') AS outcome_state,
  COALESCE(latest_outcome.provider_code, zs.provider_code) AS provider_code,
  COALESCE(latest_outcome.provider_warning_code, zs.provider_warning_code) AS provider_warning_code,
  COALESCE(latest_outcome.provider_warning_message, zs.provider_warning_message) AS provider_warning_message,
  COALESCE(latest_outcome.provider_reference, zs.provider_reference) AS provider_reference,
  zs.status AS legacy_status,
  zs.rejection_code,
  zs.rejection_msg,
  zs.submitted_at,
  zs.retry_count,
  zs.last_attempt_at,
  zs.next_retry_at,
  zs.request_xml_path,
  zs.cleared_xml_path,
  zs.outcome_at
FROM zatca_submission zs
JOIN bill bl ON bl.id = zs.bill_id
LEFT JOIN branches b ON b.id = bl.branch_id
LEFT JOIN zatca_submission_outcome latest_outcome
  ON latest_outcome.id = (
    SELECT so.id
    FROM zatca_submission_outcome so
    WHERE so.submission_id = zs.id
    ORDER BY so.observed_at DESC, so.id DESC
    LIMIT 1
  )
WHERE bl.merchant_id = sqlc.arg('merchant_id')
  AND zs.merchant_id = sqlc.arg('merchant_id')
  AND (sqlc.arg('pending_flag') = ''
       OR COALESCE(NULLIF(latest_outcome.outcome_state, ''), NULLIF(zs.outcome_state, ''), 'pending') = 'pending')
  AND (sqlc.arg('state_filter') = ''
       OR COALESCE(latest_outcome.outcome_state, zs.outcome_state, 'pending')
          = sqlc.arg('state_filter'))
  AND (sqlc.arg('status_code') = -1 OR zs.status = sqlc.arg('status_code'))
  AND (sqlc.arg('branch_id') = 0 OR bl.branch_id = sqlc.arg('branch_id'))
ORDER BY COALESCE(zs.outcome_at, zs.submitted_at, zs.updated_at) DESC, zs.id DESC
LIMIT ?;

-- name: ListZatcaSubmissionAttempts :many
SELECT
  a.id,
  a.submission_id,
  a.attempt_number,
  a.attempt_state,
  a.provider_code,
  a.provider_warning_code,
  a.provider_warning_message,
  a.provider_reference,
  a.error_code,
  a.error_message,
  a.retry_count,
  a.next_retry_at,
  a.request_xml_path,
  a.cleared_xml_path,
  a.started_at,
  a.completed_at,
  a.created_at,
  a.updated_at
FROM zatca_submission_attempt a
JOIN zatca_submission zs ON zs.id = a.submission_id
JOIN bill bl ON bl.id = zs.bill_id
WHERE bl.merchant_id = sqlc.arg('merchant_id')
  AND zs.merchant_id = sqlc.arg('merchant_id')
  AND zs.id = sqlc.arg('submission_id')
ORDER BY a.attempt_number ASC;

-- name: ListZatcaSubmissionArtifacts :many
SELECT
  za.id,
  za.submission_id,
  za.attempt_id,
  za.artifact_kind,
  za.artifact_path,
  za.content_type,
  za.sha256,
  za.metadata,
  za.created_at
FROM zatca_submission_artifact za
JOIN zatca_submission zs ON zs.id = za.submission_id
JOIN bill bl ON bl.id = zs.bill_id
WHERE bl.merchant_id = sqlc.arg('merchant_id')
  AND zs.merchant_id = sqlc.arg('merchant_id')
  AND zs.id = sqlc.arg('submission_id')
ORDER BY za.created_at ASC, za.id ASC;
