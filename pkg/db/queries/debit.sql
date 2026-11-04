-- name: GetDebitNoteByRequestIdentity :one
SELECT id, bill_id, request_fingerprint, request_fingerprint_version FROM debit_note WHERE pos_id = ? AND bill_request_id = ?;

-- name: InsertDebitNote :execresult
INSERT INTO debit_note (bill_id, state, note, amount, lines_total, invoice_uuid, pos_id, bill_request_id, request_fingerprint, request_fingerprint_version)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertDebitNoteLine :exec
INSERT INTO debit_note_line (debit_note_id, name, quantity, price, vat)
VALUES (?, ?, ?, ?, ?);
