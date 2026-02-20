-- name: GetCompanyIdByUser :one
SELECT company_id FROM user where id = ?;
