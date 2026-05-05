package handlers

import (
	"database/sql"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
	"slices"
	"strconv"
	"time"
)

// nullInt64FromInt32Ptr wraps a nullable int32 sentinel into the
// sql.NullInt64 shape that sqlc emits for narg() params wrapped in
// `CAST(... AS SIGNED)`. nil → invalid, otherwise widened.
func nullInt64FromInt32Ptr(v *int32) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// nullInt64FromUint64Ptr wraps a nullable uint64 sentinel into
// sql.NullInt64. The values we pass here (cursor ids, sequence
// numbers) are real-world bounded well below int64 max, so the
// reinterpret-cast is safe.
func nullInt64FromUint64Ptr(v *uint64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// nullInt64FromAny accepts the cursor's raw decoded value (which can
// arrive as float64, int64, json.Number or untyped nil) and produces
// the sql.NullInt64 the generated bill query expects for
// `cursor_is_credit`. Any decode failure becomes an invalid Null,
// which the SQL treats as "no cursor".
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

// listRequestFromPagination adapts the existing model.PaginationRequest
// into the shared pagination.ListRequest. Used by handlers that don't
// need extra resource-specific filters (client/supplier/product etc.).
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

// cursorIDOnly decodes the trailing id from a cursor for resources
// whose sort key is just `id DESC`. Returns nil when the cursor is
// empty (first page) or when the id cannot be parsed.
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

// cursorDateAndID decodes a (date, id) keyset cursor. Returns (nil,nil)
// for first-page cursors and (nonnil, nonnil) for subsequent pages.
// Returns (nil, nil, false) when the cursor is malformed and the caller
// should respond 400.
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

// applyListSort and the per-type comparator helpers were removed:
// sort over the current page is now handled by the frontend. The
// backend returns rows in the canonical keyset order only.

// ── Shared filter / cursor helpers ──────────────────────────────────────────
//
// These factor out small repetitive blocks from list handlers so the
// per-handler cognitive complexity stays under Sonar's S3776 ceiling
// without changing observable behaviour.

// buildLikeAndDigitsExact splits a free-text query into the two
// sentinels every list endpoint that supports digits-exact search
// uses: a `%term%` LIKE wrapper and an integer value when the query
// is all digits. Returns (nil, nil) when the query is absent or
// empty. The integer is non-zero — a leading-zero / negative input
// degrades to a LIKE-only search.
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

// nonNegativeStateFilter normalises the FE-supplied `state` filter:
// nil or negative → return nil ("any state, soft-deletes excluded
// by the SQL"); >= 0 → return a non-aliased pointer to the value.
func nonNegativeStateFilter(state *int32) *int32 {
	if state == nil || *state < 0 {
		return nil
	}
	v := *state
	return &v
}

// allStoresAllowed returns true iff every requested store id is in
// the caller's accessible set. Used by bill / purchase_bill list to
// gate scope-jumping requests with a 400.
func allStoresAllowed(reqIDs, allowed []int) bool {
	for _, id := range reqIDs {
		if !slices.Contains(allowed, id) {
			return false
		}
	}
	return true
}

// decodeCursorUint64 unwraps an opaque cursor key that was JSON-encoded
// as a number (typed as float64 / int64 / json.Number after the
// round trip). Returns nil on any zero / invalid value so the caller
// can treat it as "no cursor set".
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

// decodeCursorInt32 mirrors decodeCursorUint64 for signed-int32
// cursor keys (e.g. bill.is_credit). Returns nil when the input is
// nil; otherwise always returns a value (zero is a legitimate key
// for boolean-shaped columns).
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

// parseRFC3339Cursor decodes the leading time-keyed cursor key. The
// FE round-trips the date as a string so we accept both Nano and
// non-Nano forms (some legacy paths drop the fractional seconds).
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

// decodeBillCursor decodes the (effective_date, id, is_credit) triple.
// Returns ok=false on a half-decoded cursor so the caller can map it
// to a 400. ok=true with all-nil out-params is the first-page case.
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
