package handlers

// ZATCA submission monitor — read-only ops dashboard endpoints.
//
// Backed by sqlc queries in pkg/db/queries/zatca_monitor.sql:
//   * GetZatcaMonitorStats        — aggregated counters
//   * ListZatcaMonitorBranches    — per-branch dashboard
//   * ListZatcaMonitorSubmissions — recent submissions list
//
// Explicit states and legacy numeric values: see
// pkg/handlers/zatca_constants.go.
//
// All endpoints are mounted behind admin middleware (see main.go).

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
)

// ZatcaMonitorStats returns tenant-scoped submission counters across branches.
func (h *handler) ZatcaMonitorStats(c *gin.Context) {
	row, err := h.queries.GetZatcaMonitorStats(c.Request.Context(), db.GetZatcaMonitorStatsParams{
		MerchantID: int32(getMerchantID(c)),
	})
	if err != nil {
		log.Printf("ZatcaMonitorStats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": ErrZatcaLoadStats})
		return
	}
	c.JSON(http.StatusOK, model.ZatcaMonitorStats{
		TotalSubmitted:      row.Total,
		Accepted:            row.Accepted,
		AcceptedWithWarning: row.AcceptedWithWarning,
		Warnings:            row.AcceptedWithWarning,
		Rejected:            row.Rejected,
		Ambiguous:           row.Ambiguous,
		TerminalFailure:     row.TerminalFailure,
		Pending:             row.Pending,
	})
}

// ZatcaMonitorBranches returns one row per branch with a branch_zatca_config
// entry, plus today's submission counts and 7-day success rate.
func (h *handler) ZatcaMonitorBranches(c *gin.Context) {
	rows, err := h.queries.ListZatcaMonitorBranches(c.Request.Context(), db.ListZatcaMonitorBranchesParams{
		MerchantID: int32(getMerchantID(c)),
	})
	if err != nil {
		log.Printf("ZatcaMonitorBranches: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": ErrZatcaLoadBranches})
		return
	}

	out := make([]model.ZatcaBranchRow, 0, len(rows))
	for _, r := range rows {
		item := model.ZatcaBranchRow{
			BranchID:    int64(r.BranchID),
			BranchName:  r.BranchName,
			ZatcaStatus: int(r.ZatcaStatus),
			TodayCount:  r.TodayCount,
		}
		if r.WeekTotal > 0 {
			item.SuccessRate = float64(r.WeekAccepted) / float64(r.WeekTotal) * 100.0
		}
		if r.Certificate != nil {
			if exp := certExpiryDate(*r.Certificate); exp != "" {
				item.CertExpiry = &exp
			}
		}
		if t, ok := lastSubmissionTime(r.LastSubmissionAt); ok {
			s := t.Format(time.RFC3339)
			item.LastSubmissionAt = &s
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, out)
}

// lastSubmissionTime extracts a time.Time from sqlc's `interface{}` column for
// MAX(submitted_at). The driver returns either time.Time or []byte depending
// on driver flags.
func lastSubmissionTime(v interface{}) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, !x.IsZero()
	case *time.Time:
		if x == nil {
			return time.Time{}, false
		}
		return *x, !x.IsZero()
	case []byte:
		if t, err := time.Parse("2006-01-02 15:04:05", string(x)); err == nil {
			return t, true
		}
	case string:
		if t, err := time.Parse("2006-01-02 15:04:05", x); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// certExpiryDate parses an X.509 certificate and returns its NotAfter as
// "YYYY-MM-DD". Accepts either a PEM block or a base64-encoded DER blob (ZATCA
// returns the production cert base64-encoded). Returns "" on any parse error.
func certExpiryDate(s string) string {
	if s == "" {
		return ""
	}
	var der []byte
	if block, _ := pem.Decode([]byte(s)); block != nil {
		der = block.Bytes
	} else if raw, err := base64.StdEncoding.DecodeString(s); err == nil {
		der = raw
	} else {
		return ""
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return ""
	}
	return cert.NotAfter.Format("2006-01-02")
}

// legacyStatusToState converts the old numeric status into the explicit state
// used when a legacy row has not yet been populated with outcome_state.
func legacyStatusToState(s int) string {
	switch s {
	case ZatcaStatusAccepted:
		return ZatcaStateAccepted
	case ZatcaStatusRejected:
		return ZatcaStateRejected
	case ZatcaStatusWarning:
		return ZatcaStateAcceptedWithWarning
	case ZatcaStatusPending, ZatcaStatusSubmitted:
		return ZatcaStatePending
	default:
		return ZatcaLabelUnknown
	}
}

func normalizeZatcaState(state string, legacyStatus int) string {
	switch state {
	case ZatcaStatePending,
		ZatcaStateAccepted,
		ZatcaStateAcceptedWithWarning,
		ZatcaStateRejected,
		ZatcaStateAmbiguous,
		ZatcaStateTerminalFailure:
		return state
	default:
		return legacyStatusToState(legacyStatus)
	}
}

// parseSubmissionFilters reads `status` and `branch_id` query params into the
// sentinel-filter shape consumed by ListZatcaMonitorSubmissions. On failure
// it writes a 400 response and returns ok=false.
func parseSubmissionFilters(c *gin.Context) (pendingFlag, stateFilter string, statusCode int, branchID uint32, ok bool) {
	pendingFlag = zatcaPendingFlagOff
	stateFilter = zatcaStateAny
	statusCode = zatcaStatusCodeAny
	branchID = zatcaBranchAny

	if s := c.Query("status"); s != "" {
		switch s {
		case ZatcaLabelAccepted:
			stateFilter = ZatcaStateAccepted
		case ZatcaLabelAcceptedWithWarning, ZatcaLabelWarning:
			stateFilter = ZatcaStateAcceptedWithWarning
		case ZatcaLabelRejected:
			stateFilter = ZatcaStateRejected
		case ZatcaLabelPending:
			pendingFlag = zatcaPendingFlagOn
			stateFilter = ZatcaStatePending
		case ZatcaLabelAmbiguous:
			stateFilter = ZatcaStateAmbiguous
		case ZatcaLabelTerminalFailure:
			stateFilter = ZatcaStateTerminalFailure
		default:
			n, err := strconv.ParseInt(s, 10, 8)
			if err != nil {
				log.Printf("ZatcaMonitorSubmissions: invalid status %q: %v", s, err)
				c.JSON(http.StatusBadRequest, gin.H{"error": ErrZatcaInvalidStatus})
				return "", "", 0, 0, false
			}
			statusCode = int(n)
		}
	}
	if bs := c.Query("branch_id"); bs != "" {
		bid, err := strconv.ParseUint(bs, 10, 32)
		if err != nil {
			log.Printf("ZatcaMonitorSubmissions: invalid branch_id %q: %v", bs, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": ErrZatcaInvalidBranch})
			return "", "", 0, 0, false
		}
		branchID = uint32(bid)
	}
	return pendingFlag, stateFilter, statusCode, branchID, true
}

// ZatcaMonitorSubmissions returns recent submissions, optionally filtered by
// status (numeric or label) and branch_id, ordered by submitted_at DESC.
func (h *handler) ZatcaMonitorSubmissions(c *gin.Context) {
	limit := zatcaSubmissionsHint
	if v, err := strconv.ParseInt(c.DefaultQuery("limit", strconv.Itoa(zatcaSubmissionsHint)), 10, 32); err == nil && v > 0 && v <= int64(zatcaSubmissionsCap) {
		limit = int(v)
	}

	pendingFlag, stateFilter, statusCode, branchID, ok := parseSubmissionFilters(c)
	if !ok {
		return
	}

	branchPtr := &branchID
	rows, err := h.queries.ListZatcaMonitorSubmissions(c.Request.Context(), db.ListZatcaMonitorSubmissionsParams{
		MerchantID:  int32(getMerchantID(c)),
		PendingFlag: pendingFlag,
		StateFilter: stateFilter,
		StatusCode:  int8(statusCode),
		BranchID:    branchPtr,
		Limit:       int32(limit),
	})
	if err != nil {
		log.Printf("ZatcaMonitorSubmissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": ErrZatcaLoadSubmissions})
		return
	}

	out := make([]model.ZatcaSubmissionRow, 0, len(rows))
	for _, r := range rows {
		state := normalizeZatcaState(r.OutcomeState, int(r.LegacyStatus))
		item := model.ZatcaSubmissionRow{
			InvoiceID:           int64(r.InvoiceID),
			BranchName:          r.BranchName,
			DocumentType:        r.DocumentType,
			SourceID:            int64(r.SourceID),
			StatusCode:          int(r.LegacyStatus),
			Status:              state,
			State:               state,
			DocumentID:          r.DocumentID,
			ProviderCode:        r.ProviderCode,
			ProviderWarningCode: r.ProviderWarningCode,
			ProviderReference:   r.ProviderReference,
			RetryCount:          int(r.RetryCount),
		}
		if r.SequenceNumber != nil {
			v := *r.SequenceNumber
			item.SequenceNumber = &v
		}
		if r.EgsID != nil {
			v := *r.EgsID
			item.EGSID = &v
		}
		if r.StableDocumentKey != nil {
			v := *r.StableDocumentKey
			item.StableDocumentKey = &v
		}
		if r.SequenceNumber != nil {
			item.InvoiceNo = "INV-" + strconv.FormatUint(*r.SequenceNumber, 10)
		} else {
			item.InvoiceNo = "INV-" + strconv.FormatUint(r.InvoiceID, 10)
		}
		if r.BranchID != nil {
			v := int64(*r.BranchID)
			item.BranchID = &v
		}
		if r.ProviderWarningMessage != nil {
			v := *r.ProviderWarningMessage
			item.ProviderWarning = &v
			item.WarningMsg = &v
		} else if r.RejectionMsg != nil {
			v := *r.RejectionMsg
			item.WarningMsg = &v
		}
		if r.ProviderReference != nil {
			v := *r.ProviderReference
			item.ZatcaRef = &v
		} else if r.RejectionCode != nil {
			v := *r.RejectionCode
			item.ZatcaRef = &v
		}
		if r.SubmittedAt != nil {
			s := r.SubmittedAt.Format(time.RFC3339)
			item.SubmittedAt = &s
		}
		item.LastAttemptAt = formatZatcaTime(r.LastAttemptAt)
		item.NextRetryAt = formatZatcaTime(r.NextRetryAt)
		item.OutcomeAt = formatZatcaTime(r.OutcomeAt)
		if r.RequestXmlPath != nil {
			v := *r.RequestXmlPath
			item.RequestXMLPath = &v
		}
		if r.ClearedXmlPath != nil {
			v := *r.ClearedXmlPath
			item.ClearedXMLPath = &v
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, out)
}

func formatZatcaTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
