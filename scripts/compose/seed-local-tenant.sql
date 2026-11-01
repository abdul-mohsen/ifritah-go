-- Local development only. Sets company 1 with its tenant key after the goose migrations, so the API can read the key.
-- Deployment writes this row for real tenants. The tenant key is the database name, the value zatca_master backfills
-- from db_name.
INSERT INTO company (id, name, vat_number, tenant_key)
VALUES (1, 'Local', '300000000000003', DATABASE())
ON DUPLICATE KEY UPDATE tenant_key = VALUES(tenant_key);
