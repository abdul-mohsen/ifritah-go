package goosemigrations

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestBillIdentityContract checks what 00002 adds to bill. A (pos_id, bill_request_id) pair is
// unique, bills without identity can be any number, and a fingerprint must be 64 lowercase hex
// characters. It needs a MySQL server, like TestBaselineMatchesLegacySchema.
func TestBillIdentityContract(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}

	db, provider := openFreshGooseDB(t, dsn, "goose_bill_identity")
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	const fingerprint = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	insert := func(posID, requestID, fp any) error {
		_, err := db.Exec(`INSERT INTO bill (discount, store_id, merchant_id, maintenance_cost,
			pos_id, bill_request_id, request_fingerprint) VALUES (0, 1, 1, 0, ?, ?, ?)`, posID, requestID, fp)
		return err
	}

	// Bills from before identity existed have NULL identity, and any number of them is allowed.
	for i := 0; i < 2; i++ {
		if err := insert(nil, nil, nil); err != nil {
			t.Fatalf("bill without identity: %v", err)
		}
	}

	if err := insert("pos-1", "req-1", fingerprint); err != nil {
		t.Fatalf("first bill with identity: %v", err)
	}
	// A POS that retries the same request hits the unique key. B5 turns this into a replay.
	if err := insert("pos-1", "req-1", fingerprint); err == nil || !strings.Contains(err.Error(), "Duplicate entry") {
		t.Fatalf("duplicate (pos_id, bill_request_id) was not rejected: %v", err)
	}
	// Another POS may reuse the request id, because the key is scoped to the POS.
	if err := insert("pos-2", "req-1", fingerprint); err != nil {
		t.Fatalf("same request id on another POS: %v", err)
	}
	// The fingerprint is 64 lowercase hex characters.
	for _, bad := range []string{"short", strings.ToUpper(fingerprint)} {
		err := insert("pos-3", "req-3", bad)
		if err == nil || !strings.Contains(err.Error(), "chk_bill_request_fingerprint") {
			t.Fatalf("fingerprint %q was not rejected by the check: %v", bad, err)
		}
	}
}
