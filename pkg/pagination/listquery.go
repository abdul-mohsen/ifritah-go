package pagination

import (
	"errors"
	"strings"
)

// MaxLimit is the BE hard cap on page size. The FE caps itself at 50
// per the SEARCH_API.md contract — this 100 is a defence-in-depth cap
// for any non-FE caller (curl, scripts, integration tests).
const MaxLimit = 100

// DefaultLimit is the page size used when a request omits `limit`.
const DefaultLimit = 25

// ListRequest is the shared request shape for every cursor-paginated
// list endpoint. The legacy keys (PageNumber/PageSize) are accepted
// for one release so existing FE code paths keep working while the
// envelope rolls out — they are ignored once Cursor is non-empty.
//
// Resource-specific filters (StoreIds, VoucherType, etc.) live on the
// per-resource request types; they are not part of this shared shape
// because each list has its own filter columns.
type ListRequest struct {
	// New, preferred wire keys.
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
	Sort   string `json:"sort"`

	// Search term. Honored server-side per FE §4 (Search-on-list).
	Query *string `json:"query"`

	// Legacy keys — kept so this PR is wire-compatible with clients
	// that haven't switched to cursors yet. Ignored when Cursor != "".
	PageNumber int `json:"page_number"`
	PageSize   int `json:"page_size"`
}

// EffectiveLimit clamps the requested limit into [1, MaxLimit] and
// applies DefaultLimit when neither limit nor page_size is set.
func (r *ListRequest) EffectiveLimit() int {
	n := r.Limit
	if n <= 0 {
		n = r.PageSize
	}
	if n <= 0 {
		return DefaultLimit
	}
	if n > MaxLimit {
		return MaxLimit
	}
	return n
}

// ErrLimitTooLarge is returned when a caller asks for more rows than
// MaxLimit. Per the FE contract this is a 400 — we don't silently
// clamp because that would hide bugs in client pagination code.
var ErrLimitTooLarge = errors.New("limit exceeds max")

// ErrSortMismatch is returned when the cursor's S field disagrees
// with the request's Sort. The seek predicate would walk through the
// wrong index, so we reject rather than silently re-sort.
var ErrSortMismatch = errors.New("cursor sort spec does not match request sort")

// Validate checks limit + cursor sort consistency. expectedSort is the
// canonical sort spec for the resource (e.g. "-effective_date" for
// invoices, "-id" for catalogue tables). If the request omits sort
// the resource default is assumed.
func (r *ListRequest) Validate(expectedSort string) error {
	if r.Limit > MaxLimit {
		return ErrLimitTooLarge
	}
	if r.Cursor == "" {
		return nil
	}
	c, err := Decode(r.Cursor)
	if err != nil {
		return err
	}
	wantSort := r.Sort
	if wantSort == "" {
		wantSort = expectedSort
	}
	if c.S != "" && c.S != wantSort {
		return ErrSortMismatch
	}
	return nil
}

// DecodedCursor returns the parsed cursor or the zero Cursor when the
// request has no cursor. Call Validate first — this method does not
// re-check sort consistency.
func (r *ListRequest) DecodedCursor() (Cursor, error) {
	return Decode(r.Cursor)
}

// SortSpec returns the resolved sort spec for the request. When the
// request omits sort the resource default is returned.
func (r *ListRequest) SortSpec(defaultSort string) string {
	if r.Sort == "" {
		return defaultSort
	}
	return r.Sort
}

// SortColumn parses a sort spec like "-effective_date" into the bare
// column name and the direction. A leading '-' means DESC; otherwise
// ASC. This is intentionally tiny and only handles the single-column
// specs the FE actually emits — multi-column sort isn't in the
// contract yet.
func SortColumn(spec string) (col string, desc bool) {
	if strings.HasPrefix(spec, "-") {
		return spec[1:], true
	}
	return spec, false
}

// Envelope is the new response shape. Generic on the item type so
// each handler keeps its strongly-typed slice without an interface{}
// hop. Keep the JSON tags identical to the FE's tryDecodeEnvelope
// detection rule — adding/renaming a field here breaks the contract.
type Envelope[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor"`
	PrevCursor string `json:"prev_cursor"`
	HasMore    bool   `json:"has_more"`
}

// BuildEnvelope applies the +1 row trick: callers fetch limit+1 rows
// and pass them to this helper. If we got back limit+1, the extra row
// is dropped and HasMore is set to true.
//
// keyFn extracts the keyset tuple for the *last* item kept on the
// page; the result becomes NextCursor. If keyFn returns a nil/empty
// slice, NextCursor stays empty (signals "no more pages").
//
// PrevCursor mirroring is left to the caller because not every list
// supports backward paging — the FE today only walks forward via
// next_cursor, so most resources can pass an empty string.
func BuildEnvelope[T any](
	rows []T,
	limit int,
	sortSpec string,
	keyFn func(T) []any,
) Envelope[T] {
	hasMore := false
	if len(rows) > limit {
		rows = rows[:limit]
		hasMore = true
	}
	env := Envelope[T]{Items: rows, HasMore: hasMore}
	// Always serialize Items as [] not null even when empty — Go would
	// otherwise emit `"items": null`, which trips up some FE decoders.
	if env.Items == nil {
		env.Items = []T{}
	}
	if hasMore && len(rows) > 0 && keyFn != nil {
		key := keyFn(rows[len(rows)-1])
		if len(key) > 0 {
			env.NextCursor = Encode(Cursor{
				K: key,
				S: sortSpec,
				D: DirectionAfter,
			})
		}
	}
	return env
}
