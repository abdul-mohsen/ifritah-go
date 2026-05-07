-- name: GetAllPurchaseBill :many
-- Keyset pagination on (effective_date DESC, id DESC). Caller fetches
-- limit+1 to detect has_more.
-- query_seq_exact matches either supplier_sequence_number or b.id.
select b.*
	from purchase_bill as b
	join store on store.id = b.store_id
	join company on company.id = store.company_id
	join user on user.id = ? and company.id = user.company_id
	left join supplier on supplier.id = b.supplier_id
	cross join (select CAST(sqlc.narg('state_filter')               AS SIGNED)      AS sf,
	                   CAST(sqlc.narg('query_like') AS CHAR(255))   AS q,
	                   CAST(sqlc.narg('query_seq_exact')            AS UNSIGNED)    AS qe,
	                   CAST(sqlc.narg('seq_prefix') AS CHAR(32))    AS fs,
	                   CAST(sqlc.narg('supplier_seq_prefix') AS CHAR(64))    AS fss,
	                   CAST(sqlc.narg('phone_prefix') AS CHAR(20))    AS fp,
	                   CAST(sqlc.narg('cursor_date')                AS DATETIME(6)) AS cd,
	                   CAST(sqlc.narg('cursor_id')                  AS UNSIGNED)    AS ci) p
	where b.state >= 0
	  and (p.sf is null or b.state = p.sf)
	  and (
	        (p.q is null and p.qe is null)
	     or supplier.name COLLATE utf8mb4_unicode_ci LIKE p.q
	     or CAST(b.supplier_sequence_number AS CHAR) = CAST(p.qe AS CHAR)
	     or CAST(b.id AS CHAR)                       = CAST(p.qe AS CHAR)
	  )
	  and (p.fs  is null or CAST(b.sequence_number AS CHAR) COLLATE utf8mb4_unicode_ci LIKE p.fs)
	  and (p.fss is null or CAST(b.supplier_sequence_number AS CHAR) COLLATE utf8mb4_unicode_ci LIKE p.fss)
	  and (p.fp  is null or supplier.phone_number COLLATE utf8mb4_unicode_ci LIKE p.fp)
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
