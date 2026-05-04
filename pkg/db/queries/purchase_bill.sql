-- name: GetAllPurchaseBill :many
-- Keyset pagination on (effective_date DESC, id DESC). Backed by
-- idx_pb_keyset (migration 0003) so the seek runs as a single index
-- range scan with no filesort. Caller fetches limit+1 to detect
-- has_more without a COUNT.
--
-- Search:
--   - query_like: pre-wrapped %term% used against supplier.name.
--   - query_seq_exact: integer value of q when q is all-digits
--     (exact match on supplier_sequence_number). NULL otherwise.
--   All-NULL = "no search".
--
-- NOTE: total exact-match deferred — sqlc nullable-decimal narg
-- limitation (see bill.sql for details). Will land in a follow-up
-- once we move to a nullable-decimal-aware pattern.
--
-- state_filter: NULL = "any non-deleted" (state >= 0). Otherwise
-- exact match on b.state. Negative values are filtered out at the
-- request layer (mapped to NULL before binding).
select b.*
	from purchase_bill as b
	join store on store.id = b.store_id
	join company on company.id = store.company_id
	join user on user.id = ? and company.id = user.company_id
	left join supplier on supplier.id = b.supplier_id
	where b.state >= 0
	  and (sqlc.narg('state_filter') is null or b.state = sqlc.narg('state_filter'))
	  and (
	        (sqlc.narg('query_like') is null
	         and sqlc.narg('query_seq_exact') is null)
	     or supplier.name like sqlc.narg('query_like')
	     or CAST(b.supplier_sequence_number AS CHAR) = sqlc.narg('query_seq_exact')
	  )
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
