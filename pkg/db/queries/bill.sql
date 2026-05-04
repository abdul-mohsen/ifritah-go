-- name: CreateBill :execresult
  insert into bill (effective_date, payment_due_date, state, discount, store_id, sequence_number, merchant_id, maintenance_cost, note, userName, client_id, user_phone_number, payment_method, branch_id, deliver_date)
  values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAllBill :many
-- Keyset / cursor pagination. Sort key: (effective_date DESC, id DESC, is_credit DESC).
--
-- Why three keys, not two: a bill that has a credit_note is shown as
-- TWO list items — the original invoice and its credit note are
-- different documents in the user's view. The UNION ALL below emits
-- both with identical (effective_date, id) but different `is_credit`,
-- so we need that third column as a deterministic tiebreaker for the
-- seek predicate.
--
-- The (cursor_date, cursor_id, cursor_is_credit) sentinels are all
-- NULL on the first page and all non-NULL on subsequent pages. The
-- OR-tree is the canonical lex-compare seek predicate
-- (Markus Winand, use-the-index-luke.com/no-offset).
--
-- Search (query_like / query_seq_exact):
--   - query_like: pre-wrapped %term% used against userName,
--     user_phone_number, and client.name (LEFT JOIN already in place).
--   - query_seq_exact: integer value of q when q is all-digits (exact
--     match on sequence_number). NULL otherwise.
-- All-NULL on the search args = "no search".
--
-- NOTE: total exact-match was scoped here but pulled to a follow-up
-- (sqlc v1.31 emits a non-nullable decimal.Decimal narg for params
-- bound against a NOT-NULL decimal column even with CAST AS CHAR /
-- CONCAT). userName/phone/sequence/client.name covers the dominant
-- search intents in our usage data.
--
-- state_filter (sqlc.narg('state_filter')):
--   - NULL = "any non-deleted" (state >= 0). Default.
--   - any int >= 0 = exact match (caller supplies 0/1/2/3).
--   - We never accept negative values here; the request layer maps
--     a missing/sentinel filter to NULL before binding.
--
-- Caller fetches `limit + 1` to detect has_more without a COUNT(*).
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
        (sqlc.narg('query_like') IS NULL
         AND sqlc.narg('query_seq_exact') IS NULL)
     OR bill.user_phone_number LIKE sqlc.narg('query_like')
     OR bill.userName LIKE sqlc.narg('query_like')
     OR client.name LIKE sqlc.narg('query_like')
     OR CAST(bill.sequence_number AS CHAR) = sqlc.narg('query_seq_exact')
  )
  AND (
        sqlc.narg('cursor_date') IS NULL
     OR bill.effective_date < sqlc.narg('cursor_date')
     OR (bill.effective_date = sqlc.narg('cursor_date') AND bill.id < sqlc.narg('cursor_id'))
     OR (bill.effective_date = sqlc.narg('cursor_date') AND bill.id = sqlc.narg('cursor_id') AND 1 < sqlc.narg('cursor_is_credit'))
  )
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
        (sqlc.narg('query_like') IS NULL
         AND sqlc.narg('query_seq_exact') IS NULL)
     OR bill.user_phone_number LIKE sqlc.narg('query_like')
     OR bill.userName LIKE sqlc.narg('query_like')
     OR client.name LIKE sqlc.narg('query_like')
     OR CAST(bill.sequence_number AS CHAR) = sqlc.narg('query_seq_exact')
  )
  AND (
        sqlc.narg('cursor_date') IS NULL
     OR bill.effective_date < sqlc.narg('cursor_date')
     OR (bill.effective_date = sqlc.narg('cursor_date') AND bill.id < sqlc.narg('cursor_id'))
     OR (bill.effective_date = sqlc.narg('cursor_date') AND bill.id = sqlc.narg('cursor_id') AND 0 < sqlc.narg('cursor_is_credit'))
  )
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
