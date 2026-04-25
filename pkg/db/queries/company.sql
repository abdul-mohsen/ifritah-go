-- name: GetCompanyIdByUser :one
SELECT company_id FROM user where id = ?;

-- name: GetCompanyByID :one
SELECT id, name, COALESCE(name_ar,'') AS name_ar,
       COALESCE(vat_registration_number,'') AS vat_registration_number,
       COALESCE(commercial_registration_number,'') AS commercial_registration_number,
       COALESCE(business_category,'Supply activities') AS business_category
FROM company WHERE id = ? LIMIT 1;

-- name: UpdateCompany :exec
UPDATE company
   SET name = ?, name_ar = ?
 WHERE id = ?;
