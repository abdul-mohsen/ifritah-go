-- ============================================================================
-- sqlc queries for stock_movements table
-- ============================================================================
-- Copy this file to: pkg/db/queries/stock.sql
-- Then run: sqlc generate
-- ============================================================================


-- name: InsertStockMovement :execresult
INSERT INTO stock_movements (
  product_id, store_id, quantity, movement_type,
  reference_type, reference_id, item_id,
  reason, note, created_by, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);


-- name: GetStockMovementsByProduct :many
SELECT
  id, product_id, store_id, quantity, movement_type,
  reference_type, reference_id, item_id,
  reason, note, created_by, created_at
FROM stock_movements
WHERE product_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;


-- name: GetStockMovementsByStore :many
SELECT
  id, product_id, store_id, quantity, movement_type,
  reference_type, reference_id, item_id,
  reason, note, created_by, created_at
FROM stock_movements
WHERE store_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;


-- name: GetStockMovementsByReference :many
SELECT
  id, product_id, store_id, quantity, movement_type,
  reference_type, reference_id, item_id,
  reason, note, created_by, created_at
FROM stock_movements
WHERE reference_type = ? AND reference_id = ?
ORDER BY created_at DESC;


-- name: GetStockLedgerSum :one
SELECT COALESCE(SUM(quantity), 0) AS total
FROM stock_movements
WHERE product_id = ?;


-- name: CountStockMovementsByProduct :one
SELECT COUNT(*) AS cnt
FROM stock_movements
WHERE product_id = ?;


-- name: GetProductQuantity :one
SELECT id, store_id, quantity, is_deleted
FROM product
WHERE id = ? AND is_deleted = 0;


-- name: UpdateProductQuantity :exec
UPDATE product
SET quantity = quantity + ?
WHERE id = ?;


-- name: GetStockEnforcement :one
SELECT COALESCE(value, 'disable') AS value
FROM settings
WHERE setting_key = 'stock_enforcement';


-- name: GetBillProductsByBillIDForStock :many
SELECT bp.id, bp.product_id, bp.quantity, b.store_id, b.id AS bill_id, b.sequence_number
FROM bill_product bp
JOIN bill b ON bp.bill_id = b.id
WHERE bp.bill_id = ? AND bp.product_id IS NOT NULL;


-- name: GetPurchaseBillProductsByBillIDForStock :many
SELECT pbp.id, pbp.product_id, pbp.quantity, pb.store_id, pb.id AS bill_id, pb.sequence_number
FROM purchase_bill_product pbp
JOIN purchase_bill pb ON pbp.bill_id = pb.id
WHERE pbp.bill_id = ? AND pbp.product_id IS NOT NULL;


-- name: GetCreditNoteByID :one
SELECT cn.id, cn.bill_id, cn.state, b.store_id, b.sequence_number
FROM credit_note cn
JOIN bill b ON cn.bill_id = b.id
WHERE cn.id = ?;


-- name: GetBillProductsForCreditNote :many
SELECT bp.id, bp.product_id, bp.quantity
FROM bill_product bp
WHERE bp.bill_id = ? AND bp.product_id IS NOT NULL AND bp.quantity > 0;

