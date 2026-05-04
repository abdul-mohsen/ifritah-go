package handlers

import (
	"database/sql"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/pagination"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ============================================================================
// Cash Voucher CRUD + Workflow (سند صرف / سند قبض / سند صرف صندوق)
// ============================================================================
//
// Endpoints:
//   POST   /api/v2/cash_voucher/all           → ListCashVouchers
//   GET    /api/v2/cash_voucher/:id            → GetCashVoucher
//   POST   /api/v2/cash_voucher               → CreateCashVoucher
//   PUT    /api/v2/cash_voucher/:id            → UpdateCashVoucher
//   DELETE /api/v2/cash_voucher/:id            → DeleteCashVoucher
//   POST   /api/v2/cash_voucher/:id/approve    → ApproveCashVoucher
//   POST   /api/v2/cash_voucher/:id/post       → PostCashVoucher
//
// DATABASE: Run api_docs/cash_voucher/schema.sql first
//
// Workflow: Draft (0) → Approved (1) → Posted (2)
//   - Only draft vouchers can be edited/deleted
//   - Approve requires manager+ role
//   - Post is irreversible
//
// Follows patterns from: store.go.txt, branch.go.txt
// ============================================================================

// --- Request / Response types (local to this file) ---

type cashVoucherListRequest struct {
	// New cursor pagination keys.
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
	Sort   string `json:"sort"`
	Dir    string `json:"dir"`
	// Legacy keys — accepted for backwards compat, ignored once Cursor != "".
	PageNumber int `json:"page_number"`
	PageSize   int `json:"page_size"`

	Query       string `json:"query"`
	VoucherType string `json:"voucher_type"` // "disbursement", "receipt", or "" for all
	// State filter (FE §3). nil/absent = "any state" for cash vouchers
	// (workflow: 0=draft, 1=approved, 2=posted). Negative values are
	// treated as "any" so the FE can send -1 as the sentinel.
	State *int `json:"state"`
}

type cashVoucherCreateRequest struct {
	VoucherType          string          `json:"voucher_type" binding:"required"`
	EffectiveDate        time.Time       `json:"effective_date" binding:"required"`
	Amount               decimal.Decimal `json:"amount" binding:"required"`
	PaymentMethod        string          `json:"payment_method"`
	RecipientType        string          `json:"recipient_type" binding:"required"`
	RecipientID          *int32          `json:"recipient_id"`
	RecipientName        string          `json:"recipient_name" binding:"required"`
	ReferenceType        *string         `json:"reference_type"`
	ReferenceID          *int32          `json:"reference_id"`
	Description          *string         `json:"description"`
	Note                 *string         `json:"note"`
	BankName             *string         `json:"bank_name"`
	BankAccount          *string         `json:"bank_account"`
	TransactionReference *string         `json:"transaction_reference"`
	StoreID              int32           `json:"store_id" binding:"required"`
	BranchID             uint32          `json:"branch_id" binding:"required"`
}

type cashVoucherListItem struct {
	ID            int       `json:"id"`
	VoucherNumber int       `json:"voucher_number"`
	VoucherType   string    `json:"voucher_type"`
	EffectiveDate time.Time `json:"effective_date"`
	Amount        string    `json:"amount"` // string for decimal precision
	PaymentMethod string    `json:"payment_method"`
	State         int       `json:"state"`
	ReferenceType *string   `json:"reference_type"`
	ReferenceID   *int      `json:"reference_id"`
	RecipientType string    `json:"recipient_type"`
	RecipientID   *int      `json:"recipient_id"`
	RecipientName string    `json:"recipient_name"`
	Description   *string   `json:"description"`
	StoreID       int       `json:"store_id"`
	MerchantID    int       `json:"merchant_id"`
	CreatedBy     int       `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

type cashVoucherDetail struct {
	ID                   int        `json:"id"`
	VoucherNumber        int        `json:"voucher_number"`
	VoucherType          string     `json:"voucher_type"`
	EffectiveDate        time.Time  `json:"effective_date"`
	Amount               string     `json:"amount"`
	PaymentMethod        string     `json:"payment_method"`
	State                int        `json:"state"`
	ReferenceType        *string    `json:"reference_type"`
	ReferenceID          *int       `json:"reference_id"`
	RecipientType        string     `json:"recipient_type"`
	RecipientID          *int       `json:"recipient_id"`
	RecipientName        string     `json:"recipient_name"`
	Description          *string    `json:"description"`
	Note                 *string    `json:"note"`
	BankName             *string    `json:"bank_name"`
	BankAccount          *string    `json:"bank_account"`
	TransactionReference *string    `json:"transaction_reference"`
	StoreID              int        `json:"store_id"`
	MerchantID           int        `json:"merchant_id"`
	BranchID             *int       `json:"branch_id"`
	CreatedBy            int        `json:"created_by"`
	ApprovedBy           *int       `json:"approved_by"`
	ApprovedAt           *time.Time `json:"approved_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// ── List ────────────────────────────────────────────────────────────────────

const cashVoucherListSort = "-effective_date"

// ListCashVouchers returns a cursor-paginated page of cash vouchers
// scoped to the calling merchant. Sort key: (effective_date DESC, id
// DESC), backed by idx_cv_keyset (migration 0003).
// POST /api/v2/cash_voucher/all
func (h *handler) ListCashVouchers(c *gin.Context) {
	var req cashVoucherListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = cashVoucherListRequest{}
	}

	listReq := pagination.ListRequest{
		Limit:      req.Limit,
		Cursor:     req.Cursor,
		Sort:       req.Sort,
		Dir:        req.Dir,
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
	}
	if err := listReq.Validate(cashVoucherListSort); err != nil {
		log.Printf("ListCashVouchers: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	cur, _ := listReq.DecodedCursor()
	cursorDate, cursorID, ok := cursorDateAndID(cur)
	if !ok {
		log.Printf("ListCashVouchers: malformed cursor")
		c.Status(http.StatusBadRequest)
		return
	}

	limit := listReq.EffectiveLimit()
	merchantID := getMerchantID(c)

	where := "WHERE merchant_id = ?"
	args := []any{merchantID}

	if req.VoucherType != "" {
		if req.VoucherType != "disbursement" && req.VoucherType != "receipt" && req.VoucherType != "cash_box" {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "نوع السند غير صالح"})
			return
		}
		where += " AND voucher_type = ?"
		args = append(args, req.VoucherType)
	}

	// State filter (FE §3). Skip when nil or negative ("any state").
	if req.State != nil && *req.State >= 0 {
		where += " AND state = ?"
		args = append(args, *req.State)
	}

	if req.Query != "" {
		where += " AND (recipient_name LIKE ? OR description LIKE ? OR note LIKE ?)"
		q := "%" + req.Query + "%"
		args = append(args, q, q, q)
	}

	// Seek predicate (skip on first page).
	if cursorDate != nil && cursorID != nil {
		where += " AND (effective_date < ? OR (effective_date = ? AND id < ?))"
		args = append(args, *cursorDate, *cursorDate, *cursorID)
	}

	// +1 row trick for has_more.
	args = append(args, limit+1)

	dataSQL := `
		SELECT id, voucher_number, voucher_type, effective_date, amount,
		       payment_method, state, reference_type, reference_id,
		       recipient_type, recipient_id, recipient_name,
		       description, store_id, merchant_id, created_by, created_at
		FROM cash_voucher ` + where + `
		ORDER BY effective_date DESC, id DESC
		LIMIT ?
	`

	rows, err := h.DB.Query(dataSQL, args...)
	if err != nil {
		log.Printf("ERROR ListCashVouchers query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في قراءة البيانات"})
		return
	}
	defer rows.Close()

	items := make([]cashVoucherListItem, 0)
	for rows.Next() {
		var v cashVoucherListItem
		err := rows.Scan(
			&v.ID, &v.VoucherNumber, &v.VoucherType, &v.EffectiveDate, &v.Amount,
			&v.PaymentMethod, &v.State, &v.ReferenceType, &v.ReferenceID,
			&v.RecipientType, &v.RecipientID, &v.RecipientName,
			&v.Description, &v.StoreID, &v.MerchantID, &v.CreatedBy, &v.CreatedAt,
		)
		if err != nil {
			log.Printf("ERROR ListCashVouchers scan: %v", err)
			continue
		}
		items = append(items, v)
	}

	// Server-driven table sort (FE round-2 §1). Amount is wire-encoded
	// as a decimal string for precision; parse once per compare so the
	// numeric ordering matches the on-screen value.
	applyListSort(items, listReq.Sort, listReq.Dir, func(a, b cashVoucherListItem, k string) (int, bool) {
		switch k {
		case "voucher_number":
			return int64Cmp(int64(a.VoucherNumber), int64(b.VoucherNumber)), true
		case "voucher_type":
			return strCmp(a.VoucherType, b.VoucherType), true
		case "recipient_name":
			return strCmp(a.RecipientName, b.RecipientName), true
		case "amount":
			da, _ := decimal.NewFromString(a.Amount)
			db_, _ := decimal.NewFromString(b.Amount)
			return decCmp(da, db_), true
		case "effective_date":
			return timeCmp(a.EffectiveDate, b.EffectiveDate), true
		case "state":
			return int64Cmp(int64(a.State), int64(b.State)), true
		}
		return 0, false
	})

	envelope := pagination.BuildEnvelope(
		items,
		limit,
		cashVoucherListSort,
		func(v cashVoucherListItem) []any {
			return []any{v.EffectiveDate.UTC().Format(time.RFC3339Nano), int64(v.ID)}
		},
	)
	c.JSON(http.StatusOK, envelope)
}

// ── Detail ──────────────────────────────────────────────────────────────────

// GetCashVoucher returns a single cash voucher by ID.
// GET /api/v2/cash_voucher/:id
func (h *handler) GetCashVoucher(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("GetCashVoucher: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": "معرّف غير صالح"})
		return
	}

	merchantID := getMerchantID(c)
	var v cashVoucherDetail

	err = h.DB.QueryRow(`
		SELECT id, voucher_number, voucher_type, effective_date, amount,
		       payment_method, state, reference_type, reference_id,
		       recipient_type, recipient_id, recipient_name,
		       description, note, bank_name, bank_account, transaction_reference,
		       store_id, merchant_id, branch_id,
		       created_by, approved_by, approved_at, created_at, updated_at
		FROM cash_voucher
		WHERE id = ? AND merchant_id = ?
	`, id, merchantID).Scan(
		&v.ID, &v.VoucherNumber, &v.VoucherType, &v.EffectiveDate, &v.Amount,
		&v.PaymentMethod, &v.State, &v.ReferenceType, &v.ReferenceID,
		&v.RecipientType, &v.RecipientID, &v.RecipientName,
		&v.Description, &v.Note, &v.BankName, &v.BankAccount, &v.TransactionReference,
		&v.StoreID, &v.MerchantID, &v.BranchID,
		&v.CreatedBy, &v.ApprovedBy, &v.ApprovedAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"detail": "السند غير موجود"})
		} else {
			log.Printf("ERROR GetCashVoucher: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في قراءة البيانات"})
		}
		return
	}

	c.JSON(http.StatusOK, v)
}

// ── Create ──────────────────────────────────────────────────────────────────

// CreateCashVoucher creates a new cash voucher in draft state.
// POST /api/v2/cash_voucher
func (h *handler) CreateCashVoucher(c *gin.Context) {
	var req cashVoucherCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("CreateCashVoucher: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": "بيانات غير صالحة: " + err.Error()})
		return
	}

	// Validate fields
	if err := validateCashVoucherRequest(&req); err != nil {
		log.Printf("CreateCashVoucher: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	merchantID := int32(getMerchantID(c))

	effectiveDate := req.EffectiveDate

	// Verify store exists
	var storeExists int
	h.DB.QueryRow("SELECT COUNT(*) FROM store WHERE id = ?",
		req.StoreID).Scan(&storeExists)
	if storeExists == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "المخزن غير موجود"})
		return
	}

	// Get next voucher number for this merchant (atomic)
	tx, err := h.DB.Begin()
	if err != nil {
		log.Printf("ERROR CreateCashVoucher begin tx: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في إنشاء السند"})
		return
	}
	defer tx.Rollback()

	var nextNumber int
	err = tx.QueryRow(
		"SELECT COALESCE(MAX(voucher_number), 0) + 1 FROM cash_voucher WHERE merchant_id = ?",
		merchantID,
	).Scan(&nextNumber)
	if err != nil {
		log.Printf("ERROR CreateCashVoucher next_number: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في إنشاء السند"})
		return
	}

	// Default payment method
	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = "cash"
	}

	qtx := h.queries.WithTx(tx)

	arg := db.CreateCashVoucherParams{
		VoucherNumber:        int32(nextNumber),
		VoucherType:          db.CashVoucherVoucherType(req.VoucherType),
		EffectiveDate:        effectiveDate,
		Amount:               req.Amount,
		PaymentMethod:        db.CashVoucherPaymentMethod(paymentMethod),
		ReferenceType:        req.ReferenceType,
		ReferenceID:          req.ReferenceID,
		RecipientType:        db.CashVoucherRecipientType(req.RecipientType),
		RecipientID:          req.RecipientID,
		RecipientName:        req.RecipientName,
		Description:          req.Description,
		Note:                 req.Note,
		BankName:             req.BankName,
		BankAccount:          req.BankAccount,
		TransactionReference: req.TransactionReference,
		StoreID:              req.StoreID,
		MerchantID:           merchantID,
		BranchID:             &req.BranchID,
		CreatedBy:            merchantID,
		ApprovedBy:           &merchantID,
	}
	result, err := qtx.CreateCashVoucher(c.Request.Context(), arg)

	if err != nil {
		log.Printf("ERROR CreateCashVoucher insert: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في إنشاء السند"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR CreateCashVoucher commit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في إنشاء السند"})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"id":             id,
		"voucher_number": nextNumber,
	})
}

// ── Update ──────────────────────────────────────────────────────────────────

// UpdateCashVoucher updates a draft cash voucher.
// PUT /api/v2/cash_voucher/:id
func (h *handler) UpdateCashVoucher(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("UpdateCashVoucher: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": "معرّف غير صالح"})
		return
	}

	var req cashVoucherCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("UpdateCashVoucher: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": "بيانات غير صالحة: " + err.Error()})
		return
	}

	if err := validateCashVoucherRequest(&req); err != nil {
		log.Printf("UpdateCashVoucher: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	merchantID := getMerchantID(c)

	effectiveDate := req.EffectiveDate

	// Verify the voucher exists and is in draft state
	var currentState int
	err = h.DB.QueryRow(
		"SELECT state FROM cash_voucher WHERE id = ? AND merchant_id = ?",
		id, merchantID,
	).Scan(&currentState)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"detail": "السند غير موجود"})
		} else {
			log.Printf("ERROR UpdateCashVoucher check: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في تحديث السند"})
		}
		return
	}
	if currentState != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "لا يمكن تعديل سند غير مسودة"})
		return
	}

	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = "cash"
	}

	_, err = h.DB.Exec(`
		UPDATE cash_voucher SET
			voucher_type = ?,
			effective_date = ?,
			amount = ?,
			payment_method = ?,
			reference_type = ?,
			reference_id = ?,
			recipient_type = ?,
			recipient_id = ?,
			recipient_name = ?,
			description = ?,
			note = ?,
			bank_name = ?,
			bank_account = ?,
			transaction_reference = ?,
			store_id = ?
		WHERE id = ? AND merchant_id = ? AND state = 0
	`,
		req.VoucherType, effectiveDate, req.Amount, paymentMethod,
		req.ReferenceType, req.ReferenceID,
		req.RecipientType, req.RecipientID, req.RecipientName,
		req.Description, req.Note, req.BankName, req.BankAccount, req.TransactionReference,
		req.StoreID,
		id, merchantID,
	)
	if err != nil {
		log.Printf("ERROR UpdateCashVoucher: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في تحديث السند"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// ── Delete ──────────────────────────────────────────────────────────────────

// DeleteCashVoucher deletes a draft cash voucher.
// DELETE /api/v2/cash_voucher/:id
func (h *handler) DeleteCashVoucher(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("DeleteCashVoucher: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": "معرّف غير صالح"})
		return
	}

	merchantID := getMerchantID(c)

	// Check state before delete
	var currentState int
	err = h.DB.QueryRow(
		"SELECT state FROM cash_voucher WHERE id = ? AND merchant_id = ?",
		id, merchantID,
	).Scan(&currentState)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"detail": "السند غير موجود"})
		} else {
			log.Printf("ERROR DeleteCashVoucher check: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في حذف السند"})
		}
		return
	}
	if currentState != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "لا يمكن حذف سند غير مسودة"})
		return
	}

	_, err = h.DB.Exec(
		"DELETE FROM cash_voucher WHERE id = ? AND merchant_id = ? AND state = 0",
		id, merchantID,
	)
	if err != nil {
		log.Printf("ERROR DeleteCashVoucher: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في حذف السند"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ── Approve ─────────────────────────────────────────────────────────────────

// ApproveCashVoucher transitions a draft voucher to approved (0 → 1).
// POST /api/v2/cash_voucher/:id/approve
// Requires manager or admin role.
func (h *handler) ApproveCashVoucher(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("ApproveCashVoucher: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": "معرّف غير صالح"})
		return
	}

	// Check role (manager+ only)
	role := getUserRole(c)
	if role != "admin" && role != "manager" {
		c.JSON(http.StatusForbidden, gin.H{"detail": "صلاحيات غير كافية — يتطلب مدير أو أعلى"})
		return
	}

	merchantID := getMerchantID(c)
	userID := getUserID(c)

	// Verify voucher exists and is in draft state
	var currentState int
	err = h.DB.QueryRow(
		"SELECT state FROM cash_voucher WHERE id = ? AND merchant_id = ?",
		id, merchantID,
	).Scan(&currentState)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"detail": "السند غير موجود"})
		} else {
			log.Printf("ERROR ApproveCashVoucher check: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في اعتماد السند"})
		}
		return
	}
	if currentState != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "لا يمكن اعتماد سند ليس في حالة مسودة"})
		return
	}

	now := time.Now()
	_, err = h.DB.Exec(`
		UPDATE cash_voucher SET state = 1, approved_by = ?, approved_at = ?
		WHERE id = ? AND merchant_id = ? AND state = 0
	`, userID, now, id, merchantID)
	if err != nil {
		log.Printf("ERROR ApproveCashVoucher update: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في اعتماد السند"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "approved",
		"state":       1,
		"approved_by": userID,
		"approved_at": now.Format(time.RFC3339),
	})
}

// ── Post ────────────────────────────────────────────────────────────────────

// PostCashVoucher transitions an approved voucher to posted (1 → 2).
// POST /api/v2/cash_voucher/:id/post
// This is irreversible — represents the actual cash movement.
func (h *handler) PostCashVoucher(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("PostCashVoucher: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": "معرّف غير صالح"})
		return
	}

	merchantID := getMerchantID(c)

	// Verify voucher exists and is in approved state
	var currentState int
	err = h.DB.QueryRow(
		"SELECT state FROM cash_voucher WHERE id = ? AND merchant_id = ?",
		id, merchantID,
	).Scan(&currentState)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"detail": "السند غير موجود"})
		} else {
			log.Printf("ERROR PostCashVoucher check: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في ترحيل السند"})
		}
		return
	}
	if currentState != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "لا يمكن ترحيل سند غير معتمد"})
		return
	}

	_, err = h.DB.Exec(`
		UPDATE cash_voucher SET state = 2
		WHERE id = ? AND merchant_id = ? AND state = 1
	`, id, merchantID)
	if err != nil {
		log.Printf("ERROR PostCashVoucher update: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "خطأ في ترحيل السند"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "posted",
		"state":   2,
	})
}

// ── Dashboard Aggregation ───────────────────────────────────────────────────

// GetCashVoucherSummary returns totals for disbursements and receipts.
// Can be called from the dashboard handler to include cash flow stats.
// GET /api/v2/cash_voucher/summary (optional, for dashboard integration)
func (h *handler) GetCashVoucherSummary(c *gin.Context) {
	merchantID := getMerchantID(c)

	var totalDisbursements, totalReceipts float64
	var pendingCount int

	h.DB.QueryRow(`
		SELECT COALESCE(SUM(amount), 0) FROM cash_voucher
		WHERE merchant_id = ? AND voucher_type = 'disbursement' AND state = 2
	`, merchantID).Scan(&totalDisbursements)

	h.DB.QueryRow(`
		SELECT COALESCE(SUM(amount), 0) FROM cash_voucher
		WHERE merchant_id = ? AND voucher_type = 'receipt' AND state = 2
	`, merchantID).Scan(&totalReceipts)

	h.DB.QueryRow(`
		SELECT COUNT(*) FROM cash_voucher
		WHERE merchant_id = ? AND state IN (0, 1)
	`, merchantID).Scan(&pendingCount)

	c.JSON(http.StatusOK, gin.H{
		"total_disbursements": totalDisbursements,
		"total_receipts":      totalReceipts,
		"net_cash_flow":       totalReceipts - totalDisbursements,
		"pending_vouchers":    pendingCount,
	})
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// validateCashVoucherRequest validates the create/update request fields.
func validateCashVoucherRequest(req *cashVoucherCreateRequest) error {
	// Amount
	// TODO @ssda please fix this
	// if req.Amount <= 0 {
	// 	return &validationError{"المبلغ يجب أن يكون أكبر من صفر"}
	// }

	// Voucher type
	if req.VoucherType != "disbursement" && req.VoucherType != "receipt" && req.VoucherType != "cash_box" {
		return &validationError{"نوع السند يجب أن يكون سند صرف أو سند قبض أو سند صرف صندوق"}
	}

	// Recipient type
	validRecipients := map[string]bool{
		"supplier": true, "client": true, "employee": true, "other": true,
	}
	if !validRecipients[req.RecipientType] {
		return &validationError{"نوع المستلم غير صالح"}
	}

	// Payment method
	pm := strings.ToLower(req.PaymentMethod)
	if pm == "" {
		pm = "cash"
		req.PaymentMethod = pm
	}
	if pm != "cash" && pm != "bank_transfer" {
		return &validationError{"طريقة الدفع غير صالحة"}
	}

	// Bank fields required for bank_transfer
	if pm == "bank_transfer" {
		if req.BankName == nil || *req.BankName == "" {
			return &validationError{"اسم البنك مطلوب عند التحويل البنكي"}
		}
		if req.BankAccount == nil || *req.BankAccount == "" {
			return &validationError{"رقم الحساب مطلوب عند التحويل البنكي"}
		}
	}

	return nil
}

// validationError implements the error interface for validation messages.
type validationError struct {
	msg string
}

func (e *validationError) Error() string {
	return e.msg
}

// getMerchantID extracts the merchant_id from the JWT context.
// The JWT middleware sets this from the token claims.
func getMerchantID(c *gin.Context) int64 {
	return GetSessionInfo(c).id
}

// getUserID extracts the user_id from the JWT context.
func getUserID(c *gin.Context) int64 {
	return GetSessionInfo(c).id
}

// getUserRole extracts the role from the JWT context.
func getUserRole(c *gin.Context) string {
	if role, exists := c.Get("user_role"); exists {
		if s, ok := role.(string); ok {
			return s
		}
	}
	return ""
}
