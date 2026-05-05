// Package pagination implements the keyset/cursor pagination contract
// shared with the frontend. The wire format is documented in the
// afrita-go repo at api_docs/search/SEARCH_API.md §9 and the matching
// FE helper at helpers/cursor.go — keep this file in lockstep with that
// one (struct shape, JSON keys, base64 variant).
//
// Why keyset and not OFFSET:
//   - OFFSET scans + discards N rows on every request. It defeats any
//     covering index and silently skips/duplicates rows under concurrent
//     writes. (Markus Winand, use-the-index-luke.com/no-offset; same
//     reasoning Stripe, Slack and GitHub used when they dropped page
//     numbers from their list APIs.)
//   - Keyset reads the next page by `WHERE (sort_value, id) < (?, ?)`,
//     which the optimizer can serve as a single range scan on the
//     composite index — O(log n) seek + O(limit) read.
package pagination

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// Cursor is the opaque pagination token. Tiny on purpose because it
// travels in URLs and HTML fragments — the FE never inspects fields,
// it just round-trips the encoded string.
type Cursor struct {
	// K is the keyset tuple of the boundary row, ordered the same way
	// as the ORDER BY clause. The trailing element is always the row
	// id (tie-breaker so rows with identical sort values still get a
	// deterministic order).
	//
	// The slice carries interface{} so dates, ints and strings all
	// round-trip without a per-resource cursor type.
	K []any `json:"k"`

	// S is the canonical sort spec this cursor was minted under, e.g.
	// "-effective_date". Empty means "default sort for the resource".
	// The BE rejects a cursor whose S doesn't match the request's sort
	// — otherwise the seek predicate would walk through the wrong
	// index and skip/duplicate rows.
	S string `json:"s,omitempty"`

	// D is the direction relative to K: "after" for next-page, "before"
	// for previous-page. Empty defaults to "after".
	D string `json:"d,omitempty"`
}

// Direction constants. Keep these in sync with helpers/cursor.go on
// the FE so the contract stays symmetric.
const (
	DirectionAfter  = "after"
	DirectionBefore = "before"
)

// Direction returns the cursor direction, defaulting to "after".
func (c Cursor) Direction() string {
	if c.D == "" {
		return DirectionAfter
	}
	return c.D
}

// LastID returns the trailing id component of the keyset tuple. JSON
// numbers come back as either float64 (default) or json.Number when
// the decoder was set with UseNumber(); we accept both because the FE
// uses UseNumber and the BE typically does not.
func (c Cursor) LastID() (int64, bool) {
	if len(c.K) == 0 {
		return 0, false
	}
	switch v := c.K[len(c.K)-1].(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return i, true
	}
	return 0, false
}

// SortValue returns the leading sort-value component of the keyset
// tuple — i.e. K[0] when the cursor has at least 2 elements. For
// id-only cursors (single-element K) it returns nil so callers can
// short-circuit into a pure id-only seek.
func (c Cursor) SortValue() any {
	if len(c.K) < 2 {
		return nil
	}
	return c.K[0]
}

// ErrInvalidCursor is returned when the input is not a valid cursor.
// Callers should map this to HTTP 400 — the user probably hand-edited
// the URL.
var ErrInvalidCursor = errors.New("invalid cursor")

// Encode renders a Cursor as a URL-safe base64 JSON string. Empty
// cursors return "" so callers can treat empty == "first page".
func Encode(c Cursor) string {
	if len(c.K) == 0 {
		return ""
	}
	raw, err := json.Marshal(c)
	if err != nil {
		// Can't fail for our shape. If it ever does we'd rather lose
		// pagination than crash a list page — the caller will treat ""
		// as "no more pages".
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// Decode parses a string previously returned by Encode. An empty
// input is a valid first-page cursor and yields the zero value
// without an error.
//
// We accept both raw (no padding, RFC 4648 §5) and padded base64url
// because some HTTP clients and middlewares re-add the padding.
func Decode(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return Cursor{}, fmt.Errorf("%w: base64: %v", ErrInvalidCursor, err)
		}
	}
	var c Cursor
	dec := json.NewDecoder(bytes.NewReader(raw))
	// UseNumber so big int64 ids don't lose precision in float64.
	dec.UseNumber()
	if err := dec.Decode(&c); err != nil {
		return Cursor{}, fmt.Errorf("%w: json: %v", ErrInvalidCursor, err)
	}
	return c, nil
}
