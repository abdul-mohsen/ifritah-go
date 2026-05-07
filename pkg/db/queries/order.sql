-- name: GetOrders :many
SELECT o.id, o.sequence_number, o.client_id, COALESCE(o.customer_name, '') AS customer_name,
       o.store_id, o.status, o.total, COALESCE(o.note, '') AS note,
       o.created_by, o.created_at, o.updated_at,
       COALESCE(c.name, o.customer_name, '') AS client_name
FROM orders o
LEFT JOIN client c ON c.id = o.client_id
WHERE o.store_id IN (SELECT id FROM store WHERE company_id = sqlc.narg('company_id'))
  AND (sqlc.narg('query_like') IS NULL
       OR o.sequence_number LIKE sqlc.narg('query_like')
       OR o.customer_name   LIKE sqlc.narg('query_like'))
  AND (sqlc.narg('seq_prefix')   IS NULL OR o.sequence_number LIKE sqlc.narg('seq_prefix'))
  AND (sqlc.narg('phone_prefix') IS NULL OR c.phone           LIKE sqlc.narg('phone_prefix'))
  AND (sqlc.narg('cursor_created_at') IS NULL
       OR o.created_at < sqlc.narg('cursor_created_at')
       OR (o.created_at = sqlc.narg('cursor_created_at') AND o.id < sqlc.narg('cursor_id')))
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
