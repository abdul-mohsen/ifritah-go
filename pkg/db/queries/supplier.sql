-- name: UpdateSupplier :exec
UPDATE supplier SET name=?, address=?, phone_number=?, number=?, vat_number=?, bank_account=?, is_postpaid=?, credit_limit=?, payment_terms_days=?, preferred_payment_method=?, commercial_registration=?, short_address=?, email=? WHERE id=?;

-- name: GetSupplier :one
SELECT * From supplier where is_deleted = FALSE and id = ?;

-- name: GetAllSupplier :many
-- Keyset on (id DESC). Typed filters AND with query_like.
SELECT s.* From supplier s
CROSS JOIN (SELECT CAST(sqlc.narg('query_like') AS CHAR(255)) AS q,
                   CAST(sqlc.narg('phone_prefix') AS CHAR(20))  AS fp,
                   CAST(sqlc.narg('vat_prefix') AS CHAR(20))  AS fv,
                   CAST(sqlc.narg('cr_prefix') AS CHAR(50))  AS fc,
                   CAST(sqlc.narg('cursor_id')           AS UNSIGNED)  AS ci) p
WHERE s.is_deleted = FALSE
  AND (p.q IS NULL
       OR s.name COLLATE utf8mb4_unicode_ci LIKE p.q
       OR s.phone_number COLLATE utf8mb4_unicode_ci LIKE p.q
       OR s.vat_number COLLATE utf8mb4_unicode_ci LIKE p.q)
  AND (p.fp IS NULL OR s.phone_number COLLATE utf8mb4_unicode_ci LIKE p.fp)
  AND (p.fv IS NULL OR s.vat_number COLLATE utf8mb4_unicode_ci LIKE p.fv)
  AND (p.fc IS NULL OR s.commercial_registration COLLATE utf8mb4_unicode_ci LIKE p.fc)
  AND (p.ci IS NULL OR s.id < p.ci)
ORDER BY s.id DESC
LIMIT ?;

-- name: AddSupplier :execresult
INSERT INTO supplier (company_id, name, address, phone_number, number, vat_number, bank_account, is_postpaid, credit_limit, payment_terms_days, preferred_payment_method, commercial_registration, short_address, email) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
