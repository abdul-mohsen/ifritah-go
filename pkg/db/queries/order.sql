-- name: GetOrders :many
-- Keyset (created_at DESC, id DESC). Typed filters AND with query_like.
SELECT o.id, o.sequence_number, o.client_id, COALESCE(o.customer_name, '') AS customer_name,
       o.store_id, o.status, o.total, COALESCE(o.note, '') AS note,
       o.created_by, o.created_at, o.updated_at,
       COALESCE(c.name, o.customer_name, '') AS client_name
FROM orders o
LEFT JOIN client c ON c.id = o.client_id
CROSS JOIN (SELECT CAST(sqlc.narg('company_id')           AS UNSIGNED)    AS cid,
                   CAST(sqlc.narg('query_like')           AS CHAR(255))   AS q,
                   CAST(sqlc.narg('filter_seq_prefix')    AS CHAR(32))    AS fs,
                   CAST(sqlc.narg('filter_phone_prefix')  AS CHAR(20))    AS fp,
                   CAST(sqlc.narg('cursor_created_at')    AS DATETIME(6)) AS cca,
                   CAST(sqlc.narg('cursor_id')            AS UNSIGNED)    AS ci) p
WHERE o.store_id IN (SELECT id FROM store WHERE company_id = p.cid)
  AND (p.q IS NULL
       OR o.sequence_number LIKE p.q COLLATE utf8mb4_unicode_ci
       OR o.customer_name LIKE p.q COLLATE utf8mb4_unicode_ci)
  AND (p.fs IS NULL OR o.sequence_number LIKE p.fs COLLATE utf8mb4_unicode_ci)
  AND (p.fp IS NULL OR c.phone           COLLATE utf8mb4_unicode_ci LIKE p.fp)
  AND (p.cca IS NULL
       OR o.created_at < p.cca
       OR (o.created_at = p.cca AND o.id < p.ci))
ORDER BY o.created_at DESC, o.id DESC
LIMIT ?;

-- name: GetOrderByID :one
SELECT o.id, o.sequence_number, o.client_id, COALESCE(o.customer_name, '') AS customer_name,
       o.store_id, o.status, o.total, COALESCE(o.note, '') AS note,
       o.created_by,
       DATE_FORMAT(o.created_at, '%Y-%m-%dT%H:%i:%s') AS created_at,
       DATE_FORMAT(o.updated_at, '%Y-%m-%dT%H:%i:%s') AS updated_at,
       COALESCE(c.name, o.customer_name, '') AS client_name,
       COALESCE(s.name, '') AS store_name
FROM orders o
LEFT JOIN client c ON c.id = o.client_id
LEFT JOIN store s ON s.id = o.store_id
WHERE o.id = ?;

-- name: GetOrderItemsByOrderID :many
SELECT id, part_id, part_name, quantity, unit_price, line_total
FROM order_items
WHERE order_id = ?
ORDER BY id ASC;

-- name: GetOrderForCompany :one
SELECT o.id FROM orders o
WHERE o.id = ? AND o.store_id IN (SELECT id FROM store WHERE company_id = ?);

-- name: CountClientForOrder :one
SELECT COUNT(*) FROM client WHERE id = ? AND is_deleted = 0;

-- name: GetClientNameForOrder :one
SELECT COALESCE(name, '') FROM client WHERE id = ?;

-- name: CountStoreForOrder :one
SELECT COUNT(*) FROM store WHERE id = ? AND company_id = ?;

-- name: CountOrderBySequence :one
SELECT COUNT(*) FROM orders WHERE sequence_number = ?;

-- name: CountOrderBySequenceExcludingID :one
SELECT COUNT(*) FROM orders WHERE sequence_number = ? AND id != ?;

-- name: CreateOrder :execresult
INSERT INTO orders (sequence_number, client_id, customer_name, store_id,
                    status, total, note, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateOrderItem :exec
INSERT INTO order_items (order_id, part_id, part_name, quantity, unit_price, line_total)
VALUES (?, ?, ?, ?, ?, ?);

-- name: UpdateOrder :exec
UPDATE orders SET sequence_number = ?, client_id = ?, customer_name = ?,
       store_id = ?, status = ?, total = ?, note = ?
WHERE id = ?;

-- name: DeleteOrderItemsByOrderID :exec
DELETE FROM order_items WHERE order_id = ?;

-- name: DeleteOrderByID :execresult
DELETE FROM orders WHERE id = ?;
