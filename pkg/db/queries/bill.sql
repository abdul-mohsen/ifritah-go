-- name: CreateBill :execresult
  insert into bill (effective_date, payment_due_date, state, discount, store_id, sequence_number, merchant_id, maintenance_cost, note, userName, client_id, user_phone_number, payment_method, branch_id, deliver_date)
  values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAllBill :many
-- Keyset pagination on (effective_date DESC, id DESC, is_credit DESC).
-- query_name_like / query_phone_digits split (PR #32) for free-text q.
-- filter_phone_prefix / filter_seq_prefix: typed-chip prefix filters.
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
CROSS JOIN (SELECT CAST(sqlc.narg('state_filter')        AS SIGNED)       AS sf,
                   CAST(sqlc.narg('query_name_like') AS CHAR(255))    AS qn,
                   CAST(sqlc.narg('query_phone_digits') AS CHAR(32))     AS qp,
                   CAST(sqlc.narg('phone_prefix') AS CHAR(20))    AS fp,
                   CAST(sqlc.narg('seq_prefix') AS CHAR(32))    AS fs,
                   CAST(sqlc.narg('cursor_date')        AS DATETIME(6))  AS cd,
                   CAST(sqlc.narg('cursor_id')          AS UNSIGNED)     AS ci,
                   CAST(sqlc.narg('cursor_is_credit')   AS UNSIGNED)     AS cic) p
WHERE bill.state >= 0
  AND (p.sf IS NULL OR bill.state = p.sf)
  AND (
        (p.qn IS NULL AND p.qp IS NULL)
     OR (p.qp IS NOT NULL AND (
            REGEXP_REPLACE(IFNULL(bill.user_phone_number, ''), '[^0-9]+', '') COLLATE utf8mb4_unicode_ci LIKE p.qp
         OR REGEXP_REPLACE(IFNULL(client.phone, ''),            '[^0-9]+', '') COLLATE utf8mb4_unicode_ci LIKE p.qp
        ))
     OR (p.qn IS NOT NULL AND ( bill.userName COLLATE utf8mb4_unicode_ci LIKE p.qn
         OR client.name COLLATE utf8mb4_unicode_ci LIKE p.qn
        ))
  )
  AND (
        p.fp IS NULL
     OR bill.user_phone_number COLLATE utf8mb4_unicode_ci LIKE p.fp
     OR client.phone COLLATE utf8mb4_unicode_ci LIKE p.fp
  )
  AND (
        p.fs IS NULL
     OR CAST(bill.sequence_number AS CHAR) COLLATE utf8mb4_unicode_ci LIKE p.fs
  )
  AND (
        p.cd IS NULL
     OR bill.effective_date < p.cd
     OR (bill.effective_date = p.cd AND bill.id < p.ci)
     OR (bill.effective_date = p.cd AND bill.id = p.ci AND 1 < p.cic)
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
CROSS JOIN (SELECT CAST(sqlc.narg('state_filter')        AS SIGNED)       AS sf,
                   CAST(sqlc.narg('query_name_like') AS CHAR(255))    AS qn,
                   CAST(sqlc.narg('query_phone_digits') AS CHAR(32))     AS qp,
                   CAST(sqlc.narg('phone_prefix') AS CHAR(20))    AS fp,
                   CAST(sqlc.narg('seq_prefix') AS CHAR(32))    AS fs,
                   CAST(sqlc.narg('cursor_date')        AS DATETIME(6))  AS cd,
                   CAST(sqlc.narg('cursor_id')          AS UNSIGNED)     AS ci,
                   CAST(sqlc.narg('cursor_is_credit')   AS UNSIGNED)     AS cic) q
WHERE bill.state >= 0
  AND (q.sf IS NULL OR bill.state = q.sf)
  AND (
        (q.qn IS NULL AND q.qp IS NULL)
     OR (q.qp IS NOT NULL AND (
            REGEXP_REPLACE(IFNULL(bill.user_phone_number, ''), '[^0-9]+', '') COLLATE utf8mb4_unicode_ci LIKE q.qp
         OR REGEXP_REPLACE(IFNULL(client.phone, ''),            '[^0-9]+', '') COLLATE utf8mb4_unicode_ci LIKE q.qp
        ))
     OR (q.qn IS NOT NULL AND ( bill.userName COLLATE utf8mb4_unicode_ci LIKE q.qn
         OR client.name COLLATE utf8mb4_unicode_ci LIKE q.qn
        ))
  )
  AND (
        q.fp IS NULL
     OR bill.user_phone_number COLLATE utf8mb4_unicode_ci LIKE q.fp
     OR client.phone COLLATE utf8mb4_unicode_ci LIKE q.fp
  )
  AND (
        q.fs IS NULL
     OR CAST(bill.sequence_number AS CHAR) COLLATE utf8mb4_unicode_ci LIKE q.fs
  )
  AND (
        q.cd IS NULL
     OR bill.effective_date < q.cd
     OR (bill.effective_date = q.cd AND bill.id < q.ci)
     OR (bill.effective_date = q.cd AND bill.id = q.ci AND 0 < q.cic)
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
select CAST(COALESCE(max(sequence_number), 1) AS UNSIGNED) from bill;

-- name: GetBillProductsWithArticle :many
SELECT bp.product_id, bp.price, bp.quantity,
       a.id AS article_id, a.articleNumber, a.genericArticleDescription
FROM bill_product bp
LEFT JOIN articles a ON a.id = bp.product_id
WHERE bp.bill_id = ?;

-- name: SoftDeleteBill :execresult
UPDATE bill SET state = -1 WHERE id = ?
