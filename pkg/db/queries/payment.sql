-- name: GetCurrencyIDByCode :one
SELECT id FROM currency WHERE code = ?;

-- name: GetBillTotal :one
SELECT total FROM bill WHERE id = ?;

-- name: InsertBillPayment :exec
INSERT INTO bill_payment (bill_id, date, paid_at, amount, currency_id, payment_method, recorded_by) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: DeleteBillPayments :exec
DELETE FROM bill_payment WHERE bill_id = ?;
