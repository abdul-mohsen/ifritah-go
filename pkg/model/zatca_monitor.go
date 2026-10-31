package model

// Response shapes for the ZATCA submission monitor (read-only ops dashboard).
// Backed by handlers in pkg/handlers/zatca_monitor.go.

// ZatcaMonitorStats is the aggregated submission counter response for
// GET /zatca/monitor/stats.
type ZatcaMonitorStats struct {
	TotalSubmitted int64 `json:"total_submitted"`
	Accepted       int64 `json:"accepted"`
	Warnings       int64 `json:"warnings"`
	Rejected       int64 `json:"rejected"`
	Pending        int64 `json:"pending"`
}

// ZatcaBranchRow is one row in the GET /zatca/monitor/branches response.
type ZatcaBranchRow struct {
	BranchID         int64   `json:"branch_id"`
	BranchName       string  `json:"branch_name"`
	ZatcaStatus      int     `json:"zatca_status"`
	CertExpiry       *string `json:"cert_expiry"`
	TodayCount       int64   `json:"today_count"`
	SuccessRate      float64 `json:"success_rate"`
	LastSubmissionAt *string `json:"last_submission_at"`
}

// ZatcaSubmissionRow is one row in the GET /zatca/monitor/submissions response.
type ZatcaSubmissionRow struct {
	TenantID             int64   `json:"tenant_id"`
	DocumentType         string  `json:"document_type"`
	SourceID             int64   `json:"source_id"`
	PosID                string  `json:"pos_id"`
	RequestID            string  `json:"request_id"`
	InvoiceUUID          string  `json:"invoice_uuid"`
	OriginalDocumentType *string `json:"original_document_type"`
	OriginalSourceID     *int64  `json:"original_source_id"`
	OriginalInvoiceUUID  *string `json:"original_invoice_uuid"`
	DocumentNumber       *string `json:"document_number"`
	BranchID             *int64  `json:"branch_id"`
	BranchName           string  `json:"branch_name"`
	Status               string  `json:"status"`
	StatusCode           int     `json:"status_code"`
	ZatcaRef             *string `json:"zatca_ref"`
	WarningMsg           *string `json:"warning_msg"`
	SubmittedAt          *string `json:"submitted_at"`
}
