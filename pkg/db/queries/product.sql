-- name: GetProduct :one
select p.* from product p where p.id = ? and p.is_deleted = False;

-- name: GetAllProduct :many
SELECT p.*
FROM user
JOIN store s     ON s.company_id = user.company_id
JOIN product p   ON p.store_id = s.id
LEFT JOIN articles a ON a.legacyArticleId = p.article_id
WHERE user.id = ? AND p.is_deleted = FALSE
  AND (sqlc.narg('query_like') IS NULL
       OR p.name LIKE sqlc.narg('query_like')
       OR COALESCE(p.shelf_number,'') LIKE sqlc.narg('query_like')
       OR p.id = sqlc.narg('query_id_match'))
  AND (sqlc.narg('part_number_prefix') IS NULL
       OR a.articleNumber LIKE sqlc.narg('part_number_prefix'))
  AND (sqlc.narg('barcode_prefix') IS NULL
       OR EXISTS (SELECT 1 FROM articleean ae
                   WHERE ae.legacyArticleId = p.article_id
                     AND ae.eancode LIKE sqlc.narg('barcode_prefix')))
  AND (sqlc.narg('cursor_id') IS NULL OR p.id < sqlc.narg('cursor_id'))
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
