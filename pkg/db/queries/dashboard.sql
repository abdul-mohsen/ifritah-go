-- queries/dashboard.sql
-- Drop into pkg/db/queries/dashboard.sql in the ifritah-go backend.
--
-- Single-tenant assumption: each MySQL database hosts exactly one company.
-- Tenancy is enforced at the connection / database level, NOT in WHERE clauses.
-- That's why none of these queries take a company_id / merchant_id argument.
--
-- Verified against schema.sql 2026-04-24.
--   * Reads cached bill.total / bill.total_before_vat / bill.total_vat from
--     migration 002_cached_totals.sql.
--   * Joins through bill for credit-note totals so this works whether or not
--     migration 008_partial_credit_notes.sql has been applied.
--
-- Param naming: nullable filters use sqlc.narg('foo'); reused params use
-- sqlc.arg('foo') so the generated method has a clean field name.

-- ── 1. counts (one row, no params) ─────────────────────────────────

-- name: GetDashboardCounts :one
SELECT
    (SELECT COUNT(*) FROM product p
        WHERE p.is_deleted = 0)                          AS total_products,
    (SELECT COUNT(*) FROM client cl
        WHERE cl.is_deleted = 0)                         AS total_clients,
    (SELECT COUNT(*) FROM supplier sp
        WHERE sp.is_deleted = 0)                         AS total_suppliers,
    (SELECT COUNT(*) FROM store)                         AS total_stores,
    (SELECT COUNT(*) FROM branches)                      AS total_branches,
    (SELECT COUNT(*) FROM product p
        WHERE p.is_deleted = 0
          AND p.quantity <= p.min_stock)                 AS low_stock_count,
    (SELECT COALESCE(SUM(p.price * p.quantity), 0)
       FROM product p
        WHERE p.is_deleted = 0)                          AS inventory_value,
    (SELECT COUNT(DISTINCT b.client_id) FROM bill b
        LEFT JOIN credit_note cn ON cn.bill_id = b.id
        WHERE b.client_id IS NOT NULL AND cn.id IS NULL) AS active_clients,
    (SELECT COUNT(*) FROM bill b
        LEFT JOIN credit_note cn ON cn.bill_id = b.id
        WHERE b.state = 3 AND cn.id IS NULL)             AS issued_invoices;

-- ── 2. sales KPIs (revenue / VAT / discount / status counts) ───────

-- name: GetDashboardSalesKPIs :one
SELECT
    COALESCE(SUM(b.total), 0)                                            AS total_revenue,
    COALESCE(SUM(b.total_vat), 0)                                        AS total_vat,
    COALESCE(SUM(b.discount), 0)                                         AS total_discount,
    COUNT(CASE WHEN b.state IN (0,1) THEN 1 END)                         AS pending_count,
    COALESCE(SUM(CASE WHEN b.state IN (0,1) THEN b.total ELSE 0 END), 0) AS pending_amount,
    COUNT(CASE WHEN b.state = 0 THEN 1 END)                              AS draft_count,
    COUNT(CASE WHEN b.state = 1 THEN 1 END)                              AS processing_count,
    COUNT(CASE WHEN b.state = 2 THEN 1 END)                              AS processed_count,
    COUNT(CASE WHEN b.state = 3 THEN 1 END)                              AS issued_count
FROM bill b
LEFT JOIN credit_note cn ON cn.bill_id = b.id
WHERE cn.id IS NULL
  AND (sqlc.narg('state')      IS NULL OR b.state = sqlc.narg('state'))
  AND (sqlc.narg('start_date') IS NULL OR b.effective_date >= sqlc.narg('start_date'))
  AND (sqlc.narg('end_date')   IS NULL OR b.effective_date <= sqlc.narg('end_date'));

-- ── 3. credit-note KPI ─────────────────────────────────────────────
--
-- Joins through bill so this works regardless of whether migration 008
-- has been applied.

-- name: GetDashboardCreditNoteStats :one
SELECT
    COUNT(*)                       AS credit_note_count,
    COALESCE(SUM(b.total), 0)      AS credit_note_total
FROM credit_note cn
INNER JOIN bill b ON b.id = cn.bill_id
WHERE cn.state = 1
  AND (sqlc.narg('start_date') IS NULL OR b.effective_date >= sqlc.narg('start_date'))
  AND (sqlc.narg('end_date')   IS NULL OR b.effective_date <= sqlc.narg('end_date'));

-- ── 4. purchase KPIs ───────────────────────────────────────────────

-- name: GetDashboardPurchaseKPIs :one
SELECT
    COUNT(*)                           AS purchase_count,
    COALESCE(SUM(pb.total), 0)         AS purchases_total,
    COALESCE(SUM(pb.total_vat), 0)     AS purchases_vat
FROM purchase_bill pb
WHERE (sqlc.narg('start_date') IS NULL OR pb.effective_date >= sqlc.narg('start_date'))
  AND (sqlc.narg('end_date')   IS NULL OR pb.effective_date <= sqlc.narg('end_date'));

-- ── 5. monthly trailing series ─────────────────────────────────────

-- name: GetMonthlyRevenue :many
SELECT DATE_FORMAT(b.effective_date, '%m/%Y') AS month_key,
       COALESCE(SUM(b.total), 0)              AS revenue
FROM bill b
LEFT JOIN credit_note cn ON cn.bill_id = b.id
WHERE cn.id IS NULL
  AND b.effective_date >= DATE_SUB(NOW(), INTERVAL sqlc.arg('months') MONTH)
GROUP BY month_key;

-- name: GetMonthlyPurchases :many
SELECT DATE_FORMAT(pb.effective_date, '%m/%Y') AS month_key,
       COALESCE(SUM(pb.total), 0)              AS purchases
FROM purchase_bill pb
WHERE pb.effective_date >= DATE_SUB(NOW(), INTERVAL sqlc.arg('months') MONTH)
GROUP BY month_key;

-- ── 6. recent invoices ─────────────────────────────────────────────

-- name: GetDashboardRecentInvoices :many
SELECT b.id, b.sequence_number, COALESCE(b.total, 0) AS total, b.state,
       DATE_FORMAT(b.effective_date, '%Y-%m-%d')     AS effective_date,
       (cn.id IS NOT NULL)                           AS is_credit_note,
       b.user_phone_number
FROM bill b
LEFT JOIN credit_note cn ON cn.bill_id = b.id
ORDER BY b.effective_date DESC, b.id DESC
LIMIT 10;

-- ── 7. low-stock + top-stock products ──────────────────────────────

-- name: GetDashboardLowStockProducts :many
SELECT p.id,
       COALESCE(a.articleNumber, CAST(p.article_id AS CHAR)) AS article_number,
       p.price, p.quantity, p.cost_price, p.min_stock, p.store_id
FROM product p
LEFT JOIN articles a ON a.legacyArticleId = p.article_id
WHERE p.is_deleted = 0
  AND p.quantity <= p.min_stock
ORDER BY p.quantity ASC, p.id ASC
LIMIT 10;

-- name: GetDashboardTopStockProducts :many
SELECT p.id,
       COALESCE(a.articleNumber, CAST(p.article_id AS CHAR)) AS article_number,
       p.quantity, p.price
FROM product p
LEFT JOIN articles a ON a.legacyArticleId = p.article_id
WHERE p.is_deleted = 0
ORDER BY p.quantity DESC, p.id DESC
LIMIT 8;

-- ── 8. price tiers ─────────────────────────────────────────────────

-- name: GetDashboardPriceTiers :many
SELECT
    CASE
        WHEN p.price <  50  THEN 0
        WHEN p.price <  200 THEN 1
        WHEN p.price <  500 THEN 2
        ELSE 3
    END                            AS tier,
    COUNT(*)                       AS product_count,
    COALESCE(AVG(p.price), 0)      AS avg_price
FROM product p
WHERE p.is_deleted = 0
GROUP BY tier;

-- ── 9. supplier performance (top 5 by spend) ───────────────────────

-- name: GetDashboardSupplierPerformance :many
SELECT s.id, s.name,
       COUNT(pb.id)                       AS bill_count,
       COALESCE(SUM(pb.total), 0)         AS total_spent,
       COALESCE(AVG(pb.total), 0)         AS avg_total
FROM supplier s
LEFT JOIN purchase_bill pb ON pb.supplier_id = s.id
WHERE s.is_deleted = 0
GROUP BY s.id, s.name
ORDER BY total_spent DESC
LIMIT 5;

-- ── 10. monthly returns ────────────────────────────────────────────

-- name: GetDashboardMonthlyReturns :many
SELECT DATE_FORMAT(b.effective_date, '%m/%Y') AS month_key,
       COUNT(CASE WHEN cn.id IS NULL     THEN 1 END) AS invoice_count,
       COUNT(CASE WHEN cn.id IS NOT NULL THEN 1 END) AS credit_note_count
FROM bill b
LEFT JOIN credit_note cn ON cn.bill_id = b.id
WHERE b.effective_date >= DATE_SUB(NOW(), INTERVAL sqlc.arg('months') MONTH)
GROUP BY month_key
ORDER BY month_key;

-- ── 11. weekday revenue ────────────────────────────────────────────

-- name: GetDashboardWeekdayRevenue :many
SELECT DAYOFWEEK(b.effective_date)         AS dow,
       COALESCE(AVG(b.total), 0)           AS avg_revenue
FROM bill b
LEFT JOIN credit_note cn ON cn.bill_id = b.id
WHERE cn.id IS NULL
  AND b.effective_date >= DATE_SUB(NOW(), INTERVAL sqlc.arg('months') MONTH)
GROUP BY dow;

-- ── 12. YoY revenue (12-month-shifted window for trailing N months) ──

-- name: GetDashboardYoYRevenue :many
SELECT DATE_FORMAT(b.effective_date, '%m/%Y') AS month_key,
       COALESCE(SUM(b.total), 0)              AS revenue
FROM bill b
LEFT JOIN credit_note cn ON cn.bill_id = b.id
WHERE cn.id IS NULL
  AND b.effective_date >= DATE_SUB(sqlc.arg('anchor'), INTERVAL sqlc.arg('months_back_start') MONTH)
  AND b.effective_date <  DATE_SUB(sqlc.arg('anchor'), INTERVAL sqlc.arg('months_back_end')   MONTH)
GROUP BY month_key;

-- ── 13. top clients ────────────────────────────────────────────────

-- name: GetDashboardTopClients :many
SELECT c.id, c.name,
       COUNT(b.id)                                                  AS invoice_count,
       COALESCE(SUM(b.total), 0)                                    AS total_value,
       COALESCE(MAX(DATE_FORMAT(b.effective_date, '%Y-%m-%d')), '-') AS last_invoice_date
FROM client c
INNER JOIN bill b      ON b.client_id = c.id
LEFT JOIN credit_note cn ON cn.bill_id = b.id
WHERE cn.id IS NULL AND c.is_deleted = 0
GROUP BY c.id, c.name
ORDER BY total_value DESC
LIMIT 5;

-- ── 14. AR / AP aging ──────────────────────────────────────────────

-- name: GetDashboardARAging :many
SELECT
    CASE
        WHEN DATEDIFF(NOW(), b.effective_date) <= 30 THEN 0
        WHEN DATEDIFF(NOW(), b.effective_date) <= 60 THEN 1
        WHEN DATEDIFF(NOW(), b.effective_date) <= 90 THEN 2
        ELSE 3
    END                                 AS bucket,
    COUNT(*)                            AS bill_count,
    COALESCE(SUM(b.total), 0)           AS total
FROM bill b
LEFT JOIN credit_note cn ON cn.bill_id = b.id
WHERE b.state IN (0,1) AND cn.id IS NULL
GROUP BY bucket;

-- name: GetDashboardAPAging :many
SELECT
    CASE
        WHEN DATEDIFF(NOW(), pb.effective_date) <= 30 THEN 0
        WHEN DATEDIFF(NOW(), pb.effective_date) <= 60 THEN 1
        WHEN DATEDIFF(NOW(), pb.effective_date) <= 90 THEN 2
        ELSE 3
    END                                  AS bucket,
    COUNT(*)                             AS bill_count,
    COALESCE(SUM(pb.total), 0)           AS total
FROM purchase_bill pb
WHERE pb.state != 3
GROUP BY bucket;

-- ── 15. period roll-up (analytics KPI trend + compare endpoint) ───

-- name: GetDashboardPeriodInvoices :one
SELECT
    COUNT(*)                                AS invoice_count,
    COALESCE(SUM(b.total), 0)               AS revenue,
    COALESCE(SUM(CASE WHEN b.state IN (0,1) THEN b.total ELSE 0 END), 0) AS pending_amount,
    COUNT(CASE WHEN b.state = 3 THEN 1 END) AS issued_count,
    COUNT(CASE WHEN b.state = 0 THEN 1 END) AS draft_count
FROM bill b
LEFT JOIN credit_note cn ON cn.bill_id = b.id
WHERE cn.id IS NULL
  AND DATE(b.effective_date) >= sqlc.arg('start_date')
  AND DATE(b.effective_date) <= sqlc.arg('end_date');

-- name: GetDashboardPeriodPurchases :one
SELECT COALESCE(SUM(pb.total), 0) AS purchases_total
FROM purchase_bill pb
WHERE DATE(pb.effective_date) >= sqlc.arg('start_date')
  AND DATE(pb.effective_date) <= sqlc.arg('end_date');


-- ── 16. orders summary (pending count + totals) ──────────────────

-- name: GetDashboardOrderStats :one
SELECT
    COUNT(*)                                                         AS total_orders,
    COUNT(CASE WHEN o.status IN ('pending','processing') THEN 1 END) AS pending_orders,
    COUNT(CASE WHEN o.status = 'completed' THEN 1 END)               AS completed_orders,
    COUNT(CASE WHEN o.status = 'cancelled' THEN 1 END)               AS cancelled_orders,
    COALESCE(SUM(o.total), 0)                                        AS total_orders_amount
FROM orders o;

-- ── 17. average order processing time (days) ─────────────────────
-- Approximates days between order creation and the first issued bill
-- for the same client. NULL when no completed orders have a matching bill.

-- name: GetDashboardAvgOrderProcessing :one
SELECT COALESCE(AVG(diff_days), 0) AS avg_days
FROM (
    SELECT TIMESTAMPDIFF(DAY, o.created_at,
        (SELECT MIN(b.effective_date)
           FROM bill b
           WHERE b.client_id = o.client_id
             AND b.state = 3
             AND b.effective_date >= o.created_at)
    ) AS diff_days
    FROM orders o
    WHERE o.status = 'completed'
      AND o.client_id IS NOT NULL
) t
WHERE diff_days IS NOT NULL;

-- ── 18. customer lifetime value (top 20 by total order spend) ────

-- name: GetDashboardOrdersCLV :many
SELECT
    COALESCE(NULLIF(c.name, ''), NULLIF(o.customer_name, ''), 'غير معروف') AS client_name,
    COALESCE(o.client_id, 0)   AS client_id,
    COUNT(*)                   AS order_count,
    COALESCE(SUM(o.total), 0)  AS total_value
FROM orders o
LEFT JOIN client c ON c.id = o.client_id
WHERE o.status IN ('completed','processing','pending')
GROUP BY client_name, o.client_id
ORDER BY total_value DESC
LIMIT 20;

-- ── 19. quarterly VAT (output - input) ───────────────────────────

-- name: GetDashboardVATQuarterly :many
SELECT
    YEAR(d.dt)                AS year,
    QUARTER(d.dt)             AS quarter,
    COALESCE(SUM(d.output_vat), 0) AS output_vat,
    COALESCE(SUM(d.input_vat), 0)  AS input_vat
FROM (
    SELECT b.effective_date AS dt, b.total_vat AS output_vat, 0.0 AS input_vat
        FROM bill b
        LEFT JOIN credit_note cn ON cn.bill_id = b.id
        WHERE cn.id IS NULL AND b.state = 3
    UNION ALL
    SELECT pb.effective_date AS dt, 0.0 AS output_vat, pb.total_vat AS input_vat
        FROM purchase_bill pb
) d
GROUP BY YEAR(d.dt), QUARTER(d.dt)
ORDER BY YEAR(d.dt), QUARTER(d.dt);
