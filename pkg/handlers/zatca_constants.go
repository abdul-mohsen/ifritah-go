package handlers

// ZATCA submission status enum — matches the `status` column on
// the `zatca_submission` table and pkg/db/gen/dashboard.sql.go.
const (
	ZatcaStatusPending   = 0
	ZatcaStatusSubmitted = 1
	ZatcaStatusAccepted  = 2
	ZatcaStatusRejected  = 3
	ZatcaStatusWarning   = 4
)

// String labels exposed to the frontend for the numeric status codes above.
const (
	ZatcaLabelPending  = "pending"
	ZatcaLabelAccepted = "accepted"
	ZatcaLabelRejected = "rejected"
	ZatcaLabelWarning  = "warning"
	ZatcaLabelUnknown  = "unknown"
)

// Sentinel values for ListZatcaMonitorSubmissions filters. Kept here so the
// handler and the sqlc query stay aligned.
const (
	zatcaPendingFlagOn   = "y"
	zatcaPendingFlagOff  = ""
	zatcaStatusCodeAny   = -1
	zatcaBranchAny       = uint32(0)
	zatcaSubmissionsCap  = 200
	zatcaSubmissionsHint = 50
)

// Repeated 5xx detail strings for ZATCA monitor handlers.
const (
	ErrZatcaLoadStats       = "failed to load stats"
	ErrZatcaLoadBranches    = "failed to load branches"
	ErrZatcaLoadSubmissions = "failed to load submissions"
	ErrZatcaInvalidStatus   = "invalid status"
	ErrZatcaInvalidBranch   = "invalid branch_id"
)
