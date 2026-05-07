-- name: UpdateSupplier :exec
UPDATE supplier SET name=?, address=?, phone_number=?, number=?, vat_number=?, bank_account=?, is_postpaid=?, credit_limit=?, payment_terms_days=?, preferred_payment_method=?, commercial_registration=?, short_address=?, email=? WHERE id=?;

-- name: GetSupplier :one
SELECT * From supplier where is_deleted = FALSE and id = ?;

-- name: GetAllSupplier :many
SELECT * FROM supplier
WHERE is_deleted = FALSE
  AND (sqlc.narg('query_like') IS NULL
       OR name         LIKE sqlc.narg('query_like')
       OR phone_number LIKE sqlc.narg('query_like')
       OR vat_number   LIKE sqlc.narg('query_like'))
  AND (sqlc.narg('phone_prefix') IS NULL OR phone_number            LIKE sqlc.narg('phone_prefix'))
  AND (sqlc.narg('vat_prefix')   IS NULL OR vat_number              LIKE sqlc.narg('vat_prefix'))
  AND (sqlc.narg('cr_prefix')    IS NULL OR commercial_registration LIKE sqlc.narg('cr_prefix'))
  AND (sqlc.narg('cursor_id')    IS NULL OR id < sqlc.narg('cursor_id'))
ORDER BY id DESC
LIMIT ?;

-- name: AddSupplier :execresult
INSERT INTO supplier (company_id, name, address, phone_number, number, vat_number, bank_account, is_postpaid, credit_limit, payment_terms_days, preferred_payment_method, commercial_registration, short_address, email) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
