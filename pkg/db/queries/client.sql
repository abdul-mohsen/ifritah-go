-- name: GetClients :many
SELECT * FROM client
WHERE is_deleted = FALSE
  AND (sqlc.narg('query_like') IS NULL
       OR name  LIKE sqlc.narg('query_like')
       OR email LIKE sqlc.narg('query_like')
       OR phone LIKE sqlc.narg('query_like'))
  AND (sqlc.narg('phone_prefix') IS NULL OR phone                   LIKE sqlc.narg('phone_prefix'))
  AND (sqlc.narg('vat_prefix')   IS NULL OR vat_number              LIKE sqlc.narg('vat_prefix'))
  AND (sqlc.narg('cr_prefix')    IS NULL OR commercial_registration LIKE sqlc.narg('cr_prefix'))
  AND (sqlc.narg('cursor_updated_at') IS NULL
       OR updated_at < sqlc.narg('cursor_updated_at')
       OR (updated_at = sqlc.narg('cursor_updated_at') AND id < sqlc.narg('cursor_id')))
ORDER BY updated_at DESC, id DESC
LIMIT ?;

-- name: GetClientByID :one
SELECT * FROM client WHERE id = ? and is_deleted = FALSE LIMIT 1;

-- name: CreateClient :exec
INSERT INTO client (name, company_name, email, phone, address, vat_number, number, bank_account, preferred_payment_method, credit_limit, payment_terms_days, short_address, commercial_registration) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateClient :exec
UPDATE client set name = ?, company_name=?, email=?, phone=?, address=?, vat_number=?, number=?, bank_account=?, preferred_payment_method=?, credit_limit=?, payment_terms_days=?, short_address=?, commercial_registration=? WHERE id = ?;

-- name: DeleteClient :exec
update client set is_deleted = TRUE where id = ?;
