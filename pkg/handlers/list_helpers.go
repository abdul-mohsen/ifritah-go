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

// foldDigits maps Arabic-Indic (٠-٩) and Extended Arabic-Indic (۰-۹)
// digits to ASCII 0-9. Other runes are passed through unchanged.
func foldDigits(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= '\u0660' && r <= '\u0669': // ٠-٩
			out = append(out, '0'+(r-'\u0660'))
		case r >= '\u06F0' && r <= '\u06F9': // ۰-۹
			out = append(out, '0'+(r-'\u06F0'))
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

// digitsOnly returns only the ASCII digits in s after folding Arabic
// digit forms. Used to normalize phone-number search input on the
// query side; the SQL strips the same set on the column side via
// REGEXP_REPLACE.
func digitsOnly(s string) string {
	folded := foldDigits(s)
	out := make([]byte, 0, len(folded))
	for i := 0; i < len(folded); i++ {
		if folded[i] >= '0' && folded[i] <= '9' {
			out = append(out, folded[i])
		}
	}
	return string(out)
}

// minPhoneSearchDigits is the minimum digit count below which we do
// not treat a query as a phone-number search. Below this threshold
// the digit string would match too broadly (e.g. "12" appears in
// thousands of phone numbers).
const minPhoneSearchDigits = 4

// buildBillSearchParams splits a free-text query into a name LIKE
// and a digits-only phone fragment. Either may be nil.
//
//   - phoneDigits: %digits% wrapper, set when the query yields at
//     least minPhoneSearchDigits digits after folding/stripping.
//     Compared in SQL against REGEXP_REPLACE(phone, '[^0-9]+', '')
//     so input format does not have to match storage format.
//   - nameLike:    %query% wrapper, set when the query contains any
//     non-digit (i.e. it could be a name) OR when no phoneDigits
//     was produced (so a short numeric query still finds a name).
//
// The previous helper silently ParseUint'd the entire query into a
// sequence_number match, which made phone-shaped searches like
// "0512345678" return invoice #512345678. That heuristic is gone;
// sequence-number search now requires a dedicated request field.
func buildBillSearchParams(query *string) (nameLike *string, phoneDigits *string) {
	if query == nil || *query == "" {
		return nil, nil
	}
	q := *query
	digits := digitsOnly(q)
	if len(digits) >= minPhoneSearchDigits {
		p := "%" + digits + "%"
		phoneDigits = &p
	}
	hasNonDigit := false
	for _, r := range foldDigits(q) {
		if r < '0' || r > '9' {
			if r != ' ' && r != '-' && r != '+' && r != '(' && r != ')' {
				hasNonDigit = true
				break
			}
		}
	}
	if hasNonDigit || phoneDigits == nil {
		s := "%" + q + "%"
		nameLike = &s
	}
	return nameLike, phoneDigits
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
