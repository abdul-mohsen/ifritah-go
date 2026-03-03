-- name: CreateBill :execresult
  insert into bill (effective_date, payment_due_date, state, discount, store_id, sequence_number, merchant_id, maintenance_cost, note, userName, buyer_id, user_phone_number)
  values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAllBill :many
SELECT * from(
			SELECT bill.id as id, effective_date, payment_due_date, bill.state as state, discount, sequence_number, bill.user_phone_number, TRUE as bill_type, cn.state as credit_state, total, total_vat, total_before_vat
			FROM bill_totals as bill
			JOIN credit_note  cn on cn.bill_id = bill.id
			WHERE bill.user_phone_number like ?  and bill.state >= 0
			UNION
			SELECT bill.id as id, effective_date, payment_due_date, bill.state as state, discount, sequence_number, user_phone_number, TRUE as bill_type, 0 as credit_state, total, total_vat, total_before_vat from bill_totals as bill WHERE bill.user_phone_number like ? and bill. state >= 0
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
company.name as company_name,
company.vat_registration_number,
store.address_name,
store.name as store_name,
qr_code,
total_before_vat,
total_vat,
total,
company.commercial_registration_number,
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
  JSON_ARRAY()) AS products,
COALESCE(
  (SELECT JSON_ARRAYAGG(
	  JSON_OBJECT(
		'part_name', m.part_name,
		'price', m.price,
		'quantity', m.quantity
	  )
	)
	FROM bill_manual_product m
	WHERE m.bill_id = b.id),
  JSON_ARRAY()) AS manual_products
FROM
bill_totals b
JOIN
store on store.id = b.store_id
JOIN
company on company.id = store.company_id
WHERE
b.id = ?
LIMIT 1 ;
