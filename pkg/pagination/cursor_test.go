package pagination

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	c := Cursor{K: []any{"2026-05-03T00:00:00Z", int64(518)}, S: "-effective_date", D: DirectionAfter}
	enc := Encode(c)
	if enc == "" {
		t.Fatal("expected non-empty cursor")
	}
	got, err := Decode(enc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.S != c.S || got.D != c.D {
		t.Fatalf("metadata mismatch: %+v", got)
	}
	id, ok := got.LastID()
	if !ok || id != 518 {
		t.Fatalf("last id mismatch: got=%d ok=%v", id, ok)
	}
}

func TestDecodeEmptyIsZero(t *testing.T) {
	got, err := Decode("")
	if err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if len(got.K) != 0 {
		t.Fatalf("expected zero cursor, got %+v", got)
	}
}

func TestDecodeMalformedReturnsErrInvalidCursor(t *testing.T) {
	_, err := Decode("!!not-base64!!")
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("want ErrInvalidCursor, got %v", err)
	}
}

func TestDecodeAcceptsPaddedBase64(t *testing.T) {
	// Pre-canned cursor with std-padded base64 (URLEncoding, not RawURL).
	raw, _ := json.Marshal(Cursor{K: []any{int64(7)}, D: DirectionAfter})
	// stdlib URLEncoding adds '=' padding; ensure Decode accepts it.
	std := base64URLEncodeWithPadding(raw)
	got, err := Decode(std)
	if err != nil {
		t.Fatalf("decode padded: %v", err)
	}
	if id, _ := got.LastID(); id != 7 {
		t.Fatalf("padded id mismatch: %d", id)
	}
}

// helper kept local to the test to avoid leaking a padded encoder
// into the package surface.
func base64URLEncodeWithPadding(b []byte) string {
	const tbl = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var out []byte
	n := len(b)
	for i := 0; i < n; i += 3 {
		switch n - i {
		case 1:
			b0 := b[i]
			out = append(out, tbl[b0>>2], tbl[(b0&0x03)<<4], '=', '=')
		case 2:
			b0, b1 := b[i], b[i+1]
			out = append(out, tbl[b0>>2], tbl[((b0&0x03)<<4)|(b1>>4)], tbl[(b1&0x0f)<<2], '=')
		default:
			b0, b1, b2 := b[i], b[i+1], b[i+2]
			out = append(out, tbl[b0>>2], tbl[((b0&0x03)<<4)|(b1>>4)], tbl[((b1&0x0f)<<2)|(b2>>6)], tbl[b2&0x3f])
		}
	}
	return string(out)
}

func TestEffectiveLimit(t *testing.T) {
	cases := []struct {
		name string
		in   ListRequest
		want int
	}{
		{"default when zero", ListRequest{}, DefaultLimit},
		{"legacy page_size", ListRequest{PageSize: 7}, 7},
		{"new limit beats legacy", ListRequest{Limit: 10, PageSize: 99}, 10},
		{"clamped to max", ListRequest{Limit: 9999}, MaxLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.in
			if got := r.EffectiveLimit(); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestValidateSortMismatchRejected(t *testing.T) {
	c := Cursor{K: []any{"2026", int64(1)}, S: "-effective_date"}
	r := ListRequest{Limit: 25, Cursor: Encode(c), Sort: "-id"}
	if err := r.Validate("-effective_date"); !errors.Is(err, ErrSortMismatch) {
		t.Fatalf("want ErrSortMismatch, got %v", err)
	}
}

func TestValidateSortDefaultAccepted(t *testing.T) {
	c := Cursor{K: []any{"2026", int64(1)}, S: "-effective_date"}
	r := ListRequest{Limit: 25, Cursor: Encode(c)}
	if err := r.Validate("-effective_date"); err != nil {
		t.Fatalf("default sort should match: %v", err)
	}
}

func TestValidateLimitTooLarge(t *testing.T) {
	r := ListRequest{Limit: MaxLimit + 1}
	if err := r.Validate("-id"); !errors.Is(err, ErrLimitTooLarge) {
		t.Fatalf("want ErrLimitTooLarge, got %v", err)
	}
}

func TestBuildEnvelopeHasMore(t *testing.T) {
	rows := []int{1, 2, 3, 4} // limit=3, +1 row signals more
	env := BuildEnvelope(rows, 3, "-id", func(v int) []any { return []any{int64(v)} })
	if !env.HasMore {
		t.Fatal("want HasMore=true")
	}
	if len(env.Items) != 3 {
		t.Fatalf("want 3 items, got %d", len(env.Items))
	}
	if env.NextCursor == "" {
		t.Fatal("want next_cursor")
	}
	got, _ := Decode(env.NextCursor)
	if id, _ := got.LastID(); id != 3 {
		t.Fatalf("next cursor should point at last kept row, got %d", id)
	}
}

func TestBuildEnvelopeNoMore(t *testing.T) {
	rows := []int{1, 2}
	env := BuildEnvelope(rows, 5, "-id", func(v int) []any { return []any{int64(v)} })
	if env.HasMore {
		t.Fatal("want HasMore=false")
	}
	if env.NextCursor != "" {
		t.Fatal("want empty next_cursor when no more pages")
	}
}

func TestBuildEnvelopeEmptyEmitsArrayNotNull(t *testing.T) {
	env := BuildEnvelope[int](nil, 5, "-id", nil)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	// Must contain `"items":[]` not `"items":null` — FE parsers expect
	// an array.
	want := `"items":[]`
	if got := string(b); !contains(got, want) {
		t.Fatalf("envelope should emit %s, got %s", want, got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
