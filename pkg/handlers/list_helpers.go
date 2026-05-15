package handlers

import (
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
	"slices"
	"strconv"
	"strings"
	"time"
)

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

// foldDigits maps Arabic-Indic and Extended Arabic-Indic digits to ASCII 0-9.
func foldDigits(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= '\u0660' && r <= '\u0669':
			out = append(out, '0'+(r-'\u0660'))
		case r >= '\u06F0' && r <= '\u06F9':
			out = append(out, '0'+(r-'\u06F0'))
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

// digitsOnly returns ASCII digits in s after folding Arabic forms.
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

const minPhoneSearchDigits = 4

// buildBillSearchParams splits a free-text query into a name LIKE and a
// digits-only phone fragment for the bill list. See PR #32 for context.
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

// buildPlainPrefixFilter returns 'value%' for a non-empty trimmed input,
// or nil. Used for typed-filter chips that match literal alnum tokens.
func buildPlainPrefixFilter(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	out := s + "%"
	return &out
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
func decodeBillCursor(c pagination.Cursor) (date *time.Time, id *uint64, isCredit *int32, ok bool) {
	if len(c.K) < 3 {
		return nil, nil, nil, true
	}
	date = parseRFC3339Cursor(c.K[0])
	id = decodeCursorUint64(c.K[1])
	isCredit = decodeCursorInt32(c.K[2])
	if date == nil || id == nil || isCredit == nil {
		return nil, nil, nil, false
	}
	return date, id, isCredit, true
}
