// Package identity holds the stable keys that tie one tax document to its records in the
// backend, NATS and the ZATCA daemon.
package identity

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Document types, as they appear in a document key.
const (
	DocBill       = "bill"
	DocCreditNote = "credit_note"
	DocDebitNote  = "debit_note"
)

// maxTenantKeyLen matches zatca_master.tenant.tenant_key, a VARCHAR(64) counted in characters.
const maxTenantKeyLen = 64

// ValidateTenantKey checks a tenant key: 1 to 64 characters, and no ':', which separates the parts
// of a document key.
func ValidateTenantKey(tenantKey string) error {
	switch {
	case tenantKey == "" || utf8.RuneCountInString(tenantKey) > maxTenantKeyLen:
		return fmt.Errorf("identity: tenant key must be 1 to %d characters", maxTenantKeyLen)
	case strings.Contains(tenantKey, ":"):
		return fmt.Errorf("identity: tenant key %q contains ':'", tenantKey)
	}
	return nil
}

// DocumentKey returns tenant_key:branch_id:doc_type:source_id. The same document always gets the
// same key. The tenant key is company.tenant_key in the tenant database, the value of
// zatca_master.tenant.tenant_key for that tenant.
func DocumentKey(tenantKey string, branchID uint64, docType string, sourceID uint64) (string, error) {
	if err := ValidateTenantKey(tenantKey); err != nil {
		return "", err
	}
	switch {
	case branchID == 0:
		return "", fmt.Errorf("identity: branch id must be set")
	case sourceID == 0:
		return "", fmt.Errorf("identity: source id must be set")
	case docType != DocBill && docType != DocCreditNote && docType != DocDebitNote:
		return "", fmt.Errorf("identity: unknown document type %q", docType)
	}
	return strings.Join([]string{
		tenantKey,
		strconv.FormatUint(branchID, 10),
		docType,
		strconv.FormatUint(sourceID, 10),
	}, ":"), nil
}
