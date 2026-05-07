-- name: GetClients :many
-- Keyset pagination on (updated_at DESC, id DESC).
-- Typed filters AND with `query_like` and with each other.
SELECT c.* FROM client c
CROSS JOIN (SELECT CAST(sqlc.narg('query_like') AS CHAR(255))   AS q,
                   CAST(sqlc.narg('phone_prefix') AS CHAR(20))    AS fp,
                   CAST(sqlc.narg('vat_prefix') AS CHAR(20))    AS fv,
                   CAST(sqlc.narg('cr_prefix') AS CHAR(50))    AS fc,
                   CAST(sqlc.narg('cursor_updated_at')   AS DATETIME(6)) AS cu,
                   CAST(sqlc.narg('cursor_id')           AS UNSIGNED)    AS ci) p
WHERE c.is_deleted = FALSE
  AND (p.q IS NULL
       OR c.name COLLATE utf8mb4_unicode_ci LIKE p.q
       OR c.email COLLATE utf8mb4_unicode_ci LIKE p.q
       OR c.phone COLLATE utf8mb4_unicode_ci LIKE p.q)
  AND (p.fp IS NULL OR c.phone COLLATE utf8mb4_unicode_ci LIKE p.fp)
  AND (p.fv IS NULL OR c.vat_number COLLATE utf8mb4_unicode_ci LIKE p.fv)
  AND (p.fc IS NULL OR c.commercial_registration COLLATE utf8mb4_unicode_ci LIKE p.fc)
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
