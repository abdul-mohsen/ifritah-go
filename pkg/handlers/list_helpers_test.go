package handlers

import "testing"

func strPtr(s string) *string { return &s }

func TestBuildBillSearchParams_PhoneShapedQuery(t *testing.T) {
	// Regression: "0512345678" used to be ParseUint'd into 512345678
	// (leading zero dropped) and matched against bill.sequence_number,
	// surfacing unrelated invoices. After the fix, a phone-shaped
	// query must produce a digits-only phone wrapper and never a
	// sequence-number match.
	name, phone := buildBillSearchParams(strPtr("0512345678"))
	if phone == nil || *phone != "%0512345678%" {
		t.Fatalf("expected phone digits %%0512345678%%, got %v", phone)
	}
	// "0512345678" has no non-digits, so we should not also do a
	// name LIKE search (precision: phone-shaped → phone fields only).
	if name != nil {
		t.Fatalf("expected nil name like for pure-digit query, got %q", *name)
	}
}

func TestBuildBillSearchParams_FormattedPhone(t *testing.T) {
	// "+966 51-234 5678" must normalize to digits-only
	// "966512345678" so SQL REGEXP_REPLACE can match storage forms
	// like "+966512345678" or "0512345678".
	_, phone := buildBillSearchParams(strPtr("+966 51-234 5678"))
	if phone == nil || *phone != "%966512345678%" {
		t.Fatalf("expected %%966512345678%%, got %v", phone)
	}
}

func TestBuildBillSearchParams_ArabicIndicDigits(t *testing.T) {
	// "٠٥١٢٣٤٥٦٧٨" must fold to "0512345678".
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
	// Mixed alphanumeric: should still try both (name LIKE original
	// query, phone digits from the digit run).
	name, phone := buildBillSearchParams(strPtr("Ali 0512345678"))
	if phone == nil || *phone != "%0512345678%" {
		t.Fatalf("expected phone %%0512345678%%, got %v", phone)
	}
	if name == nil || *name != "%Ali 0512345678%" {
		t.Fatalf("expected name LIKE %%Ali 0512345678%%, got %v", name)
	}
}

func TestBuildBillSearchParams_TooShortForPhone(t *testing.T) {
	// "12" is below minPhoneSearchDigits — must not be treated as
	// phone digits (would match thousands of phone numbers).
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
