-- name: GetAllPurchaseBill :many
-- Keyset pagination on (effective_date DESC, id DESC). Backed by
-- idx_pb_keyset (migration 0003) so the seek runs as a single index
-- range scan with no filesort. Caller fetches limit+1 to detect
-- has_more without a COUNT.
--
-- Search:
--   - query_like: pre-wrapped %term% used against supplier.name.
--   - query_seq_exact: integer value of q when q is all-digits.
--     Matches both supplier_sequence_number AND b.id so users can
--     paste either an internal id or the supplier's reference number
--     and get a direct hit (FE round-2 ask).
--   All-NULL = "no search".
--
-- NOTE: total exact-match deferred — sqlc nullable-decimal narg
-- limitation (see bill.sql for details). Will land in a follow-up
-- once we move to a nullable-decimal-aware pattern.
--
-- state_filter: NULL = "any non-deleted" (state >= 0). Otherwise
-- exact match on b.state. Negative values are filtered out at the
-- request layer (mapped to NULL before binding).
--
-- Each sqlc.narg() literal appears once, in the `p` derived table,
-- so the file stays under plsql:S1192's repeated-literal threshold.
select b.*
	from purchase_bill as b
	join store on store.id = b.store_id
	join company on company.id = store.company_id
	join user on user.id = ? and company.id = user.company_id
	left join supplier on supplier.id = b.supplier_id
	cross join (select CAST(sqlc.narg('state_filter')    AS SIGNED)      AS sf,
	                   CAST(sqlc.narg('query_like')      AS CHAR(255))   AS q,
	                   CAST(sqlc.narg('query_seq_exact') AS UNSIGNED)    AS qe,
	                   CAST(sqlc.narg('cursor_date')     AS DATETIME(6)) AS cd,
	                   CAST(sqlc.narg('cursor_id')       AS UNSIGNED)    AS ci) p
	where b.state >= 0
	  and (p.sf is null or b.state = p.sf)
	  and (
	        (p.q is null and p.qe is null)
	     or supplier.name like p.q
	     or CAST(b.supplier_sequence_number AS CHAR) = CAST(p.qe AS CHAR)
	     or CAST(b.id AS CHAR)                       = CAST(p.qe AS CHAR)
	  )
	  and (
	        p.cd is null
	     or b.effective_date < p.cd
	     or (b.effective_date = p.cd and b.id < p.ci)
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

-- name: SoftDeletePurchaseBill :execresult
UPDATE purchase_bill SET state = -1 WHERE id = ?;
