package identity

import (
	"strings"
	"testing"
)

func TestValidatePosID(t *testing.T) {
	for _, ok := range []string{"pos-1", "كاشير-2", strings.Repeat("p", 64)} {
		if err := ValidatePosID(ok); err != nil {
			t.Errorf("ValidatePosID(%q) = %v; want nil", ok, err)
		}
	}
	for _, bad := range []string{"", " pos", "pos ", strings.Repeat("p", 65)} {
		if err := ValidatePosID(bad); err == nil {
			t.Errorf("ValidatePosID(%q) was accepted", bad)
		}
	}
}

func TestValidateBillRequestID(t *testing.T) {
	if err := ValidateBillRequestID("3f2b9c1e-7a4d-4e8b-9c21-5d0e6f7a8b9c"); err != nil {
		t.Fatalf("valid lowercase UUID rejected: %v", err)
	}
	for _, bad := range []string{"", "3F2B9C1E-7A4D-4E8B-9C21-5D0E6F7A8B9C", "req-1", "3f2b9c1e7a4d4e8b9c215d0e6f7a8b9c"} {
		if err := ValidateBillRequestID(bad); err == nil {
			t.Errorf("ValidateBillRequestID(%q) was accepted", bad)
		}
	}
}
