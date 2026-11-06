# Data Model Impact

This feature removes an operational compatibility path and introduces no new
entities, fields, tables, relationships, or persisted state.

## Preserved Migration State

- Goose's existing version table remains managed by the Goose provider.
- `BaselineVersion` remains `1`.
- The migration files remain `00001_baseline.sql` through
  `00010_zatca_invoice_record.sql`.
- No code writes a version row to adopt an externally created schema.

## Validation Rules

- A fresh tenant database is initialized through the normal migration command.
- The baseline remains the non-rollback boundary.
- Tenant-local privilege validation remains in the existing CI workflow.
