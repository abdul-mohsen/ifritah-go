-- name: GetAllPurchaseBill :many
SELECT b.*
FROM purchase_bill AS b
JOIN store ON store.id = b.store_id
JOIN company ON company.id = store.company_id
JOIN user ON user.id = ? AND company.id = user.company_id
LEFT JOIN supplier ON supplier.id = b.supplier_id
WHERE b.state >= 0
  AND (sqlc.narg('state_filter') IS NULL OR b.state = sqlc.narg('state_filter'))
  AND (
        (sqlc.narg('query_like') IS NULL AND sqlc.narg('query_seq_exact') IS NULL)
     OR supplier.name LIKE sqlc.narg('query_like')
     OR b.supplier_sequence_number = sqlc.narg('query_seq_exact')
     OR b.id                       = sqlc.narg('query_seq_exact')
  )
  AND (sqlc.narg('seq_prefix')          IS NULL OR b.sequence_number_str          LIKE sqlc.narg('seq_prefix'))
  AND (sqlc.narg('supplier_seq_prefix') IS NULL OR b.supplier_sequence_number_str LIKE sqlc.narg('supplier_seq_prefix'))
  AND (sqlc.narg('phone_prefix')        IS NULL OR supplier.phone_number          LIKE sqlc.narg('phone_prefix'))
  AND (sqlc.narg('cursor_date') IS NULL
       OR b.effective_date < sqlc.narg('cursor_date')
       OR (b.effective_date = sqlc.narg('cursor_date') AND b.id < sqlc.narg('cursor_id')))
ORDER BY b.effective_date DESC, b.id DESC
LIMIT ?;

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

-- name: RefreshPurchaseBillTotals :exec
UPDATE purchase_bill
SET
   total_before_vat = COALESCE((SELECT SUM(pbp.total_before_vat) FROM purchase_bill_product AS pbp WHERE pbp.bill_id = sqlc.arg('target_bill_id')), 0),
   total_vat = COALESCE((SELECT SUM(pbp.vat_total) FROM purchase_bill_product AS pbp WHERE pbp.bill_id = sqlc.arg('target_bill_id')), 0),
   total = COALESCE((SELECT SUM(pbp.total_including_vat) FROM purchase_bill_product AS pbp WHERE pbp.bill_id = sqlc.arg('target_bill_id')), 0)
WHERE purchase_bill.id = sqlc.arg('target_id');

-- name: AddAttachmentsPurchaseBill :exec
insert into purchase_bill_attachments  (purchase_bill_id, file_key) values (?, ?);

-- name: SoftDeletePurchaseBill :execresult
UPDATE purchase_bill SET state = -1 WHERE id = ?;
