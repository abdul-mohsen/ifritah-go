-- name: GetZatcaMonitorStats :one
SELECT
  COUNT(*)                                     AS total,
  COUNT(CASE WHEN status = 2 THEN 1 END)       AS accepted,
  COUNT(CASE WHEN status = 4 THEN 1 END)       AS warnings,
  COUNT(CASE WHEN status = 3 THEN 1 END)       AS rejected,
  COUNT(CASE WHEN status IN (0, 1) THEN 1 END) AS pending
FROM zatca_submission;

-- name: ListZatcaMonitorBranches :many
SELECT
  b.id                                                              AS branch_id,
  b.name                                                            AS branch_name,
  bzc.zatca_status                                                  AS zatca_status,
  bzc.zatca_compliance_certificate                                  AS certificate,
  (SELECT COUNT(*)
     FROM zatca_submission zs
     JOIN bill bl ON bl.id = zs.bill_id
     WHERE bl.branch_id = b.id
       AND DATE(zs.submitted_at) = CURDATE())                       AS today_count,
  (SELECT COUNT(*)
     FROM zatca_submission zs
     JOIN bill bl ON bl.id = zs.bill_id
     WHERE bl.branch_id = b.id
       AND zs.submitted_at >= NOW() - INTERVAL 7 DAY)               AS week_total,
  (SELECT COUNT(*)
     FROM zatca_submission zs
     JOIN bill bl ON bl.id = zs.bill_id
     WHERE bl.branch_id = b.id
       AND zs.submitted_at >= NOW() - INTERVAL 7 DAY
       AND zs.status = 2)                                           AS week_accepted,
  (SELECT MAX(zs.submitted_at)
     FROM zatca_submission zs
     JOIN bill bl ON bl.id = zs.bill_id
     WHERE bl.branch_id = b.id)                                     AS last_submission_at
FROM branches b
JOIN branch_zatca_config bzc ON bzc.branch_id = b.id
ORDER BY b.id ASC;

-- name: ListZatcaMonitorSubmissions :many
-- Sentinel filters keep the SQL static for code-scanners (Sonar S2077):
--   pending_flag = 'y' includes status IN (0,1); otherwise no pending bias.
--   status_code  = -1  disables the exact-status match.
--   branch_id    = 0   disables the branch filter.
SELECT
  bl.id            AS invoice_id,
  bl.sequence_number,
  bl.branch_id,
  COALESCE(b.name, '')  AS branch_name,
  zs.status,
  zs.rejection_code,
  zs.rejection_msg,
  zs.submitted_at
FROM zatca_submission zs
JOIN bill bl     ON bl.id = zs.bill_id
LEFT JOIN branches b ON b.id = bl.branch_id
WHERE (sqlc.arg('pending_flag') = '' OR zs.status IN (0,1))
  AND (sqlc.arg('status_code') = -1 OR zs.status = sqlc.arg('status_code'))
  AND (sqlc.arg('branch_id') = 0 OR bl.branch_id = sqlc.arg('branch_id'))
ORDER BY zs.submitted_at DESC
LIMIT ?;
