-- name: LockBillForNote :one
SELECT id, state, branch_id, store_id, total FROM bill WHERE id = ? FOR UPDATE;

-- name: GetBillStoreID :one
SELECT store_id FROM bill WHERE id = ?;
