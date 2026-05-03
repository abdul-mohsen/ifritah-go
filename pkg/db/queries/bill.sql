-- name: CreateBill :execresult
  insert into bill (effective_date, payment_due_date, state, discount, store_id, sequence_number, merchant_id, maintenance_cost, note, userName, client_id, user_phone_number, payment_method, branch_id, deliver_date)
  values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAllBill :many
-- Keyset / cursor pagination. Sort key: (effective_date DESC, id DESC).
-- The (cursor_date, cursor_id) sentinels are NULL on the first page and
-- non-NULL on subsequent pages; the OR-tree is the canonical "seek
-- predicate" form (Markus Winand, use-the-index-luke.com/no-offset).
-- Backed by `idx_bill_keyset(effective_date DESC, id DESC)` from
-- migration 0003 so the optimizer serves this as a single range scan.
--
-- The earlier UNION is gone — it was duplicating every bill that had a
-- credit_note row and silently emitting two ids per row. A LEFT JOIN
-- gives the same result (cn.state when present, NULL otherwise) without
-- the duplicate.
--
-- Caller fetches `limit + 1` to detect has_more without a COUNT(*).
SELECT bill.id AS id,
       effective_date,
       payment_due_date,
       bill.state AS state,
       discount,
       sequence_number,
       bill.user_phone_number,
       client.id IS NOT NULL AS bill_type,
       cn.state AS credit_state,
       total,
       total_vat,
       total_before_vat
FROM bill
LEFT JOIN client ON client.id = bill.client_id
LEFT JOIN credit_note cn ON cn.bill_id = bill.id
WHERE bill.state >= 0
  AND (sqlc.narg('phonenumber') IS NULL OR bill.user_phone_number LIKE sqlc.narg('phonenumber'))
  AND (
        sqlc.narg('cursor_date') IS NULL
     OR bill.effective_date < sqlc.narg('cursor_date')
     OR (bill.effective_date = sqlc.narg('cursor_date') AND bill.id < sqlc.narg('cursor_id'))
  )
ORDER BY bill.effective_date DESC, bill.id DESC
LIMIT ?;


-- name: GetBillByID :one
SELECT
CONCAT('https://ifritah.com/bill_pdf/', b.id) AS url,
effective_date,
payment_due_date,
b.state as state,
b.discount,
b.store_id,
sequence_number,
merchant_id,
maintenance_cost,
b.note,
b.userName as userName,
user_phone_number,
qr_code,
total_before_vat,
total_vat,
total,
company.name as company_name,
company.vat_registration_number,
company.commercial_registration_number,
store.address_name,
store.name as store_name,
COALESCE(
  (SELECT JSON_ARRAYAGG(
	  JSON_OBJECT(
		'product_id', p.product_id,
		'price', p.price,
		'quantity', p.quantity
	  )
	)
	FROM bill_product p
	WHERE p.bill_id = b.id),
  JSON_ARRAY()) AS products
FROM bill b
JOIN store on store.id = b.store_id
JOIN company on company.id = store.company_id
WHERE b.id = ? LIMIT 1 ;


-- name: GetBillPDFByID :one
SELECT b.*,
company.name as company_name,
company.vat_registration_number,
company.commercial_registration_number,
store.address_name,
store.name as store_name,
cn.state as credit_state,
cn.note as credit_note,
cn.id as credit_id
FROM bill b
JOIN store on store.id = b.store_id
JOIN company on company.id = store.company_id
LEFT JOIN credit_note cn on cn.bill_id = b.id
WHERE b.id = ? LIMIT 1 ;

-- name: UpdateBillByID :exec
UPDATE bill SET
effective_date = ?,
payment_due_date = ?,
state = ?,
discount = ?,
store_id = ?,
sequence_number = ?,
merchant_id = ?,
maintenance_cost = ?,
note = ?,
userName = ?,
client_id = ?,
user_phone_number = ?,
payment_method = ?,
branch_id = ?,
deliver_date = ?
WHERE id = ?;

-- name: AddProductToBill :exec
insert into bill_product (name, product_id, price, quantity, bill_id, part_name) values (?, ?, ?, ?, ?, ?);

-- name: DeleteProductToBill :exec
DELETE FROM bill_product where bill_id = ?;

-- name: GetBillProductByBillID :many
select * from bill_product where bill_id = ?;

-- name: GetStoreIDAndSequenceNumberFromBill :one
SELECT store_id, sequence_number FROM bill WHERE id = ? limit 1;


-- name: GetProductOfBill :many
SELECT bp.id, bp.product_id, bp.quantity, b.store_id, b.sequence_number
		 FROM bill_product bp
		 JOIN bill b ON bp.bill_id = b.id
		 WHERE bp.bill_id = ? AND bp.product_id IS NOT NULL;

-- name: GetMaxSequenceNumber :one
select CAST(COALESCE(max(sequence_number), 1) AS UNSIGNED) from bill
