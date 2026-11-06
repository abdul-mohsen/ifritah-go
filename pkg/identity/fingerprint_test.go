package identity

import (
	"regexp"
	"testing"
)

type fingerprintSample struct {
	Amount string `json:"amount"`
	Count  int    `json:"count"`
}

// A fingerprint is the 64-digit lowercase hex form that bill.request_fingerprint accepts. It repeats for the same value
// and changes with the content.
func TestFingerprintIsAStableHexDigestOfTheContent(t *testing.T) {
	first, err := Fingerprint(fingerprintSample{Amount: "10", Count: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(first) {
		t.Fatalf("fingerprint %q is not 64 lowercase hex digits", first)
	}
	again, err := Fingerprint(fingerprintSample{Amount: "10", Count: 2})
	if err != nil {
		t.Fatal(err)
	}
	other, err := Fingerprint(fingerprintSample{Amount: "10", Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	if first != again || first == other {
		t.Fatal("the fingerprint must repeat for the same value and change when the content changes")
	}
}
