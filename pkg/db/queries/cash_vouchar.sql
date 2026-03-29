-- ============================================================================
-- Cash Voucher — Suggested sqlc Queries
-- File: pkg/db/queries/cash_voucher.sql
-- Date: 2026-03-26
-- ============================================================================
-- These are suggested sqlc query definitions for the backend developer.
-- Adjust column names and types to match the sqlc.yaml configuration.
-- ============================================================================

-- name: ListCashVouchers :many
SELECT
  id, voucher_number, voucher_type, effective_date, amount,
  payment_method, state, reference_type, reference_id,
  recipient_type, recipient_id, recipient_name,
  description, store_id, merchant_id, created_by, created_at
FROM cash_voucher
WHERE merchant_id = ?
  AND (CAST(? AS CHAR) = '' OR voucher_type = ?)
  AND (CAST(? AS CHAR) = '' OR (
    recipient_name LIKE CONCAT('%', ?, '%')
    OR description LIKE CONCAT('%', ?, '%')
    OR note LIKE CONCAT('%', ?, '%')
  ))
ORDER BY effective_date DESC, id DESC
LIMIT ? OFFSET ?;

-- name: GetCashVoucher :one
SELECT * FROM cash_voucher WHERE id = ? AND merchant_id = ?;

-- name: CreateCashVoucher :execresult
INSERT INTO cash_voucher (
  voucher_number, voucher_type, effective_date, amount, payment_method,
  state, reference_type, reference_id,
  recipient_type, recipient_id, recipient_name,
  description, note, bank_name, bank_account, transaction_reference,
  store_id, merchant_id, branch_id, created_by, approved_by
) VALUES (
  ?, ?, ?, ?, ?,
  0, ?, ?,
  ?, ?, ?,
  ?, ?, ?, ?, ?,
  ?, ?, ?, ?, ?
);

-- name: UpdateCashVoucher :exec
UPDATE cash_voucher SET
  voucher_type = ?,
  effective_date = ?,
  amount = ?,
  payment_method = ?,
  reference_type = ?,
  reference_id = ?,
  recipient_type = ?,
  recipient_id = ?,
  recipient_name = ?,
  description = ?,
  note = ?,
  bank_name = ?,
  bank_account = ?,
  transaction_reference = ?,
  store_id = ?
WHERE id = ? AND merchant_id = ? AND state = 0;

-- name: DeleteCashVoucher :exec
DELETE FROM cash_voucher WHERE id = ? AND merchant_id = ? AND state = 0;

-- name: ApproveCashVoucher :exec
UPDATE cash_voucher SET
  state = 1,
  approved_by = ?,
  approved_at = NOW()
WHERE id = ? AND merchant_id = ? AND state = 0;

-- name: PostCashVoucher :exec
UPDATE cash_voucher SET state = 2
WHERE id = ? AND merchant_id = ? AND state = 1;

-- name: GetNextVoucherNumber :one
SELECT COALESCE(MAX(voucher_number), 0) + 1 AS next_number
FROM cash_voucher
WHERE merchant_id = ?;

-- name: GetCashVoucherSummary :many
SELECT
  voucher_type,
  state,
  COUNT(*) AS voucher_count,
  SUM(amount) AS total_amount,
  DATE_FORMAT(effective_date, '%Y-%m') AS month
FROM cash_voucher
WHERE merchant_id = ?
GROUP BY voucher_type, state, DATE_FORMAT(effective_date, '%Y-%m')
ORDER BY month DESC;

-- name: GetTotalDisbursements :one
SELECT COALESCE(SUM(amount), 0) AS total
FROM cash_voucher
WHERE merchant_id = ? AND voucher_type = 'disbursement' AND state = 2;

-- name: GetTotalReceipts :one
SELECT COALESCE(SUM(amount), 0) AS total
FROM cash_voucher
WHERE merchant_id = ? AND voucher_type = 'receipt' AND state = 2;

