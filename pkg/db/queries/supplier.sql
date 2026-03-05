-- name: UpdateSupplier :exec
UPDATE supplier SET name=?, address=?, phone_number=?, number=?, vat_number=?, bank_account=? WHERE company_id=? and id=?;

-- name: GetSupplier :one
SELECT * From supplier where company_id = ? and is_deleted = FALSE and id = ?;

-- name: GetAllSupplier :many
SELECT * From supplier where is_deleted = FALSE order by id desc limit ? offset ?
