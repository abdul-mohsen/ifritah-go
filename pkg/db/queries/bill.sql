-- name: CreateBill :execresult
  insert into bill (effective_date, payment_due_date, state, discount, store_id, sequence_number, merchant_id, maintenance_cost, note, userName, client_id, user_phone_number)
  values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAllBill :many
SELECT * from(
			SELECT bill.id as id, effective_date, payment_due_date, bill.state as state, discount, sequence_number, bill.user_phone_number, client.id is not null as bill_type, cn.state as credit_state, total, total_vat, total_before_vat
			FROM bill_totals as bill
			JOIN credit_note  cn on cn.bill_id = bill.id
			LEFT JOIN client  on client.id = bill.buyer_id
			WHERE bill.state >= 0
			and (sqlc.narg('phonenumber') is null or bill.user_phone_number like sqlc.narg('phonenumber'))
			UNION
			SELECT bill.id as id, effective_date, payment_due_date, bill.state as state, discount, sequence_number, user_phone_number, client.id is not null as bill_type, 0 as credit_state, total, total_vat, total_before_vat from bill_totals as bill
			LEFT JOIN client  on client.id = bill.buyer_id
			WHERE bill. state >= 0
			and (sqlc.narg('phonenumber') is null or bill.user_phone_number like sqlc.narg('phonenumber'))
		) AS T ORDER BY id DESC LIMIT ? OFFSET ?;


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
FROM bill_totals b
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
FROM bill_totals b
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
user_phone_number = ?
WHERE id = ?;

-- name: AddProductToBill :exec
insert into bill_product (name, product_id, price, quantity, bill_id) values (?, ?, ?, ?, ?);

-- name: DeleteProductToBill :exec
DELETE FROM bill_product where bill_id = ?;

-- name: GetBillProductByBillID :many
select * from bill_product where bill_id = ?;
