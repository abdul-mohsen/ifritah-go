-- name: GetProduct :one
select p.* from product p where p.id = ?;

-- name: GetAllProduct :many
select  p.article_id, p.price, p.quantity, p.cost_price, p.shelf_number
from user
join store s on s.company_id = user.company_id
join product p on p.store_id = s.id
where user.id = ?
ORDER BY p.id DESC LIMIT ? OFFSET ?;


-- name: AddProduct :execresult
INSERT INTO product (article_id, quantity, price, cost_price ,shelf_number, store_id) VALUES (?,?,?,?,?,?)
ON Duplicate key update
price = values(price),
cost_price = values(cost_price),
shelf_number = Values(shelf_number),
quantity = quantity + VALUES(quantity);

-- name: UpdateProduct :exec
update product  set price = ?, cost_price = ?, shelf_number = ?, quantity = ? where id = ?;
