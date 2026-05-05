package handlers

import (
	"database/sql"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
	"slices"
	"strconv"
	"time"
)

// nullInt64FromInt32Ptr wraps int32 → sql.NullInt64 for sqlc narg() params.
func nullInt64FromInt32Ptr(v *int32) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// nullInt64FromUint64Ptr wraps uint64 → sql.NullInt64 for sqlc narg() params.
func nullInt64FromUint64Ptr(v *uint64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// nullInt64FromAny converts cursor values to sql.NullInt64.
func nullInt64FromAny(v any) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	switch t := v.(type) {
	case int64:
		return sql.NullInt64{Int64: t, Valid: true}
	case float64:
		return sql.NullInt64{Int64: int64(t), Valid: true}
	case int:
		return sql.NullInt64{Int64: int64(t), Valid: true}
	case int32:
		return sql.NullInt64{Int64: int64(t), Valid: true}
	}
	if p := decodeCursorUint64(v); p != nil {
		return sql.NullInt64{Int64: int64(*p), Valid: true}
	}
	return sql.NullInt64{}
}

// listRequestFromPagination converts model.PaginationRequest → pagination.ListRequest.
func listRequestFromPagination(p model.PaginationRequest) pagination.ListRequest {
	page := p.Page
	if page == 0 {
		page = p.PageNumber
	}
	return pagination.ListRequest{
		Limit:      int(p.Limit),
		Cursor:     p.Cursor,
		Sort:       p.Sort,
		Dir:        p.Dir,
		Query:      p.Query,
		PageNumber: int(page),
		PageSize:   int(p.PageSize),
	}
}

// cursorIDOnly extracts the ID from a single-column cursor; returns nil if empty/invalid.
func cursorIDOnly(c pagination.Cursor) *uint64 {
	if len(c.K) == 0 {
		return nil
	}
	id, ok := c.LastID()
	if !ok || id <= 0 {
		return nil
	}
	u := uint64(id)
	return &u
}

// cursorDateAndID extracts (date, id) from a two-column keyset cursor.
func cursorDateAndID(c pagination.Cursor) (*time.Time, *uint64, bool) {
	if len(c.K) == 0 {
		return nil, nil, true
	}
	if len(c.K) < 2 {
		return nil, nil, false
	}
	var t *time.Time
	if dStr, ok := c.K[0].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, dStr); err == nil {
			t = &parsed
		} else if parsed, err := time.Parse(time.RFC3339, dStr); err == nil {
			t = &parsed
		}
	}
	id, ok := c.LastID()
	if t == nil || !ok || id <= 0 {
		return nil, nil, false
	}
	u := uint64(id)
	return t, &u, true
}

// Shared filter / cursor helpers for list endpoints.

// buildLikeAndDigitsExact returns (like, digits) from query: LIKE %term% and parsed int.
func buildLikeAndDigitsExact(query *string) (like *string, digits *uint64) {
	if query == nil || *query == "" {
		return nil, nil
	}
	s := "%" + *query + "%"
	like = &s
	if n, err := strconv.ParseUint(*query, 10, 64); err == nil && n > 0 {
		digits = &n
	}
	return like, digits
}

// nonNegativeStateFilter returns nil for negative state; otherwise returns the value.
func nonNegativeStateFilter(state *int32) *int32 {
	if state == nil || *state < 0 {
		return nil
	}
	v := *state
	return &v
}

// allStoresAllowed checks if all requested store IDs are in the allowed set.
func allStoresAllowed(reqIDs, allowed []int) bool {
	for _, id := range reqIDs {
		if !slices.Contains(allowed, id) {
			return false
		}
	}
	return true
}

// decodeCursorUint64 unwraps a JSON-encoded cursor key (>0 uint64); returns nil if invalid.
func decodeCursorUint64(v any) *uint64 {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case float64:
		if x > 0 {
			u := uint64(x)
			return &u
		}
	case int64:
		if x > 0 {
			u := uint64(x)
			return &u
		}
	default:
		if n, ok := v.(interface{ Int64() (int64, error) }); ok {
			if i, err := n.Int64(); err == nil && i > 0 {
				u := uint64(i)
				return &u
			}
		}
	}
	return nil
}

// decodeCursorInt32 unwraps a JSON-encoded int32 cursor key; returns nil if invalid.
func decodeCursorInt32(v any) *int32 {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case float64:
		i := int32(x)
		return &i
	case int64:
		i := int32(x)
		return &i
	default:
		if n, ok := v.(interface{ Int64() (int64, error) }); ok {
			if i, err := n.Int64(); err == nil {
				ii := int32(i)
				return &ii
			}
		}
	}
	return nil
}

// parseRFC3339Cursor decodes a time-keyed cursor (RFC3339 format).
func parseRFC3339Cursor(v any) *time.Time {
	dStr, ok := v.(string)
	if !ok || dStr == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339Nano, dStr); err == nil {
		return &t
	}
	if t, err := time.Parse(time.RFC3339, dStr); err == nil {
		return &t
	}
	return nil
}

// decodeBillCursor decodes the (date, id, is_credit) keyset cursor.
func decodeBillCursor(c pagination.Cursor) (date *time.Time, id *uint64, isCredit any, ok bool) {
	if len(c.K) < 3 {
		return nil, nil, nil, true
	}
	date = parseRFC3339Cursor(c.K[0])
	id = decodeCursorUint64(c.K[1])
	if v := decodeCursorInt32(c.K[2]); v != nil {
		isCredit = *v
	}
	if date == nil || id == nil || isCredit == nil {
		return nil, nil, nil, false
	}
	return date, id, isCredit, true
}
