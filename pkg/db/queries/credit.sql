-- name: GetCreditBillByID :one
SELECT
CONCAT('https://ifritah.com/bill/', b.id) AS url,
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
cn.state as credit_state,
cn.note as credit_note,
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
JOIN credit_note  cn on cn.bill_id = b.id
WHERE b.id = ? LIMIT 1;
