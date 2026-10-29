package model

import (
	"encoding/json"
	"time"
)

const (
	ZatcaImportStatusReview   = "review"
	ZatcaImportStatusApproved = "approved"
	ZatcaImportStatusPosted   = "posted"
	ZatcaImportStatusRejected = "rejected"
)

// ZatcaImportSource mirrors the read-only XMLSourceEvidence contract exposed
// by the T030 signed-UBL service.
type ZatcaImportSource struct {
	ContentHash      string `json:"content_hash,omitempty"`
	Size             int64  `json:"size,omitempty"`
	MIMEType         string `json:"mime_type,omitempty"`
	OriginalName     string `json:"original_name,omitempty"`
	ContentReference string `json:"content_reference,omitempty"`
	Provenance       string `json:"provenance,omitempty"`
}

type ZatcaImportValidationIssue struct {
	Code     string          `json:"code"`
	Field    string          `json:"field,omitempty"`
	Message  string          `json:"message"`
	Severity string          `json:"severity"`
	Details  json.RawMessage `json:"details,omitempty"`
}

// ZatcaImportValidation mirrors T030 ValidationState. The backend stores it
// as evidence and does not recompute parser policy.
type ZatcaImportValidation struct {
	WellFormed      bool                         `json:"well_formed"`
	SchemaStatus    string                       `json:"schema_status,omitempty"`
	SignatureStatus string                       `json:"signature_status,omitempty"`
	ReviewRequired  bool                         `json:"review_required"`
	Issues          []ZatcaImportValidationIssue `json:"issues,omitempty"`
}

// ZatcaImportSignature mirrors T030 SignatureEvidence.
type ZatcaImportSignature struct {
	Present             bool   `json:"present"`
	Status              string `json:"status,omitempty"`
	Algorithm           string `json:"algorithm,omitempty"`
	SignatureValueHash  string `json:"signature_value_hash,omitempty"`
	InvoiceDigest       string `json:"invoice_digest,omitempty"`
	CertificateSHA256   string `json:"certificate_sha256,omitempty"`
	CertificateIssuer   string `json:"certificate_issuer,omitempty"`
	CertificateSerial   string `json:"certificate_serial,omitempty"`
	CertificateSubject  string `json:"certificate_subject,omitempty"`
	VerificationMessage string `json:"verification_message,omitempty"`
}

// ZatcaImportRequest is the backend adapter for T030 ImportResult. It keeps
// canonical/parsed data opaque so this repository remains independent of the
// separate parser module while preserving its versioned JSON contract.
type ZatcaImportRequest struct {
	OriginalBytes  []byte                `json:"original_bytes,omitempty"`
	Bytes          []byte                `json:"bytes,omitempty"`
	XMLContent     string                `json:"xml_content,omitempty"`
	Source         ZatcaImportSource     `json:"source"`
	Canonical      json.RawMessage       `json:"canonical,omitempty"`
	Parsed         json.RawMessage       `json:"parsed,omitempty"`
	Signature      ZatcaImportSignature  `json:"signature"`
	Validation     ZatcaImportValidation `json:"validation"`
	FailureClass   string                `json:"failure_class,omitempty"`
	FailureMessage string                `json:"failure_message,omitempty"`
	BranchID       *uint32               `json:"branch_id,omitempty"`
	IdempotencyKey string                `json:"idempotency_key,omitempty"`
}

// SignedUBLImportRequest is a descriptive compatibility alias for adapters
// that name the format rather than the ZATCA intake lane.
type SignedUBLImportRequest = ZatcaImportRequest

type ZatcaImportReviewRequest struct {
	Decision                string  `json:"decision"`
	Reason                  string  `json:"reason,omitempty"`
	ExpectedWorkflowVersion *uint64 `json:"expected_workflow_version,omitempty"`
}

type ZatcaImportPostRequest struct {
	PostReference           string  `json:"post_reference,omitempty"`
	ExpectedWorkflowVersion *uint64 `json:"expected_workflow_version,omitempty"`
}

type ZatcaImportRejectRequest struct {
	Reason                  string  `json:"reason"`
	ExpectedWorkflowVersion *uint64 `json:"expected_workflow_version,omitempty"`
}

type ZatcaImportJob struct {
	ID                uint64                       `json:"id"`
	MerchantID        int32                        `json:"merchant_id"`
	BranchID          *uint32                      `json:"branch_id,omitempty"`
	Status            string                       `json:"status"`
	WorkflowVersion   uint64                       `json:"workflow_version"`
	SourceContentHash string                       `json:"source_content_hash"`
	ReviewedBy        *int32                       `json:"reviewed_by,omitempty"`
	ReviewedAt        *time.Time                   `json:"reviewed_at,omitempty"`
	PostedBy          *int32                       `json:"posted_by,omitempty"`
	PostedAt          *time.Time                   `json:"posted_at,omitempty"`
	PostReference     *string                      `json:"post_reference,omitempty"`
	RejectedBy        *int32                       `json:"rejected_by,omitempty"`
	RejectedAt        *time.Time                   `json:"rejected_at,omitempty"`
	RejectedReason    *string                      `json:"rejected_reason,omitempty"`
	CreatedBy         int32                        `json:"created_by"`
	CreatedAt         time.Time                    `json:"created_at"`
	UpdatedAt         time.Time                    `json:"updated_at"`
	Evidence          *ZatcaImportEvidence         `json:"evidence,omitempty"`
	ValidationErrors  []ZatcaImportValidationError `json:"validation_errors,omitempty"`
	Reviews           []ZatcaImportReview          `json:"reviews,omitempty"`
	StatusHistory     []ZatcaImportStatusHistory   `json:"status_history,omitempty"`
}

type ZatcaImportEvidence struct {
	ID                 uint64          `json:"id"`
	ImportJobID        uint64          `json:"import_job_id"`
	MerchantID         int32           `json:"merchant_id"`
	SourceReference    string          `json:"source_reference"`
	SourceOriginalName *string         `json:"source_original_name,omitempty"`
	SourceMIMEType     string          `json:"source_mime_type"`
	SourceProvenance   string          `json:"source_provenance"`
	SourceContentHash  string          `json:"source_content_hash"`
	SourceSize         uint64          `json:"source_size"`
	OriginalBytes      []byte          `json:"original_bytes,omitempty"`
	CanonicalData      json.RawMessage `json:"canonical_data,omitempty"`
	ParsedData         json.RawMessage `json:"parsed_data,omitempty"`
	SignatureData      json.RawMessage `json:"signature_data,omitempty"`
	ValidationData     json.RawMessage `json:"validation_data,omitempty"`
	ResultData         json.RawMessage `json:"result_data,omitempty"`
	FailureClass       *string         `json:"failure_class,omitempty"`
	FailureMessage     *string         `json:"failure_message,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

type ZatcaImportValidationError struct {
	ID          uint64          `json:"id"`
	ImportJobID uint64          `json:"import_job_id"`
	MerchantID  int32           `json:"merchant_id"`
	Severity    string          `json:"severity"`
	ErrorCode   string          `json:"error_code"`
	FieldKey    *string         `json:"field_key,omitempty"`
	Message     string          `json:"message"`
	Details     json.RawMessage `json:"details,omitempty"`
	CreatedBy   int32           `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
}

type ZatcaImportReview struct {
	ID                      uint64    `json:"id"`
	ImportJobID             uint64    `json:"import_job_id"`
	MerchantID              int32     `json:"merchant_id"`
	Decision                string    `json:"decision"`
	Reason                  *string   `json:"reason,omitempty"`
	ExpectedWorkflowVersion *uint64   `json:"expected_workflow_version,omitempty"`
	ObservedWorkflowVersion uint64    `json:"observed_workflow_version"`
	CreatedBy               int32     `json:"created_by"`
	CreatedAt               time.Time `json:"created_at"`
}

type ZatcaImportStatusHistory struct {
	ID              uint64    `json:"id"`
	ImportJobID     uint64    `json:"import_job_id"`
	MerchantID      int32     `json:"merchant_id"`
	FromStatus      *string   `json:"from_status,omitempty"`
	ToStatus        string    `json:"to_status"`
	TransitionKey   string    `json:"transition_key"`
	WorkflowVersion uint64    `json:"workflow_version"`
	Reason          *string   `json:"reason,omitempty"`
	CreatedBy       int32     `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}
