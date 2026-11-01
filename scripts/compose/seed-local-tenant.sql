-- Local development only. Writes the company row once, after the goose migrations, so the API can read its
-- tenant key (company.tenant_key). Deployment writes this row for real tenants. The tenant key is the database
-- name, the same value zatca_master backfills from db_name.
INSERT INTO company (name, vat_number, tenant_key)
SELECT 'Local', '300000000000003', DATABASE()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM company);
