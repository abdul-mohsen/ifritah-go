package handlers

import "testing"

func strPtr(s string) *string { return &s }

func TestBuildPlainPrefixFilter(t *testing.T) {
	cases := []struct {
		name string
		in   *string
		want *string
	}{
		{"nil", nil, nil},
		{"empty", strPtr(""), nil},
		{"whitespace", strPtr("   "), nil},
		{"simple", strPtr("300"), strPtr("300%")},
		{"trims", strPtr("  AB12 "), strPtr("AB12%")},
		{"unicode passthrough", strPtr("شركة"), strPtr("شركة%")},
		{"phone digits", strPtr("0501"), strPtr("0501%")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildPlainPrefixFilter(tc.in)
			switch {
			case tc.want == nil && got == nil:
				return
			case tc.want == nil || got == nil:
				t.Fatalf("got %v want %v", got, tc.want)
			case *got != *tc.want:
				t.Fatalf("got %q want %q", *got, *tc.want)
			}
		})
	}
}

func TestBuildBillSearchParams_PhoneShapedQuery(t *testing.T) {
	name, phone := buildBillSearchParams(strPtr("0512345678"))
	if phone == nil || *phone != "%0512345678%" {
		t.Fatalf("expected phone digits %%0512345678%%, got %v", phone)
	}
	if name != nil {
		t.Fatalf("expected nil name like for pure-digit query, got %q", *name)
	}
}

func TestBuildBillSearchParams_FormattedPhone(t *testing.T) {
	_, phone := buildBillSearchParams(strPtr("+966 51-234 5678"))
	if phone == nil || *phone != "%966512345678%" {
		t.Fatalf("expected %%966512345678%%, got %v", phone)
	}
}

func TestBuildBillSearchParams_ArabicIndicDigits(t *testing.T) {
	_, phone := buildBillSearchParams(strPtr("٠٥١٢٣٤٥٦٧٨"))
	if phone == nil || *phone != "%0512345678%" {
		t.Fatalf("expected %%0512345678%%, got %v", phone)
	}
}

func TestBuildBillSearchParams_NameQuery(t *testing.T) {
	name, phone := buildBillSearchParams(strPtr("Ahmad"))
	if phone != nil {
		t.Fatalf("name query should not produce phone digits, got %q", *phone)
	}
	if name == nil || *name != "%Ahmad%" {
		t.Fatalf("expected name LIKE %%Ahmad%%, got %v", name)
	}
}

func TestBuildBillSearchParams_MixedQuery(t *testing.T) {
	name, phone := buildBillSearchParams(strPtr("Ali 0512345678"))
	if phone == nil || *phone != "%0512345678%" {
		t.Fatalf("expected phone %%0512345678%%, got %v", phone)
	}
	if name == nil || *name != "%Ali 0512345678%" {
		t.Fatalf("expected name LIKE %%Ali 0512345678%%, got %v", name)
	}
}

func TestBuildBillSearchParams_TooShortForPhone(t *testing.T) {
	name, phone := buildBillSearchParams(strPtr("12"))
	if phone != nil {
		t.Fatalf("short numeric must not produce phone digits, got %q", *phone)
	}
	if name == nil || *name != "%12%" {
		t.Fatalf("expected fallback name LIKE %%12%%, got %v", name)
	}
}

func TestBuildBillSearchParams_Empty(t *testing.T) {
	name, phone := buildBillSearchParams(nil)
	if name != nil || phone != nil {
		t.Fatalf("nil query must yield nil/nil, got name=%v phone=%v", name, phone)
	}
	name, phone = buildBillSearchParams(strPtr(""))
	if name != nil || phone != nil {
		t.Fatalf("empty query must yield nil/nil, got name=%v phone=%v", name, phone)
	}
}

