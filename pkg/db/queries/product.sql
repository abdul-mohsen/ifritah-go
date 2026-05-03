-- name: GetProduct :one
select p.* from product p where p.id = ? and is_deleted = False;

-- name: GetAllProduct :many
-- Keyset pagination on (id DESC). The InnoDB primary key already
-- serves the scan order natively — no extra index needed. Caller
-- fetches limit+1 to detect has_more.
select p.*
from user
join store s on s.company_id = user.company_id
join product p on p.store_id = s.id
where user.id = ? and is_deleted = False
  and (sqlc.narg('cursor_id') is null or p.id < sqlc.narg('cursor_id'))
ORDER BY p.id DESC
LIMIT ?;

-- name: AddProduct :execresult
INSERT INTO product (article_id, quantity, price, cost_price ,shelf_number, store_id, name) VALUES (?,?,?,?,?,?,?)
ON Duplicate key update
price = values(price),
cost_price = values(cost_price),
shelf_number = Values(shelf_number),
is_deleted = FALSE,
quantity = quantity + VALUES(quantity);

-- name: UpdateProduct :exec
update product  set price = ?, cost_price = ?, shelf_number = ?, quantity = ? where id = ?;

-- name: DeleteProduct :exec
update product set is_deleted = TRUE where id = ?;

-- name: SearchProduct :many
SELECT p.*
FROM user
JOIN store s ON s.company_id = user.company_id
JOIN product p ON p.store_id = s.id
WHERE user.id = ? AND p.is_deleted = FALSE
  AND (CAST(p.id AS CHAR) LIKE CONCAT('%', ?, '%')
       OR COALESCE(p.shelf_number, '') LIKE CONCAT('%', ?, '%'))
ORDER BY p.id DESC
LIMIT ?;

-- name: SearchQuantityByID :one
SELECT quantity FROM product WHERE id = ? FOR UPDATE
