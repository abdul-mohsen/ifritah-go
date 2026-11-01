-- +goose Up
-- EGS chain reservation (B4). The committed chain is already on branch_zatca_config:
-- last_icv and previous_invoice_hash. A reservation holds one document's ICV while the document
-- is built and signed. The four reservation columns are set together or not at all, and only the
-- owner of a reservation can commit or release it (pkg/egschain). The columns are appended,
-- because sqlc appends them too.
ALTER TABLE branch_zatca_config
  ADD COLUMN icv_reservation_key VARCHAR(255) NULL,
  ADD COLUMN icv_reservation_token VARCHAR(64) NULL,
  ADD COLUMN reserved_icv BIGINT UNSIGNED NULL,
  ADD COLUMN reservation_lease_expires_at DATETIME NULL,
  ADD CONSTRAINT chk_bzc_icv_reservation CHECK (
    (icv_reservation_key IS NULL AND icv_reservation_token IS NULL AND reserved_icv IS NULL AND reservation_lease_expires_at IS NULL)
    OR (icv_reservation_key IS NOT NULL AND icv_reservation_token IS NOT NULL AND reserved_icv IS NOT NULL AND reservation_lease_expires_at IS NOT NULL)
  );

-- +goose Down
ALTER TABLE branch_zatca_config
  DROP CHECK chk_bzc_icv_reservation,
  DROP COLUMN reservation_lease_expires_at,
  DROP COLUMN reserved_icv,
  DROP COLUMN icv_reservation_token,
  DROP COLUMN icv_reservation_key;
