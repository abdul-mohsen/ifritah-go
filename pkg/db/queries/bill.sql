-- name: CreateBill :execresult
  insert into bill (effective_date, payment_due_date, state, discount, store_id, sequence_number, merchant_id, maintenance_cost, note, userName, client_id, user_phone_number, payment_method, branch_id, deliver_date)
  values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAllBill :many
SELECT bill.id AS id,
       bill.effective_date AS effective_date,
       payment_due_date,
       bill.state AS state,
       discount,
       sequence_number,
       bill.user_phone_number,
       client.id IS NOT NULL AS bill_type,
       cn.state AS credit_state,
       total,
       total_vat,
       total_before_vat,
       1 AS is_credit
FROM bill
JOIN credit_note cn ON cn.bill_id = bill.id
LEFT JOIN client  ON client.id = bill.client_id
WHERE bill.state >= 0
  AND (sqlc.narg('state_filter') IS NULL OR bill.state = sqlc.narg('state_filter'))
  AND (
        (sqlc.narg('query_name_like') IS NULL AND sqlc.narg('query_phone_digits') IS NULL)
     OR REGEXP_REPLACE(IFNULL(bill.user_phone_number, ''), '[^0-9]+', '') LIKE sqlc.narg('query_phone_digits')
     OR REGEXP_REPLACE(IFNULL(client.phone, ''),            '[^0-9]+', '') LIKE sqlc.narg('query_phone_digits')
     OR bill.userName LIKE sqlc.narg('query_name_like')
     OR client.name   LIKE sqlc.narg('query_name_like')
  )
  AND (sqlc.narg('phone_prefix') IS NULL
       OR bill.user_phone_number LIKE sqlc.narg('phone_prefix')
       OR client.phone           LIKE sqlc.narg('phone_prefix'))
  AND (sqlc.narg('seq_prefix') IS NULL OR bill.sequence_number_str LIKE sqlc.narg('seq_prefix'))
  AND (sqlc.narg('cursor_date') IS NULL
       OR bill.effective_date < sqlc.narg('cursor_date')
       OR (bill.effective_date = sqlc.narg('cursor_date') AND bill.id < sqlc.narg('cursor_id'))
       OR (bill.effective_date = sqlc.narg('cursor_date') AND bill.id = sqlc.narg('cursor_id')
           AND CAST(sqlc.narg('cursor_is_credit') AS UNSIGNED) > 1))
UNION ALL
SELECT bill.id AS id,
       bill.effective_date AS effective_date,
       payment_due_date,
       bill.state AS state,
       discount,
       sequence_number,
       bill.user_phone_number,
       client.id IS NOT NULL AS bill_type,
       0 AS credit_state,
       total,
       total_vat,
       total_before_vat,
       0 AS is_credit
FROM bill
LEFT JOIN client ON client.id = bill.client_id
WHERE bill.state >= 0
  AND (sqlc.narg('state_filter') IS NULL OR bill.state = sqlc.narg('state_filter'))
  AND (
        (sqlc.narg('query_name_like') IS NULL AND sqlc.narg('query_phone_digits') IS NULL)
     OR REGEXP_REPLACE(IFNULL(bill.user_phone_number, ''), '[^0-9]+', '') LIKE sqlc.narg('query_phone_digits')
     OR REGEXP_REPLACE(IFNULL(client.phone, ''),            '[^0-9]+', '') LIKE sqlc.narg('query_phone_digits')
     OR bill.userName LIKE sqlc.narg('query_name_like')
     OR client.name   LIKE sqlc.narg('query_name_like')
  )
  AND (sqlc.narg('phone_prefix') IS NULL
       OR bill.user_phone_number LIKE sqlc.narg('phone_prefix')
       OR client.phone           LIKE sqlc.narg('phone_prefix'))
  AND (sqlc.narg('seq_prefix') IS NULL OR bill.sequence_number_str LIKE sqlc.narg('seq_prefix'))
  AND (sqlc.narg('cursor_date') IS NULL
       OR bill.effective_date < sqlc.narg('cursor_date')
       OR (bill.effective_date = sqlc.narg('cursor_date') AND bill.id < sqlc.narg('cursor_id'))
       OR (bill.effective_date = sqlc.narg('cursor_date') AND bill.id = sqlc.narg('cursor_id')
           AND CAST(sqlc.narg('cursor_is_credit') AS UNSIGNED) > 0))
ORDER BY effective_date DESC, id DESC, is_credit DESC
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

-- name: RefreshBillTotals :exec
UPDATE bill
SET
  total_before_vat = COALESCE((SELECT SUM(bp.total_before_vat) FROM bill_product AS bp WHERE bp.bill_id = sqlc.arg('target_bill_id')), 0),
  total_vat = COALESCE((SELECT SUM(bp.vat_total) FROM bill_product AS bp WHERE bp.bill_id = sqlc.arg('target_bill_id')), 0),
  total = COALESCE((SELECT SUM(bp.total_including_vat) FROM bill_product AS bp WHERE bp.bill_id = sqlc.arg('target_bill_id')), 0),
  discount_amount = COALESCE((SELECT SUM(bp.discount) FROM bill_product AS bp WHERE bp.bill_id = sqlc.arg('target_bill_id')), 0)
WHERE bill.id = sqlc.arg('target_id');

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
select CAST(COALESCE(max(sequence_number), 1) AS UNSIGNED) from bill;

-- name: GetBillProductsWithArticle :many
SELECT bp.product_id, bp.price, bp.quantity,
       a.id AS article_id, a.articleNumber, a.genericArticleDescription
FROM bill_product bp
LEFT JOIN articles a ON a.id = bp.product_id
WHERE bp.bill_id = ?;

-- name: SoftDeleteBill :execresult
UPDATE bill SET state = -1 WHERE id = ?
