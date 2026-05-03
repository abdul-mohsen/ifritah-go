-- name: UpdateSupplier :exec
UPDATE supplier SET name=?, address=?, phone_number=?, number=?, vat_number=?, bank_account=?, is_postpaid=?, credit_limit=?, payment_terms_days=?, preferred_payment_method=?, commercial_registration=?, short_address=?, email=? WHERE id=?;

-- name: GetSupplier :one
SELECT * From supplier where is_deleted = FALSE and id = ?;

-- name: GetAllSupplier :many
-- Keyset pagination on (id DESC). Primary key serves the scan order.
SELECT * From supplier
WHERE is_deleted = FALSE
  AND (sqlc.narg('cursor_id') IS NULL OR id < sqlc.narg('cursor_id'))
ORDER BY id DESC
LIMIT ?;

-- name: AddSupplier :execresult
INSERT INTO supplier (company_id, name, address, phone_number, number, vat_number, bank_account, is_postpaid, credit_limit, payment_terms_days, preferred_payment_method, commercial_registration, short_address, email) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
