-- +goose Up
-- EGS chain reservation (B4). Backend Goose owns this tenant-schema DDL: the committed chain is
-- already on branch_zatca_config (last_icv and previous_invoice_hash), and this migration adds
-- the four reservation columns. queue.SQLChain in the external ZATCA daemon owns the runtime
-- reservation protocol: booking and lease takeover, plus owner-guarded release and commit. The
-- four reservation columns are set together or not at all; backend application code does not
-- execute this runtime protocol. The columns are appended for existing schema compatibility.
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
