package model

// Response shapes for the ZATCA submission monitor (read-only ops dashboard).
// Backed by handlers in pkg/handlers/zatca_monitor.go.

// ZatcaMonitorStats is the aggregated submission counter response for
// GET /zatca/monitor/stats.
type ZatcaMonitorStats struct {
	TotalSubmitted      int64   `json:"total_submitted"`
	Accepted            int64   `json:"accepted"`
	AcceptedWithWarning int64   `json:"accepted_with_warning"`
	Warnings            int64   `json:"warnings"`
	Rejected            int64   `json:"rejected"`
	Ambiguous           int64   `json:"ambiguous"`
	TerminalFailure     int64   `json:"terminal_failure"`
	Pending             int64   `json:"pending"`
	OldestPendingAt     *string `json:"oldest_pending_at"`
	PendingAgeSeconds   *int64  `json:"pending_age_seconds"`
}

// ZatcaBranchRow is one row in the GET /zatca/monitor/branches response.
type ZatcaBranchRow struct {
	BranchID            int64   `json:"branch_id"`
	BranchName          string  `json:"branch_name"`
	ZatcaStatus         int     `json:"zatca_status"`
	CertExpiry          *string `json:"cert_expiry"`
	TodayCount          int64   `json:"today_count"`
	SuccessRate         float64 `json:"success_rate"`
	LastSubmissionAt    *string `json:"last_submission_at"`
	TotalSubmitted      int64   `json:"total_submitted"`
	Accepted            int64   `json:"accepted"`
	AcceptedWithWarning int64   `json:"accepted_with_warning"`
	Warnings            int64   `json:"warnings"`
	Rejected            int64   `json:"rejected"`
	Pending             int64   `json:"pending"`
	Ambiguous           int64   `json:"ambiguous"`
	TerminalFailure     int64   `json:"terminal_failure"`
	OldestPendingAt     *string `json:"oldest_pending_at"`
	PendingAgeSeconds   *int64  `json:"pending_age_seconds"`
	PendingRetryCount   int64   `json:"pending_retry_count"`
	PendingNextRetryAt  *string `json:"pending_next_retry_at"`
}

// ZatcaSubmissionRow is one row in the GET /zatca/monitor/submissions response.
type ZatcaSubmissionRow struct {
	InvoiceID           int64   `json:"invoice_id"`
	InvoiceNo           string  `json:"invoice_no"`
	BranchID            *int64  `json:"branch_id"`
	BranchName          string  `json:"branch_name"`
	Status              string  `json:"status"`
	State               string  `json:"state"`
	StatusCode          int     `json:"status_code"`
	DocumentID          string  `json:"document_id"`
	ProviderCode        *string `json:"provider_code"`
	ProviderWarningCode *string `json:"provider_warning_code"`
	ProviderWarning     *string `json:"provider_warning"`
	ProviderReference   *string `json:"provider_reference"`
	ZatcaRef            *string `json:"zatca_ref"`
	WarningMsg          *string `json:"warning_msg"`
	RetryCount          int     `json:"retry_count"`
	LastAttemptAt       *string `json:"last_attempt_at"`
	NextRetryAt         *string `json:"next_retry_at"`
	RequestXMLPath      *string `json:"request_xml_path"`
	ClearedXMLPath      *string `json:"cleared_xml_path"`
	OutcomeAt           *string `json:"outcome_at"`
	SubmittedAt         *string `json:"submitted_at"`
	AttemptNumber       *int64  `json:"attempt_number"`
	RetryState          *string `json:"retry_state"`
	RetryErrorCode      *string `json:"retry_error_code"`
	RetryErrorMessage   *string `json:"retry_error_message"`
	PendingSince        *string `json:"pending_since"`
	PendingAgeSeconds   *int64  `json:"pending_age_seconds"`
}
