package handlers

// Legacy ZATCA numeric status values retained for compatibility with the
// existing status column and old dashboard consumers.
const (
	ZatcaStatusPending   = 0
	ZatcaStatusSubmitted = 1
	ZatcaStatusAccepted  = 2
	ZatcaStatusRejected  = 3
	ZatcaStatusWarning   = 4
)

// Explicit durable state values stored in outcome_state.
const (
	ZatcaStatePending             = "pending"
	ZatcaStateAccepted            = "accepted"
	ZatcaStateAcceptedWithWarning = "accepted_with_warning"
	ZatcaStateRejected            = "rejected"
	ZatcaStateAmbiguous           = "ambiguous"
	ZatcaStateTerminalFailure     = "terminal_failure"
)

// String labels accepted by the monitor query parameters. "warning" is kept
// as a backwards-compatible alias for accepted_with_warning.
const (
	ZatcaLabelPending             = ZatcaStatePending
	ZatcaLabelAccepted            = ZatcaStateAccepted
	ZatcaLabelAcceptedWithWarning = ZatcaStateAcceptedWithWarning
	ZatcaLabelRejected            = ZatcaStateRejected
	ZatcaLabelWarning             = "warning"
	ZatcaLabelAmbiguous           = ZatcaStateAmbiguous
	ZatcaLabelTerminalFailure     = ZatcaStateTerminalFailure
	ZatcaLabelUnknown             = "unknown"
)

// Sentinel values for ListZatcaMonitorSubmissions filters. Kept here so the
// handler and the sqlc query stay aligned.
const (
	zatcaPendingFlagOn   = "y"
	zatcaPendingFlagOff  = ""
	zatcaStateAny        = ""
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
