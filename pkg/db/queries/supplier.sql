-- name: UpdateSupplier :exec
UPDATE supplier SET name=?, address=?, phone_number=?, number=?, vat_number=?, bank_account=?, is_postpaid=?, credit_limit=?, payment_terms_days=? WHERE company_id=? and id=?;

-- name: GetSupplier :one
SELECT * From supplier where company_id = ? and is_deleted = FALSE and id = ?;

-- name: GetAllSupplier :many
SELECT * From supplier where is_deleted = FALSE order by id desc limit ? offset ?;

-- name: AddSupplier :exec
INSERT INTO supplier (company_id, name, address, phone_number, number, vat_number, bank_account, is_postpaid, credit_limit, payment_terms_days) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
