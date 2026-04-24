-- ============================================================================
-- Supplier Report (كشف حساب): sqlc Queries
-- ============================================================================
-- Add these to the sqlc query file (e.g. pkg/db/queries/supplier_report.sql)
--
-- REAL SCHEMA (from schema.sql):
--   purchase_bill (11 columns):
--     id, effective_date, payment_due_date, state, discount (BIGINT),
--     supplier_id, sequence_number, supplier_sequence_number, vat_sequence_number,
--     store_id, merchant_id
--     ⚠ NO: pdf_link, created_at, updated_at, payment_method, deliver_date, branch_id
--     ⚠ NO: total, total_before_vat, total_vat (computed via VIEW from product rows)
--
--   purchase_bill_product (11 columns):
--     id, product_id (nullable), bill_id (FK→purchase_bill.id),
--     vat DECIMAL(5,2) DEFAULT 15.00,
--     price DECIMAL(12,2), quantity DECIMAL(10,3), name,
--     type (GENERATED: 0=catalog, 1=manual),
--     total_before_vat  (GENERATED: ROUND(price * quantity, 2)),
--     vat_total          (GENERATED: ROUND(total_before_vat * vat / 100, 2)),
--     total_including_vat (GENERATED: ROUND(total_before_vat + vat_total, 2))
--
--   purchase_bill_totals (VIEW):
--     LEFT JOIN purchase_bill → purchase_bill_product, GROUP BY pb.id
--     Computes: SUM(total_before_vat), SUM(vat_total), SUM(total_including_vat)
--
--   cash_voucher:
--     id, merchant_id, amount DECIMAL(12,2), voucher_type, effective_date, state,
--     recipient_type enum('supplier','client','employee','other'),
--     recipient_id, payment_method enum('cash','bank_transfer'),
--     reference_type, reference_id, description, ...
--
--   supplier:
--     id, company_id, name, vat_number, bank_account, is_deleted, ...
--
-- TENANT ISOLATION: purchase_bill.merchant_id, cash_voucher.merchant_id
-- ============================================================================


-- ═══════════════════════════════════════════════════════════════════════════
-- 1. PURCHASE BILLS BY SUPPLIER (with computed totals)
-- ═══════════════════════════════════════════════════════════════════════════

-- name: GetPurchaseBillsBySupplier :many
-- Fetch all purchase bills for a specific supplier within a date range.
-- Totals computed from purchase_bill_product (no denormalized columns on purchase_bill).
SELECT
    pb.id,
    pb.sequence_number,
    pb.supplier_sequence_number,
    pb.supplier_id,
    pb.store_id,
    pb.state,
    pb.discount,
    pb.effective_date,
    pb.payment_due_date,
    pb.received_at,
    pb.received_by,
    -- Totals from GENERATED columns on purchase_bill_product
    -- (total_before_vat = ROUND(price*quantity,2), vat uses per-item vat rate)
    CAST(COALESCE(SUM(pbp.total_before_vat), 0) AS DECIMAL(12,2)) AS total_before_vat,
    CAST(COALESCE(SUM(pbp.vat_total), 0) AS DECIMAL(12,2)) AS total_vat,
    CAST(COALESCE(SUM(pbp.total_including_vat), 0) AS DECIMAL(12,2)) AS total
FROM purchase_bill pb
LEFT JOIN purchase_bill_product pbp ON pbp.bill_id = pb.id
WHERE pb.supplier_id = sqlc.arg('supplier_id')
  AND pb.merchant_id = sqlc.arg('merchant_id')
  AND (sqlc.narg('date_from') IS NULL OR pb.effective_date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to') IS NULL OR pb.effective_date <= sqlc.narg('date_to'))
GROUP BY pb.id
ORDER BY pb.effective_date DESC;


-- ═══════════════════════════════════════════════════════════════════════════
-- 2. SUPPLIER BILL SUMMARY (aggregate stats for header cards)
-- ═══════════════════════════════════════════════════════════════════════════

-- name: GetSupplierBillSummary :one
-- Aggregate stats for a supplier's purchase bills in a date range.
-- Uses a subquery to get per-bill totals, then aggregates.
SELECT
    CAST(COUNT(*) AS SIGNED)                                                             AS bill_count,
    CAST(COALESCE(SUM(bill_total), 0) AS DECIMAL(12,2))                                  AS total_spent,
    CAST(COALESCE(SUM(bill_total_before_vat), 0) AS DECIMAL(12,2))                       AS total_before_vat,
    CAST(COALESCE(SUM(bill_total - bill_total_before_vat), 0) AS DECIMAL(12,2))          AS total_vat,
    CAST(COALESCE(SUM(CASE WHEN state = 0 THEN bill_total ELSE 0 END), 0) AS DECIMAL(12,2))   AS unpaid_total,
    CAST(COALESCE(SUM(CASE WHEN state >= 1 THEN bill_total ELSE 0 END), 0) AS DECIMAL(12,2))  AS paid_total,
    CAST(SUM(CASE WHEN received_at IS NOT NULL THEN 1 ELSE 0 END) AS SIGNED)             AS received_count,
    CAST(COALESCE(AVG(bill_total), 0) AS DECIMAL(12,2))                                  AS avg_bill,
    CAST(COALESCE(SUM(discount), 0) AS SIGNED)                                           AS total_discount
FROM (
    SELECT
        pb.id,
        pb.state,
        pb.discount,
        pb.received_at,
        CAST(COALESCE(SUM(pbp.total_including_vat), 0) AS DECIMAL(12,2)) AS bill_total,
        CAST(COALESCE(SUM(pbp.total_before_vat), 0) AS DECIMAL(12,2))    AS bill_total_before_vat
    FROM purchase_bill pb
    LEFT JOIN purchase_bill_product pbp ON pbp.bill_id = pb.id
    WHERE pb.supplier_id = sqlc.arg('supplier_id')
      AND pb.merchant_id = sqlc.arg('merchant_id')
      AND (sqlc.narg('date_from') IS NULL OR pb.effective_date >= sqlc.narg('date_from'))
      AND (sqlc.narg('date_to') IS NULL OR pb.effective_date <= sqlc.narg('date_to'))
    GROUP BY pb.id
) AS bill_agg;


-- ═══════════════════════════════════════════════════════════════════════════
-- 3. TOP PURCHASED ITEMS FROM SUPPLIER
-- ═══════════════════════════════════════════════════════════════════════════

-- name: GetTopPurchasedItems :many
-- Top items purchased from a supplier by total value.
-- Uses purchase_bill_product (NOT bill_product — that's for sales bills).
SELECT
    COALESCE(p.name, pbp.name, 'غير معروف') AS item_name,
    CAST(SUM(pbp.quantity) AS DECIMAL(12,3))  AS total_qty,
    CAST(SUM(pbp.total_before_vat) AS DECIMAL(12,2)) AS total_value,
    CAST(AVG(pbp.price) AS DECIMAL(12,2))     AS avg_price,
    CAST(COUNT(DISTINCT pb.id) AS SIGNED)     AS bill_count
FROM purchase_bill_product pbp
JOIN purchase_bill pb ON pb.id = pbp.bill_id
LEFT JOIN product p ON p.id = pbp.product_id
WHERE pb.supplier_id = sqlc.arg('supplier_id')
  AND pb.merchant_id = sqlc.arg('merchant_id')
  AND (sqlc.narg('date_from') IS NULL OR pb.effective_date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to') IS NULL OR pb.effective_date <= sqlc.narg('date_to'))
GROUP BY COALESCE(p.name, pbp.name, 'غير معروف')
ORDER BY total_value DESC
LIMIT 20;


-- ═══════════════════════════════════════════════════════════════════════════
-- 4. CASH VOUCHER PAYMENTS TO SUPPLIER (for ledger / كشف حساب)
-- ═══════════════════════════════════════════════════════════════════════════

-- name: GetSupplierPayments :many
-- All cash vouchers (disbursements) paid to this supplier in date range.
-- Uses polymorphic FK: recipient_type='supplier' AND recipient_id=supplier.id
SELECT
    cv.id,
    cv.voucher_number,
    cv.voucher_type,
    cv.effective_date,
    cv.amount,
    cv.payment_method,
    cv.state,
    cv.reference_type,
    cv.reference_id,
    cv.description,
    cv.created_at
FROM cash_voucher cv
WHERE cv.recipient_type = 'supplier'
  AND cv.recipient_id = sqlc.arg('supplier_id')
  AND cv.merchant_id = sqlc.arg('merchant_id')
  AND cv.state >= 1  -- only approved/posted vouchers
  AND (sqlc.narg('date_from') IS NULL OR cv.effective_date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to') IS NULL OR cv.effective_date <= sqlc.narg('date_to'))
ORDER BY cv.effective_date DESC;

-- name: GetSupplierPaymentSummary :one
-- Total payments to supplier in date range, broken down by payment method.
SELECT
    CAST(COUNT(*) AS SIGNED)                                                                   AS payment_count,
    CAST(COALESCE(SUM(cv.amount), 0) AS DECIMAL(12,2))                                        AS total_payments,
    CAST(COALESCE(SUM(CASE WHEN cv.payment_method = 'cash' THEN cv.amount ELSE 0 END), 0) AS DECIMAL(12,2))          AS cash_total,
    CAST(COALESCE(SUM(CASE WHEN cv.payment_method = 'bank_transfer' THEN cv.amount ELSE 0 END), 0) AS DECIMAL(12,2)) AS bank_transfer_total
FROM cash_voucher cv
WHERE cv.recipient_type = 'supplier'
  AND cv.recipient_id = sqlc.arg('supplier_id')
  AND cv.merchant_id = sqlc.arg('merchant_id')
  AND cv.state >= 1
  AND (sqlc.narg('date_from') IS NULL OR cv.effective_date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to') IS NULL OR cv.effective_date <= sqlc.narg('date_to'));


-- ═══════════════════════════════════════════════════════════════════════════
-- 5. AGING ANALYSIS (overdue purchase bills)
-- ═══════════════════════════════════════════════════════════════════════════

-- name: GetSupplierAgingBuckets :many
-- Aging buckets for unpaid purchase bills (state=0) with a payment_due_date.
-- Returns one row per bucket: current, 1-30, 31-60, 61-90, 90+
SELECT
    CASE
        WHEN DATEDIFF(CURDATE(), pb.payment_due_date) <= 0  THEN 'current'
        WHEN DATEDIFF(CURDATE(), pb.payment_due_date) <= 30 THEN '1-30'
        WHEN DATEDIFF(CURDATE(), pb.payment_due_date) <= 60 THEN '31-60'
        WHEN DATEDIFF(CURDATE(), pb.payment_due_date) <= 90 THEN '61-90'
        ELSE '90+'
    END AS bucket,
    CAST(COUNT(*) AS SIGNED) AS bill_count,
    CAST(COALESCE(SUM(pbp_totals.bill_total), 0) AS DECIMAL(12,2)) AS bucket_total
FROM purchase_bill pb
LEFT JOIN (
    SELECT bill_id, CAST(SUM(total_including_vat) AS DECIMAL(12,2)) AS bill_total
    FROM purchase_bill_product
    GROUP BY bill_id
) pbp_totals ON pbp_totals.bill_id = pb.id
WHERE pb.supplier_id = sqlc.arg('supplier_id')
  AND pb.merchant_id = sqlc.arg('merchant_id')
  AND pb.state = 0  -- unpaid only
  AND pb.payment_due_date IS NOT NULL
GROUP BY bucket
ORDER BY FIELD(bucket, 'current', '1-30', '31-60', '61-90', '90+');


-- ═══════════════════════════════════════════════════════════════════════════
-- 6. MONTHLY SPENDING TREND
-- ═══════════════════════════════════════════════════════════════════════════

-- name: GetSupplierMonthlySpending :many
-- Monthly spending with this supplier over the date range.
SELECT
    DATE_FORMAT(pb.effective_date, '%Y-%m') AS month,
    CAST(COUNT(*) AS SIGNED)                                AS bill_count,
    CAST(COALESCE(SUM(pbp_totals.bill_total), 0) AS DECIMAL(12,2)) AS total_spent
FROM purchase_bill pb
LEFT JOIN (
    SELECT bill_id, CAST(SUM(total_including_vat) AS DECIMAL(12,2)) AS bill_total
    FROM purchase_bill_product
    GROUP BY bill_id
) pbp_totals ON pbp_totals.bill_id = pb.id
WHERE pb.supplier_id = sqlc.arg('supplier_id')
  AND pb.merchant_id = sqlc.arg('merchant_id')
  AND (sqlc.narg('date_from') IS NULL OR pb.effective_date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to') IS NULL OR pb.effective_date <= sqlc.narg('date_to'))
GROUP BY month
ORDER BY month ASC;


-- ═══════════════════════════════════════════════════════════════════════════
-- 7. RECEIPT TRACKING (mark/unmark bill as received)
-- ═══════════════════════════════════════════════════════════════════════════

-- name: MarkPurchaseBillReceived :exec
-- Mark a purchase bill as received (goods confirmed delivered).
UPDATE purchase_bill
SET received_at = NOW(),
    received_by = sqlc.arg('received_by')
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id');

-- name: UnmarkPurchaseBillReceived :exec
-- Clear receipt status from a purchase bill.
UPDATE purchase_bill
SET received_at = NULL,
    received_by = NULL
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id');

