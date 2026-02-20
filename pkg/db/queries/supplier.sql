-- name: UpdateSupplier :exec
UPDATE supplier SET name=?, address=?, phone_number=?, number=?, vat_number=?, bank_account=? WHERE company_id=? and id=?;
