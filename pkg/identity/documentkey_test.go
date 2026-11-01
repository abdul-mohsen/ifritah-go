package identity

import (
	"strings"
	"testing"
)

func TestDocumentKey(t *testing.T) {
	tests := []struct {
		name      string
		tenantKey string
		branchID  uint64
		docType   string
		sourceID  uint64
		want      string
	}{
		{"bill", "acme", 7, DocBill, 42, "acme:7:bill:42"},
		{"credit note", "acme", 7, DocCreditNote, 9, "acme:7:credit_note:9"},
		{"debit note", "acme", 7, DocDebitNote, 9, "acme:7:debit_note:9"},
		{"64 characters", strings.Repeat("k", 64), 1, DocBill, 1, strings.Repeat("k", 64) + ":1:bill:1"},
		{"64 Arabic characters", strings.Repeat("ك", 64), 1, DocBill, 1, strings.Repeat("ك", 64) + ":1:bill:1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DocumentKey(tt.tenantKey, tt.branchID, tt.docType, tt.sourceID)
			if err != nil {
				t.Fatalf("DocumentKey: %v", err)
			}
			if got != tt.want {
				t.Fatalf("DocumentKey = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDocumentKeyRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		tenantKey string
		branchID  uint64
		docType   string
		sourceID  uint64
	}{
		{"empty tenant key", "", 7, DocBill, 42},
		{"65 characters", strings.Repeat("k", 65), 7, DocBill, 42},
		{"65 Arabic characters", strings.Repeat("ك", 65), 7, DocBill, 42},
		{"separator in tenant key", "a:b", 7, DocBill, 42},
		{"zero branch", "acme", 0, DocBill, 42},
		{"zero source", "acme", 7, DocBill, 0},
		{"unknown document type", "acme", 7, "invoice", 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DocumentKey(tt.tenantKey, tt.branchID, tt.docType, tt.sourceID); err == nil {
				t.Fatal("DocumentKey accepted invalid input")
			}
		})
	}
}
