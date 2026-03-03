-- name: GetProduct :one
select  p.article_id, p.price, p.quantity, p.cost_price, p.shelf_number
from user
join store s on s.company_id = user.company_id
join product p on p.store_id = s.id
join car_part.oem_number o
where user.id = ? and o.articleId = p.article_id order by p.id desc;

-- name: GetBillProductByBillID :many
select * from bill_product where bill_id = ?;

-- name: GetBillManualProductByBillID :many
select * from bill_manual_product where bill_id = ?;
