-- name: GetAllPurchaseBill :many
-- Keyset pagination on (effective_date DESC, id DESC). Backed by
-- idx_pb_keyset (migration 0003) so the seek runs as a single index
-- range scan with no filesort. Caller fetches limit+1 to detect
-- has_more without a COUNT.
select b.*
	from purchase_bill as b
	join store on store.id = b.store_id
	join company on company.id = store.company_id
	join user on user.id = ? and company.id = user.company_id
	where b.state >= 0
	  and (
	        sqlc.narg('cursor_date') is null
	     or b.effective_date < sqlc.narg('cursor_date')
	     or (b.effective_date = sqlc.narg('cursor_date') and b.id < sqlc.narg('cursor_id'))
	  )
	order by b.effective_date desc, b.id desc
	limit ?;

-- name: GetPurchaseBillDetail :one
select b.*
	from purchase_bill as b
	join store on store.id = b.store_id
	join company on company.id = store.company_id
	join user on user.id = ? and company.id=user.company_id
	where b.id = ? limit 1;

-- name: GetPurchaseBillProducts :many
select * from purchase_bill_product p where p.bill_id = ?;

-- name: GetPurchaseBillAttachments :many
select * from purchase_bill_attachments p where p.purchase_bill_id = ?;

-- name: UpdatePurchaseBill :exec
UPDATE purchase_bill set effective_date = ?, payment_due_date = ?, state = ?, discount = ?, store_id = ?, merchant_id = ?, supplier_id = ?, supplier_sequence_number = ?, payment_method = ?, deliver_date = ? where id = ?;

-- name: DeleteProductPurchaseBill :exec
DELETE FROM purchase_bill_product where bill_id = ?;

-- name: AddPurchaseBill :execresult
insert into purchase_bill (effective_date, payment_due_date, state, discount, store_id, merchant_id, supplier_id, supplier_sequence_number, pdf_link, payment_method, deliver_date)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: AddProductToBillPurchase :exec
insert into purchase_bill_product  (product_id, name, price, quantity, bill_id) values (?, ?, ?, ?, ?);

-- name: AddAttachmentsPurchaseBill :exec
insert into purchase_bill_attachments  (purchase_bill_id, file_key) values (?, ?);
