-- name: GetAllPurchaseBill :many
select b.*
	from purchase_bill_totals as b
	join store on store.id = b.store_id
	join company on company.id = store.company_id
	join user on user.id= ? and company.id=user.company_id
	order by id desc limit ? offset ?;

-- name: GetPurchaseBillDetail :one
select effective_date, payment_due_date, b.state, sub_total, discount, vat, store_id, sequence_number, merchant_id,
			COALESCE(
				(SELECT JSON_ARRAYAGG(
					JSON_OBJECT(
						'product_id', p.product_id,
						'price', p.price,
						'quantity', p.quantity
					)
				)
				FROM purchase_bill_product p
				WHERE p.bill_id = b.id),
				JSON_ARRAY()) AS products,
			COALESCE(
				(SELECT JSON_ARRAYAGG(
					JSON_OBJECT(
						'part_name', m.part_name,
						'price', m.price,
						'quantity', m.quantity
					)
				)
				FROM bill_manual_purchase_product m
				WHERE m.bill_id = b.id),
				JSON_ARRAY()) AS manual_products
	from purchase_bill as b
	join store on store.id = b.store_id
	join company on company.id = store.company_id
	join user on user.id = ? and company.id=user.company_id
	where b.id = ? limit 1;
