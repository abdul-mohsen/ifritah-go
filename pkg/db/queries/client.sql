-- name: GetClients :many
-- Keyset pagination on (updated_at DESC, id DESC). Backed by
-- idx_client_keyset (migration 0003). Sort key carries updated_at so
-- the FE keeps the existing "recently-touched first" UX.
-- query_like is the sentinel-filter for search (pre-wrapped %term%
-- on the Go side; NULL = "no search"). Keeps SQL static (Sonar S2077).
--
-- Each sqlc.narg() literal appears once, in the `p` derived table, so
-- the rest of the query references `p.<col>` and the file stays
-- under plsql:S1192's duplicated-literal threshold.
SELECT c.* FROM client c
CROSS JOIN (SELECT CAST(sqlc.narg('query_like')        AS CHAR(255))   AS q,
                   CAST(sqlc.narg('cursor_updated_at') AS DATETIME(6)) AS cu,
                   CAST(sqlc.narg('cursor_id')         AS UNSIGNED)    AS ci) p
WHERE c.is_deleted = FALSE
  AND (p.q IS NULL
       OR c.name  LIKE p.q
       OR c.email LIKE p.q
       OR c.phone LIKE p.q)
  AND (p.cu IS NULL
       OR c.updated_at < p.cu
       OR (c.updated_at = p.cu AND c.id < p.ci))
ORDER BY c.updated_at DESC, c.id DESC
LIMIT ?;

-- name: GetClientByID :one
SELECT * FROM client WHERE id = ? and is_deleted = FALSE LIMIT 1;

-- name: CreateClient :exec
INSERT INTO client (name, company_name, email, phone, address, vat_number, number, bank_account, preferred_payment_method, credit_limit, payment_terms_days, short_address, commercial_registration) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateClient :exec
UPDATE client set name = ?, company_name=?, email=?, phone=?, address=?, vat_number=?, number=?, bank_account=?, preferred_payment_method=?, credit_limit=?, payment_terms_days=?, short_address=?, commercial_registration=? WHERE id = ?;

-- name: DeleteClient :exec
update client set is_deleted = TRUE where id = ?;
