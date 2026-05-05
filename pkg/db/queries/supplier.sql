-- name: UpdateSupplier :exec
UPDATE supplier SET name=?, address=?, phone_number=?, number=?, vat_number=?, bank_account=?, is_postpaid=?, credit_limit=?, payment_terms_days=?, preferred_payment_method=?, commercial_registration=?, short_address=?, email=? WHERE id=?;

-- name: GetSupplier :one
SELECT * From supplier where is_deleted = FALSE and id = ?;

-- name: GetAllSupplier :many
-- Keyset pagination on (id DESC). Primary key serves the scan order.
-- query_like is the canonical sentinel-filter for search: NULL means
-- "no search", otherwise it is the pre-wrapped %term% string. Keeping
-- the SQL static avoids Sonar S2077 (dynamic SQL).
--
-- Each sqlc.narg() is referenced once (in the `p` derived table) so
-- the file stays under plsql:S1192's repeated-literal threshold.
SELECT s.* From supplier s
CROSS JOIN (SELECT CAST(sqlc.narg('query_like') AS CHAR(255)) AS q,
                   CAST(sqlc.narg('cursor_id')  AS UNSIGNED)  AS ci) p
WHERE s.is_deleted = FALSE
  AND (p.q IS NULL
       OR s.name         LIKE p.q
       OR s.phone_number LIKE p.q
       OR s.vat_number   LIKE p.q)
  AND (p.ci IS NULL OR s.id < p.ci)
ORDER BY s.id DESC
LIMIT ?;

-- name: AddSupplier :execresult
INSERT INTO supplier (company_id, name, address, phone_number, number, vat_number, bank_account, is_postpaid, credit_limit, payment_terms_days, preferred_payment_method, commercial_registration, short_address, email) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
