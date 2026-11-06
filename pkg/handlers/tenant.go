package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/identity"
)

// tenantKeyField names the column in startup errors, so an operator knows where to look.
const tenantKeyField = "company.tenant_key"

// LoadTenantKey returns the tenant's stable key from company.tenant_key. Deployment writes the value,
// and it must equal zatca_master.tenant.tenant_key for this database. Every ZATCA document key starts
// with it, so the server refuses to start unless the key is valid.
func LoadTenantKey(ctx context.Context, queries *db.Queries) (string, error) {
	key, err := queries.GetTenantKey(ctx)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", fmt.Errorf("%s: the company table has no row", tenantKeyField)
	case err != nil:
		return "", fmt.Errorf("%s: %w", tenantKeyField, err)
	case key == "":
		return "", fmt.Errorf("%s is empty: deployment must write the tenant key", tenantKeyField)
	}
	if err := identity.ValidateTenantKey(key); err != nil {
		return "", fmt.Errorf("%s: %w", tenantKeyField, err)
	}
	return key, nil
}
