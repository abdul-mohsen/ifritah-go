package handlers

import (
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

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

// applyListSort runs sort.SliceStable on items using a per-resource
// comparator. Each handler defines a `cmp` callback that returns the
// 3-way comparison for the named key plus an `ok` flag — when the key
// is unknown the caller's existing order is preserved (ok=false makes
// the less func a no-op for every pair).
//
// Direction: dir is "asc" (default) or "desc" (case-insensitive).
//
// The FE table-sort UI sends a (sort, dir) pair on cursor-empty
// requests so the BE can return a one-shot, server-sorted slice.
// Cursor walks (where order is pinned to the keyset) skip the sort
// path because the FE does not enable sort-headers there.
func applyListSort[T any](items []T, key, dir string, cmp func(a, b T, k string) (int, bool)) {
	if key == "" || len(items) < 2 {
		return
	}
	desc := strings.EqualFold(dir, "desc")
	sort.SliceStable(items, func(i, j int) bool {
		c, ok := cmp(items[i], items[j], key)
		if !ok {
			return false
		}
		if desc {
			return c > 0
		}
		return c < 0
	})
}

// strPtrCmp compares two *string fields, treating nil and "" as equal
// and ordering them before any non-empty value. Case-insensitive
// because the FE search/sort UX is case-insensitive.
func strPtrCmp(a, b *string) int {
	sa := ""
	if a != nil {
		sa = *a
	}
	sb := ""
	if b != nil {
		sb = *b
	}
	return strings.Compare(strings.ToLower(sa), strings.ToLower(sb))
}

// strCmp is the *string-free variant for value-typed string fields.
func strCmp(a, b string) int {
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// decCmp wraps decimal.Decimal.Cmp so the cmp lambdas read uniformly.
func decCmp(a, b decimal.Decimal) int { return a.Cmp(b) }

// boolCmp orders false < true so an "asc" sort surfaces falses first.
func boolCmp(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a:
		return -1
	default:
		return 1
	}
}

// uint64PtrCmp compares two *uint64 fields, ordering nil before any
// non-nil value (FE shows blanks at the top of an asc sort).
func uint64PtrCmp(a, b *uint64) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	case *a < *b:
		return -1
	case *a > *b:
		return 1
	default:
		return 0
	}
}

// int64Cmp / int32Cmp / uint64Cmp avoid the Generics escape just for
// readable handler code.
func int64Cmp(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
func int32Cmp(a, b int32) int { return int64Cmp(int64(a), int64(b)) }
func uint64Cmp(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
func float64Cmp(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
func timeCmp(a, b time.Time) int {
	switch {
	case a.Before(b):
		return -1
	case a.After(b):
		return 1
	default:
		return 0
	}
}
