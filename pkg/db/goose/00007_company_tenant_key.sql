-- +goose Up
-- The tenant key identifies this database's tenant. It is the value of zatca_master.tenant.tenant_key,
-- and deployment writes it. The backend only reads it, and it refuses to start without a valid value.
-- The binary collation compares the key exactly, as zatca_master does.
ALTER TABLE company
  ADD COLUMN tenant_key VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE company
  DROP COLUMN tenant_key;
