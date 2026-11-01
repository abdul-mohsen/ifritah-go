-- +goose Up
-- Bill identity for the sale command (B5). A POS retries a sale with the same bill_request_id,
-- so the pair (pos_id, bill_request_id) is unique in the tenant database. The fingerprint of
-- the request decides between a replay and a conflict. Existing bills keep NULL identity, and
-- MySQL allows any number of NULL pairs in a unique key. The columns are appended, not placed
-- after invoice_uuid, because sqlc expands b.* in table order and appends the same way.
ALTER TABLE bill
  ADD COLUMN pos_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL,
  ADD COLUMN bill_request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN request_fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN request_fingerprint_version SMALLINT UNSIGNED NOT NULL DEFAULT 1,
  ADD UNIQUE KEY uq_bill_pos_request (pos_id, bill_request_id),
  ADD CONSTRAINT chk_bill_request_fingerprint CHECK (request_fingerprint IS NULL OR request_fingerprint REGEXP '^[0-9a-f]{64}$');

-- +goose Down
ALTER TABLE bill
  DROP CHECK chk_bill_request_fingerprint,
  DROP INDEX uq_bill_pos_request,
  DROP COLUMN request_fingerprint_version,
  DROP COLUMN request_fingerprint,
  DROP COLUMN bill_request_id,
  DROP COLUMN pos_id;
