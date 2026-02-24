-- name: GetClients :many
SELECT * FROM client ORDER BY updated_at DESC LIMIT ? OFFSET ?;

-- name: GetClientByID :one
SELECT * FROM client WHERE id = ? LIMIT 1;

-- name: CreateClient :exec
INSERT INTO client (name, company_name, email, phone, address, vat_number) VALUES (?, ?, ?, ?, ?, ?);

-- name: UpdateClient :exec
UPDATE client set name = ?, company_name=?, email=?, phone=?, address=?, vat_number=? WHERE id = ?;
