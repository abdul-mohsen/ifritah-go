-- name: GetAllPurchaseBill :many
select b.*
	from purchase_bill_totals as b
	join store on store.id = b.store_id
	join company on company.id = store.company_id
	join user on user.id= ? and company.id=user.company_id
	where b.state >= 0
	order by id desc limit ? offset ?;

-- name: GetPurchaseBillDetail :one
select b.*
	from purchase_bill_totals as b
	join store on store.id = b.store_id
	join company on company.id = store.company_id
	join user on user.id = ? and company.id=user.company_id
	where b.id = ? limit 1;

-- name: GetPurchaseBillProducts :many
select * from purchase_bill_product p where p.bill_id = ?;

-- name: GetPurchaseBillAttachments :many
select * from purchase_bill_attachments p where p.purchase_bill_id = ?;

-- name: UpdatePurchaseBill :exec
UPDATE purchase_bill set effective_date = ?, payment_due_date = ?, state = ?, discount = ?, store_id = ?, merchant_id = ?, supplier_id = ?, sequence_number = ? where id = ?;

-- name: DeleteProductPurchaseBill :exec
DELETE FROM purchase_bill_product where bill_id = ?;

-- name: AddPurchaseBill :execresult
insert into purchase_bill (effective_date, payment_due_date, state, discount, store_id, merchant_id, supplier_id, sequence_number, pdf_link)
values (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: AddProductToBillPurchase :exec
insert into purchase_bill_product  (product_id, name, price, quantity, bill_id) values (?, ?, ?, ?, ?);

-- name: AddAttachmentsPurchaseBill :exec
insert into purchase_bill_attachments  (purchase_bill_id, file_key) values (?, ?);
