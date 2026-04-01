-- name: UpdateSupplier :exec
UPDATE supplier SET name=?, address=?, phone_number=?, number=?, vat_number=?, bank_account=?, is_postpaid=?, credit_limit=?, payment_terms_days=?, preferred_payment_method=?, commercial_registration=?, short_address=?, email=? WHERE id=?;

-- name: GetSupplier :one
SELECT * From supplier where company_id = ? and is_deleted = FALSE and id = ?;

-- name: GetAllSupplier :many
SELECT * From supplier where is_deleted = FALSE order by id desc limit ? offset ?;

-- name: AddSupplier :execresult
INSERT INTO supplier (company_id, name, address, phone_number, number, vat_number, bank_account, is_postpaid, credit_limit, payment_terms_days, preferred_payment_method, commercial_registration, short_address, email) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
