-- name: ListStoresForUserScope :many
SELECT s.id, s.addressId, s.name
FROM store AS s
JOIN user AS u ON u.id = sqlc.arg('user_id')
LEFT JOIN branches AS b ON b.id = s.branch_id
WHERE s.company_id = u.company_id
  AND (
        sqlc.narg('role') IS NULL
     OR sqlc.narg('role') = 'admin'
     OR sqlc.narg('role') = 'owner'
     OR (sqlc.narg('role') = 'manager' AND b.manager_id = u.id)
     OR sqlc.narg('role') = 'employee'
  )
ORDER BY s.id;
