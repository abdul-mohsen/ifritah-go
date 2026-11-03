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
JOIN credit_note cn ON cn.id = (SELECT MAX(c.id) FROM credit_note c WHERE c.bill_id = b.id)
WHERE b.id = ? LIMIT 1;

-- name: GetCreditNoteByRequestIdentity :one
SELECT id, bill_id, request_fingerprint, request_fingerprint_version FROM credit_note WHERE pos_id = ? AND bill_request_id = ?;

-- name: ListBillLinesForNote :many
SELECT id, product_id, name, price, quantity, discount, vat FROM bill_product WHERE bill_id = ?;

-- name: ListCreditedLinesForBill :many
SELECT cl.bill_product_id, cl.quantity
FROM credit_note_line cl
JOIN credit_note cn ON cn.id = cl.credit_note_id
WHERE cn.bill_id = ?
FOR UPDATE;

-- name: ListCreditAmountsForBill :many
SELECT amount FROM credit_note WHERE bill_id = ? FOR UPDATE;

-- name: InsertCreditNote :execresult
INSERT INTO credit_note (bill_id, state, note, amount, lines_total, invoice_uuid, pos_id, bill_request_id, request_fingerprint, request_fingerprint_version)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertCreditNoteLine :exec
INSERT INTO credit_note_line (credit_note_id, bill_product_id, name, quantity, price, discount, vat)
VALUES (?, ?, ?, ?, ?, ?, ?);
