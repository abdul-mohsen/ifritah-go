-- name: GetProduct :one
select p.* from product p where p.id = ? and p.is_deleted = False;

-- name: GetAllProduct :many
-- Keyset pagination on (id DESC). The InnoDB primary key already
-- serves the scan order natively — no extra index needed. Caller
-- fetches limit+1 to detect has_more.
-- query_like is the sentinel-filter for search across name and
-- shelf_number; query_id_match is set to the integer value of q
-- when q is all-digits (exact id match), 0 otherwise. NULL on both
-- means "no search".
--
-- Each sqlc.narg() is referenced once (in the `prm` derived table) so
-- the file stays under plsql:S1192's repeated-literal threshold.
select p.*
from user
join store s on s.company_id = user.company_id
join product p on p.store_id = s.id
CROSS JOIN (SELECT CAST(sqlc.narg('query_like')     AS CHAR(255)) AS q,
                   CAST(sqlc.narg('query_id_match') AS UNSIGNED)  AS qid,
                   CAST(sqlc.narg('cursor_id')      AS UNSIGNED)  AS ci) prm
where user.id = ? and p.is_deleted = False
  and (prm.q IS NULL
       OR p.name LIKE prm.q COLLATE utf8mb4_unicode_ci
       OR COALESCE(p.shelf_number,'') LIKE prm.q COLLATE utf8mb4_unicode_ci
       OR p.id = prm.qid)
  and (prm.ci IS NULL OR p.id < prm.ci)
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
