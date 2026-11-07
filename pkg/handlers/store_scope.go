package handlers

import (
	"context"

	db "ifritah/web-service-gin/pkg/db/gen"
)

func (h *handler) listStoresForScope(ctx context.Context, userID int64, role string) ([]db.ListStoresForUserScopeRow, error) {
	var roleArg interface{}
	if role != "" {
		roleArg = role
	}
	return h.queries.ListStoresForUserScope(ctx, db.ListStoresForUserScopeParams{
		UserID: int32(userID),
		Role:   roleArg,
	})
}

func storesFromScopeRows(rows []db.ListStoresForUserScopeRow) []Store {
	stores := make([]Store, 0, len(rows))
	for _, row := range rows {
		store := Store{
			Id:   int(row.ID),
			Name: row.Name,
		}
		if row.Addressid != nil {
			addressID := int(*row.Addressid)
			store.AddressId = &addressID
		}
		stores = append(stores, store)
	}
	return stores
}
