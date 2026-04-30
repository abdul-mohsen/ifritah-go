package handlers

// ZATCA submission monitor — read-only ops dashboard endpoints.
//
// Backed by:
//   * zatca_submission (status, retry_count, submitted_at, cleared_at, ...)
//   * branch_zatca_config (zatca_status, zatca_*_certificate, ...)
//   * bill (branch_id, invoice_uuid, sequence_number, effective_date)
//   * branches (id, name)
//
// Status enum (matches pkg/db/gen/dashboard.sql.go GetDashboardZATCAStats):
//   0 = pending, 1 = submitted, 2 = accepted, 3 = rejected, 4 = warning
//
// All endpoints require the caller to be admin.

import (
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"ifritah/web-service-gin/pkg/model"
)

// requireAdmin aborts with 403 unless the JWT claims have role == "admin".
// Inlined here so this file is independent of the user-management PR.
func requireAdmin(c *gin.Context) bool {
	cs, ok := c.Get("decoded_jwt")
	if ok {
		if claims, ok := cs.(*model.Claims); ok && claims != nil && claims.Role == "admin" {
			return true
		}
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
	return false
}

// ──────────────────────────────────────────────────────────────────────────────
// GET /api/v2/zatca/monitor/stats
// ──────────────────────────────────────────────────────────────────────────────

// ZatcaMonitorStats returns aggregated submission counters across all branches.
func (h *handler) ZatcaMonitorStats(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	row := h.DB.QueryRow(`
        SELECT
          COUNT(*)                                                    AS total,
          COUNT(CASE WHEN status = 2 THEN 1 END)                      AS accepted,
          COUNT(CASE WHEN status = 4 THEN 1 END)                      AS warnings,
          COUNT(CASE WHEN status = 3 THEN 1 END)                      AS rejected,
          COUNT(CASE WHEN status IN (0, 1) THEN 1 END)                AS pending
        FROM zatca_submission`)
	var total, accepted, warnings, rejected, pending int64
	if err := row.Scan(&total, &accepted, &warnings, &rejected, &pending); err != nil {
		log.Printf("ZatcaMonitorStats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_submitted": total,
		"accepted":        accepted,
		"warnings":        warnings,
		"rejected":        rejected,
		"pending":         pending,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// GET /api/v2/zatca/monitor/branches
// ──────────────────────────────────────────────────────────────────────────────

// zatcaBranchRow is the JSON shape returned for each onboarded branch.
type zatcaBranchRow struct {
	BranchID         int64    `json:"branch_id"`
	BranchName       string   `json:"branch_name"`
	ZatcaStatus      int      `json:"zatca_status"`
	CertExpiry       *string  `json:"cert_expiry"`
	TodayCount       int64    `json:"today_count"`
	SuccessRate      float64  `json:"success_rate"`
	LastSubmissionAt *string  `json:"last_submission_at"`
}

// ZatcaMonitorBranches returns one row per branch with a branch_zatca_config
// entry, plus today's submission counts and 7-day success rate.
func (h *handler) ZatcaMonitorBranches(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	rows, err := h.DB.Query(`
        SELECT
          b.id,
          b.name,
          bzc.zatca_status,
          bzc.zatca_compliance_certificate,
          (SELECT COUNT(*)
             FROM zatca_submission zs
             JOIN bill bl ON bl.id = zs.bill_id
             WHERE bl.branch_id = b.id
               AND DATE(zs.submitted_at) = CURDATE())                   AS today_count,
          (SELECT COUNT(*)
             FROM zatca_submission zs
             JOIN bill bl ON bl.id = zs.bill_id
             WHERE bl.branch_id = b.id
               AND zs.submitted_at >= NOW() - INTERVAL 7 DAY)           AS week_total,
          (SELECT COUNT(*)
             FROM zatca_submission zs
             JOIN bill bl ON bl.id = zs.bill_id
             WHERE bl.branch_id = b.id
               AND zs.submitted_at >= NOW() - INTERVAL 7 DAY
               AND zs.status = 2)                                       AS week_accepted,
          (SELECT MAX(zs.submitted_at)
             FROM zatca_submission zs
             JOIN bill bl ON bl.id = zs.bill_id
             WHERE bl.branch_id = b.id)                                 AS last_submission_at
        FROM branches b
        JOIN branch_zatca_config bzc ON bzc.branch_id = b.id
        ORDER BY b.id ASC`)
	if err != nil {
		log.Printf("ZatcaMonitorBranches query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load branches"})
		return
	}
	defer rows.Close()

	out := make([]zatcaBranchRow, 0)
	for rows.Next() {
		var (
			r           zatcaBranchRow
			cert        sql.NullString
			weekTotal   int64
			weekAccept  int64
			lastSub     sql.NullString
		)
		if err := rows.Scan(
			&r.BranchID, &r.BranchName, &r.ZatcaStatus, &cert,
			&r.TodayCount, &weekTotal, &weekAccept, &lastSub,
		); err != nil {
			log.Printf("ZatcaMonitorBranches scan: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan branches"})
			return
		}
		if weekTotal > 0 {
			r.SuccessRate = float64(weekAccept) / float64(weekTotal) * 100.0
		}
		if exp := certExpiryDate(cert.String); exp != "" {
			r.CertExpiry = &exp
		}
		if lastSub.Valid {
			v := lastSub.String
			r.LastSubmissionAt = &v
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, out)
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

// ──────────────────────────────────────────────────────────────────────────────
// GET /api/v2/zatca/monitor/submissions
// ──────────────────────────────────────────────────────────────────────────────

// statusToString converts the numeric submission status into the labels the
// frontend uses.
func statusToString(s int) string {
	switch s {
	case 2:
		return "accepted"
	case 3:
		return "rejected"
	case 4:
		return "warning"
	case 0, 1:
		return "pending"
	default:
		return "unknown"
	}
}

// ZatcaMonitorSubmissions returns recent submissions, optionally filtered by
// status (numeric or label) and branch_id, ordered by submitted_at DESC.
func (h *handler) ZatcaMonitorSubmissions(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	// Sentinel filters keep the SQL static for code-scanners (Sonar S2077).
	pendingFlag := ""    // "y" => match status IN (0,1)
	statusCode := -1     // -1 => no exact-status match
	var branchID int64 = 0 // 0 => no branch filter

	if s := c.Query("status"); s != "" {
		switch s {
		case "accepted":
			statusCode = 2
		case "rejected":
			statusCode = 3
		case "warning":
			statusCode = 4
		case "pending":
			pendingFlag = "y"
		default:
			n, err := strconv.Atoi(s)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
				return
			}
			statusCode = n
		}
	}
	if bs := c.Query("branch_id"); bs != "" {
		bid, err := strconv.ParseInt(bs, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return
		}
		branchID = bid
	}

	rows, err := h.DB.Query(`
        SELECT
          bl.id            AS invoice_id,
          bl.sequence_number,
          bl.branch_id,
          COALESCE(b.name, '')  AS branch_name,
          zs.status,
          zs.rejection_code,
          zs.rejection_msg,
          zs.submitted_at
        FROM zatca_submission zs
        JOIN bill bl     ON bl.id = zs.bill_id
        LEFT JOIN branches b ON b.id = bl.branch_id
        WHERE (? = '' OR zs.status IN (0,1))
          AND (? = -1 OR zs.status = ?)
          AND (? = 0 OR bl.branch_id = ?)
        ORDER BY zs.submitted_at DESC
        LIMIT ?`,
		pendingFlag, statusCode, statusCode, branchID, branchID, limit)
	if err != nil {
		log.Printf("ZatcaMonitorSubmissions query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load submissions"})
		return
	}
	defer rows.Close()

	type submissionRow struct {
		InvoiceID    int64   `json:"invoice_id"`
		InvoiceNo    string  `json:"invoice_no"`
		BranchID     *int64  `json:"branch_id"`
		BranchName   string  `json:"branch_name"`
		Status       string  `json:"status"`
		StatusCode   int     `json:"status_code"`
		ZatcaRef     *string `json:"zatca_ref"`
		WarningMsg   *string `json:"warning_msg"`
		SubmittedAt  *string `json:"submitted_at"`
	}

	out := make([]submissionRow, 0)
	for rows.Next() {
		var (
			r          submissionRow
			seq        sql.NullInt64
			branchID   sql.NullInt64
			rejCode    sql.NullString
			rejMsg     sql.NullString
			submitted  sql.NullString
		)
		if err := rows.Scan(
			&r.InvoiceID, &seq, &branchID, &r.BranchName,
			&r.StatusCode, &rejCode, &rejMsg, &submitted,
		); err != nil {
			log.Printf("ZatcaMonitorSubmissions scan: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan submissions"})
			return
		}
		r.Status = statusToString(r.StatusCode)
		if seq.Valid {
			r.InvoiceNo = "INV-" + strconv.FormatInt(seq.Int64, 10)
		} else {
			r.InvoiceNo = "INV-" + strconv.FormatInt(r.InvoiceID, 10)
		}
		if branchID.Valid {
			v := branchID.Int64
			r.BranchID = &v
		}
		if rejCode.Valid {
			v := rejCode.String
			r.ZatcaRef = &v
		}
		if rejMsg.Valid {
			v := rejMsg.String
			r.WarningMsg = &v
		}
		if submitted.Valid {
			v := submitted.String
			r.SubmittedAt = &v
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, out)
}
