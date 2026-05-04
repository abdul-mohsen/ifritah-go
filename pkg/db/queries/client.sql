-- name: GetClients :many
-- Keyset pagination on (updated_at DESC, id DESC). Backed by
-- idx_client_keyset (migration 0003). Sort key carries updated_at so
-- the FE keeps the existing "recently-touched first" UX.
-- query_like is the sentinel-filter for search (pre-wrapped %term%
-- on the Go side; NULL = "no search"). Keeps SQL static (Sonar S2077).
SELECT * FROM client
WHERE is_deleted = FALSE
  AND (sqlc.narg('query_like') IS NULL
       OR name LIKE sqlc.narg('query_like')
       OR email LIKE sqlc.narg('query_like')
       OR phone LIKE sqlc.narg('query_like'))
  AND (
        sqlc.narg('cursor_updated_at') IS NULL
     OR updated_at < sqlc.narg('cursor_updated_at')
     OR (updated_at = sqlc.narg('cursor_updated_at') AND id < sqlc.narg('cursor_id'))
  )
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
