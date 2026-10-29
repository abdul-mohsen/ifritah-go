package handlers

import (
	"errors"
	"fmt"
	"strings"
)

type DocType = string

const (
	DocBill    DocType = zatcaDocBill
	DocCredit  DocType = zatcaDocCredit
	DocDebit   DocType = zatcaDocDebit
	DocOnboard DocType = zatcaDocOnboard
)

const (
	ContractName    = "afrita.zatca.document-event"
	ContractVersion = "1"
)

var ErrInvalidMessage = errors.New("zatca: invalid message")

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("zatca message: %s %s", e.Field, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidMessage
}

// Message is the versioned event envelope. The first five fields are retained
// at the top level because deployed daemons still decode those legacy aliases.
type Message struct {
	DocType  DocType `json:"doc_type"`
	ID       int64   `json:"id"`
	BranchID int64   `json:"branch_id"`
	DBName   string  `json:"db_name"`
	OTP      string  `json:"otp,omitempty"`

	Contract       string `json:"contract"`
	Version        string `json:"version"`
	SchemaVersion  string `json:"schema_version"`
	EventID        string `json:"event_id"`
	IdempotencyKey string `json:"idempotency_key"`

	TenantID          string  `json:"tenant_id"`
	EGSID             string  `json:"egs_id"`
	DocumentType      DocType `json:"document_type"`
	DocumentID        int64   `json:"document_id"`
	SourceID          int64   `json:"source_id"`
	OutboxID          uint64  `json:"outbox_id"`
	StableDocumentKey string  `json:"stable_document_key"`
	Attempt           uint32  `json:"attempt"`
	AttemptCount      uint32  `json:"attempt_count"`
	AttemptID         string  `json:"attempt_id"`
}

func NewMessage(docType DocType, tenantID string, branchID, documentID int64, idempotencyKey string) Message {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" && docType != DocOnboard {
		idempotencyKey = DocumentIdempotencyKey(tenantID, branchID, docType, documentID)
	}
	message := Message{
		DocType:           docType,
		ID:                documentID,
		BranchID:          branchID,
		DBName:            tenantID,
		Contract:          ContractName,
		Version:           ContractVersion,
		SchemaVersion:     ContractVersion,
		TenantID:          tenantID,
		DocumentType:      docType,
		DocumentID:        documentID,
		SourceID:          documentID,
		IdempotencyKey:    idempotencyKey,
		StableDocumentKey: idempotencyKey,
		Attempt:           1,
	}
	if idempotencyKey != "" {
		message.EventID = "evt-" + sha256Hex(idempotencyKey)
		message.AttemptID = fmt.Sprintf("%s:1", idempotencyKey)
	}
	return message
}

func (m Message) withOTP(otp string) Message {
	m.OTP = strings.TrimSpace(otp)
	return m
}

func DocumentIdempotencyKey(tenantID string, branchID int64, docType DocType, documentID int64) string {
	return fmt.Sprintf("%s:%d:%s:%d", strings.TrimSpace(tenantID), branchID, docType, documentID)
}

func OnboardingIdempotencyKey(tenantID string, branchID int64, otp string) string {
	return fmt.Sprintf(
		"%s:%d:%s:%d:otp-%s",
		strings.TrimSpace(tenantID),
		branchID,
		DocOnboard,
		branchID,
		sha256Hex(strings.TrimSpace(otp))[:16],
	)
}

func (m Message) IdentityKey() string {
	tenantID := strings.TrimSpace(m.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(m.DBName)
	}
	docType := m.DocumentType
	if docType == "" {
		docType = m.DocType
	}
	sourceID := m.SourceID
	if sourceID == 0 {
		sourceID = m.DocumentID
	}
	if sourceID == 0 {
		sourceID = m.ID
	}
	if tenantID == "" || m.BranchID <= 0 || sourceID <= 0 || docType == "" {
		return ""
	}
	return DocumentIdempotencyKey(
		tenantID,
		m.BranchID,
		docType,
		sourceID,
	)
}

func (m Message) EffectiveIdempotencyKey() string {
	if key := strings.TrimSpace(m.StableDocumentKey); key != "" {
		return key
	}
	if key := strings.TrimSpace(m.IdempotencyKey); key != "" {
		return key
	}
	if m.DocType == DocOnboard || m.DocumentType == DocOnboard {
		return ""
	}
	return m.IdentityKey()
}

func (m Message) NATSMessageID() string {
	return m.EffectiveIdempotencyKey()
}

func (m Message) Subject() string {
	docType := strings.TrimSpace(string(m.DocType))
	if docType == "" {
		docType = strings.TrimSpace(string(m.DocumentType))
	}
	tenantID := strings.TrimSpace(m.DBName)
	if tenantID == "" {
		tenantID = strings.TrimSpace(m.TenantID)
	}
	return fmt.Sprintf("zatca.%s.%s.%d", docType, tenantID, m.BranchID)
}

func (m Message) Normalize() (Message, error) {
	m.DocType = DocType(strings.TrimSpace(string(m.DocType)))
	m.DocumentType = DocType(strings.TrimSpace(string(m.DocumentType)))
	m.DBName = strings.TrimSpace(m.DBName)
	m.TenantID = strings.TrimSpace(m.TenantID)
	m.Contract = strings.TrimSpace(m.Contract)
	m.Version = strings.TrimSpace(m.Version)
	m.SchemaVersion = strings.TrimSpace(m.SchemaVersion)
	m.EventID = strings.TrimSpace(m.EventID)
	m.IdempotencyKey = strings.TrimSpace(m.IdempotencyKey)
	m.EGSID = strings.TrimSpace(m.EGSID)
	m.StableDocumentKey = strings.TrimSpace(m.StableDocumentKey)
	m.AttemptID = strings.TrimSpace(m.AttemptID)
	m.OTP = strings.TrimSpace(m.OTP)

	if m.DocType == "" {
		m.DocType = m.DocumentType
	} else if m.DocumentType != "" && m.DocType != m.DocumentType {
		return Message{}, invalidField("document_type", "does not match doc_type")
	}
	if m.DocumentType == "" {
		m.DocumentType = m.DocType
	}

	if m.ID == 0 {
		m.ID = m.SourceID
		if m.ID == 0 {
			m.ID = m.DocumentID
		}
	} else if m.SourceID != 0 && m.ID != m.SourceID {
		return Message{}, invalidField("source_id", "does not match id")
	}
	if m.SourceID == 0 {
		m.SourceID = m.ID
	}
	if m.DocumentID == 0 {
		m.DocumentID = m.SourceID
	} else if m.DocumentID != m.SourceID {
		return Message{}, invalidField("document_id", "does not match source_id")
	}

	if m.TenantID == "" {
		m.TenantID = m.DBName
	} else if m.DBName != "" && m.TenantID != m.DBName {
		return Message{}, invalidField("tenant_id", "does not match db_name")
	}
	if m.DBName == "" {
		m.DBName = m.TenantID
	}

	if m.Contract == "" {
		m.Contract = ContractName
	}
	if m.Version == "" {
		m.Version = ContractVersion
	}
	if m.SchemaVersion == "" {
		m.SchemaVersion = m.Version
	}
	if m.EGSID == "" && m.TenantID != "" && m.BranchID > 0 {
		m.EGSID = defaultZatcaEGSID(m.TenantID, m.BranchID)
	}
	if m.StableDocumentKey == "" {
		m.StableDocumentKey = m.IdempotencyKey
	}
	if m.StableDocumentKey == "" && m.DocumentType != DocOnboard {
		m.StableDocumentKey = m.IdentityKey()
	}
	if m.IdempotencyKey == "" {
		m.IdempotencyKey = m.StableDocumentKey
	}
	if m.EventID == "" && m.StableDocumentKey != "" {
		m.EventID = "evt-" + sha256Hex(m.StableDocumentKey)
	}
	if m.Attempt == 0 {
		m.Attempt = m.AttemptCount + 1
		if m.Attempt == 0 {
			m.Attempt = 1
		}
	}
	if m.AttemptID == "" {
		if m.OutboxID > 0 {
			m.AttemptID = fmt.Sprintf("%d:%d", m.OutboxID, m.Attempt)
		} else if m.StableDocumentKey != "" {
			m.AttemptID = fmt.Sprintf("%s:%d", m.StableDocumentKey, m.Attempt)
		}
	}
	return m, nil
}

func (m Message) Validate() error {
	normalized, err := m.Normalize()
	if err != nil {
		return err
	}
	switch {
	case normalized.Contract != ContractName:
		return invalidField("contract", "is unsupported")
	case normalized.Version != ContractVersion:
		return invalidField("version", "is unsupported")
	case normalized.SchemaVersion != ContractVersion:
		return invalidField("schema_version", "is unsupported")
	case normalized.EventID == "":
		return invalidField("event_id", "is required")
	case normalized.TenantID == "":
		return invalidField("tenant_id", "is required")
	case normalized.BranchID <= 0:
		return invalidField("branch_id", "must be greater than zero")
	case !validDocType(normalized.DocumentType):
		return invalidField("document_type", "is unsupported")
	case normalized.SourceID <= 0:
		return invalidField("source_id", "must be greater than zero")
	case normalized.StableDocumentKey == "":
		return invalidField("stable_document_key", "is required")
	case normalized.Attempt == 0:
		return invalidField("attempt", "must be greater than zero")
	case normalized.DocumentType == DocOnboard && normalized.OTP == "":
		return invalidField("otp", "is required for onboarding")
	case normalized.DocumentType != DocOnboard && normalized.OTP != "":
		return invalidField("otp", "is only valid for onboarding")
	}
	return nil
}

func validDocType(docType DocType) bool {
	switch docType {
	case DocBill, DocCredit, DocDebit, DocOnboard:
		return true
	default:
		return false
	}
}

func invalidField(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}

func defaultZatcaEGSID(tenantID string, branchID int64) string {
	return fmt.Sprintf("%s:branch:%d", strings.TrimSpace(tenantID), branchID)
}
