package handlers

import "testing"

func TestZatcaCredentialTypeDefaultsToProduction(t *testing.T) {
	if got := normalizeCredentialType(""); got != zatcaCredentialTypeProduction {
		t.Fatalf("normalizeCredentialType(\"\") = %q, want %q", got, zatcaCredentialTypeProduction)
	}
	if got := normalizeCredentialType(" Compliance "); got != zatcaCredentialTypeCompliance {
		t.Fatalf("normalizeCredentialType(compliance) = %q, want %q", got, zatcaCredentialTypeCompliance)
	}
}

func TestZatcaCredentialTypeValidation(t *testing.T) {
	if _, ok := credentialTypeFromRequest("unknown"); ok {
		t.Fatal("unknown credential type accepted")
	}
	if got, ok := credentialTypeFromRequest("production"); !ok || got != zatcaCredentialTypeProduction {
		t.Fatalf("production credential type = %q, valid = %v", got, ok)
	}
}

func TestParseTenantIDRejectsMissingAndZero(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "not-a-number"} {
		if got, err := parseTenantID(value); err == nil || got != 0 {
			t.Fatalf("parseTenantID(%q) = %d, %v; want zero and an error", value, got, err)
		}
	}
	if got, err := parseTenantID("42"); err != nil || got != 42 {
		t.Fatalf("parseTenantID(42) = %d, %v; want 42", got, err)
	}
}
