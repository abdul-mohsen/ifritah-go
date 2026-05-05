package handlers

import (
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/pagination"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ZatcaStatusDeleted   = 0
	ZatcaStatusActive    = 1
	ZatcaStatusExpired   = 2
	ZatcaStatusNotActive = 3
)

type OTPRequest struct {
	OTP string `json:"otp" binding:"required"`
}

const branchListSort = "id"

// ── POST /api/v2/branch/all ─────────────────────────────────────────────────
// Cursor-paginated list of branches with store count and ZATCA
// registration status. Sort key: id ASC (matches the pre-cursor ORDER
// BY b.id ascending). Primary key serves the seek directly.

func (h *handler) ListBranches(c *gin.Context) {
	var req struct {
		Limit  int     `json:"limit"`
		Cursor string  `json:"cursor"`
		Sort   string  `json:"sort"`
		Dir    string  `json:"dir"`
		Query  *string `json:"query"`
		// Legacy keys ignored once Cursor != "".
		PageNumber int `json:"page_number"`
		PageSize   int `json:"page_size"`
	}
	c.ShouldBindJSON(&req)

	listReq := pagination.ListRequest{
		Limit:      req.Limit,
		Cursor:     req.Cursor,
		Sort:       req.Sort,
		Dir:        req.Dir,
		Query:      req.Query,
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
	}
	if err := listReq.Validate(branchListSort); err != nil {
		log.Printf("ListBranches: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	cur, _ := listReq.DecodedCursor()
	cursorID := cursorIDOnly(cur)

	limit := listReq.EffectiveLimit()

	// Sentinel-filter pattern in the dynamic builder: `(? = '' OR …)`
	// keeps the SQL effectively static so MySQL still plans it as a
	// composite range scan when the search is empty.
	q := ""
	if req.Query != nil {
		q = *req.Query
	}
	qLike := "%" + q + "%"

	where := "WHERE (? = '' OR b.name LIKE ? OR COALESCE(b.address,'') LIKE ? OR COALESCE(b.phone,'') LIKE ?)"
	args := []any{q, qLike, qLike, qLike}
	if cursorID != nil {
		where += " AND b.id > ?"
		args = append(args, *cursorID)
	}
	args = append(args, limit+1)

	rows, err := h.DB.Query(`
		SELECT b.id, b.name, COALESCE(b.address,''), COALESCE(b.city,''),
		       COALESCE(b.phone,''), b.company_id, b.manager_id, b.is_active,
		       b.created_at,
		       (SELECT COUNT(*) FROM store s WHERE s.branch_id = b.id) AS store_count,
		       (CASE
		           WHEN LENGTH(COALESCE(bzc.zatca_production_username,'')) > 0 THEN 'registered'
		           WHEN LENGTH(COALESCE(bzc.zatca_csr,'')) > 0 THEN 'compliance_only'
		           ELSE 'not_registered'
		       END) AS zatca_status
		FROM branches b
		LEFT JOIN branch_zatca_config bzc ON bzc.branch_id = b.id
		`+where+`
		ORDER BY b.id
		LIMIT ?
	`, args...)
	if err != nil {
		log.Printf("ERROR ListBranches: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch branches"})
		return
	}
	defer rows.Close()

	type branchRow struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Address     string `json:"address"`
		City        string `json:"city"`
		Phone       string `json:"phone"`
		CompanyID   *int   `json:"company_id"`
		ManagerID   *int   `json:"manager_id"`
		IsActive    bool   `json:"is_active"`
		CreatedAt   string `json:"created_at"`
		StoreCount  int    `json:"store_count"`
		ZatcaStatus string `json:"zatca_status"`
	}

	var branches []branchRow
	for rows.Next() {
		var b branchRow
		if err := rows.Scan(&b.ID, &b.Name, &b.Address, &b.City, &b.Phone,
			&b.CompanyID, &b.ManagerID, &b.IsActive, &b.CreatedAt,
			&b.StoreCount, &b.ZatcaStatus); err != nil {
			log.Printf("ERROR ListBranches scan: %v", err)
			continue
		}
		branches = append(branches, b)
	}

	// Sort over the current page is FE-driven; BE returns rows in
	// the canonical keyset order only.

	envelope := pagination.BuildEnvelope(
		branches,
		limit,
		branchListSort,
		func(b branchRow) []any { return []any{int64(b.ID)} },
	)
	c.JSON(http.StatusOK, envelope)
}

// ── GET /api/v2/branch/:id ──────────────────────────────────────────────────
// Returns a single branch with its linked stores.

func (h *handler) GetBranch(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return
	}

	var b struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Address   string `json:"address"`
		City      string `json:"city"`
		Phone     string `json:"phone"`
		CompanyID *int   `json:"company_id"`
		ManagerID *int   `json:"manager_id"`
		IsActive  bool   `json:"is_active"`
		CreatedAt string `json:"created_at"`
	}

	err = h.DB.QueryRow(`
		SELECT id, name, COALESCE(address,''), COALESCE(city,''),
		       COALESCE(phone,''), company_id, manager_id, is_active, created_at
		FROM branches WHERE id = ?
	`, id).Scan(&b.ID, &b.Name, &b.Address, &b.City, &b.Phone,
		&b.CompanyID, &b.ManagerID, &b.IsActive, &b.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "branch not found"})
		return
	}

	// Fetch stores linked to this branch
	storeRows, err := h.DB.Query(
		"SELECT id, name FROM store WHERE branch_id = ? ORDER BY id", id,
	)
	if err != nil {
		log.Printf("ERROR GetBranch stores: %v", err)
	}

	type storeItem struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var stores []storeItem
	if storeRows != nil {
		defer storeRows.Close()
		for storeRows.Next() {
			var s storeItem
			if err := storeRows.Scan(&s.ID, &s.Name); err == nil {
				stores = append(stores, s)
			}
		}
	}
	if stores == nil {
		stores = []storeItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"detail": gin.H{
			"id":          b.ID,
			"name":        b.Name,
			"address":     b.Address,
			"city":        b.City,
			"phone":       b.Phone,
			"company_id":  b.CompanyID,
			"manager_id":  b.ManagerID,
			"is_active":   b.IsActive,
			"created_at":  b.CreatedAt,
			"stores":      stores,
			"store_count": len(stores),
		},
	})
}

// ── POST /api/v2/branch ────────────────────────────────────────────────────
// Creates a new branch.

func (h *handler) CreateBranch(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		Address   string `json:"address"`
		City      string `json:"city"`
		Phone     string `json:"phone"`
		ManagerID *int   `json:"manager_id"`
		IsActive  *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "الاسم مطلوب"})
		return
	}

	// Get company_id from authenticated user's session (set in JWT claims)
	companyID := h.getUserCompany(c)

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	result, err := h.DB.Exec(`
		INSERT INTO branches (name, address, city, phone, company_id, manager_id, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, req.Name, req.Address, req.City, req.Phone, companyID, req.ManagerID, isActive)
	if err != nil {
		if IsDuplicate(err) {
			c.JSON(http.StatusConflict, gin.H{"detail": "اسم الفرع موجود بالفعل"})
		}
		log.Printf("ERROR CreateBranch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to create branch"})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"detail": gin.H{
			"id":        id,
			"name":      req.Name,
			"address":   req.Address,
			"city":      req.City,
			"phone":     req.Phone,
			"is_active": isActive,
		},
	})
}

// ── PUT /api/v2/branch/:id ──────────────────────────────────────────────────
// Updates an existing branch.

func (h *handler) UpdateBranch(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return
	}

	var req struct {
		Name      string `json:"name" binding:"required"`
		Address   string `json:"address"`
		City      string `json:"city"`
		Phone     string `json:"phone"`
		ManagerID *int   `json:"manager_id"`
		IsActive  *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	// Get company_id from authenticated user's session (set in JWT claims)
	companyID := h.getUserCompany(c)

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	_, err = h.DB.Exec(`
		UPDATE branches SET name=?, address=?, city=?, phone=?,
		       company_id=?, manager_id=?, is_active=?
		WHERE id=?
	`, req.Name, req.Address, req.City, req.Phone,
		companyID, req.ManagerID, isActive, id)
	if err != nil {
		log.Printf("ERROR UpdateBranch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update branch"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

// ── DELETE /api/v2/branch/:id ───────────────────────────────────────────────
// Deletes a branch. Prevents deletion if it has linked stores.
// branch_zatca_config rows are auto-deleted via ON DELETE CASCADE.

func (h *handler) DeleteBranch(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return
	}

	// Check if branch has stores
	var storeCount int
	h.DB.QueryRow("SELECT COUNT(*) FROM store WHERE branch_id = ?", id).Scan(&storeCount)
	if storeCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"detail": "لا يمكن حذف الفرع — يوجد مستودعات مرتبطة به",
		})
		return
	}

	res, err := h.DB.Exec("DELETE FROM branches WHERE id = ?", id)
	if err != nil {
		log.Printf("ERROR DeleteBranch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to delete branch"})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "branch not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

// ============================================================================
// ZATCA Config (separate table: branch_zatca_config)
// ============================================================================

// ── GET /api/v2/branch/:id/zatca ────────────────────────────────────────────
// Returns ZATCA configuration and registration status for a branch.
// Returns config fields + status indicators, NOT actual credentials/private keys.

func (h *handler) GetBranchZatcaConfig(c *gin.Context) {
	BranchID, err := strconv.Atoi(c.Param("id"))
	branchID := uint32(BranchID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return
	}

	config, err := h.queries.GetZatcaBranchConfig(c.Request.Context(), branchID)
	// No config row yet → return empty config with not_registered status
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, config)
}

// ── PUT /api/v2/branch/:id/zatca ────────────────────────────────────────────
// Saves ZATCA configuration for a branch. User-editable fields only.
// Credentials (zatca_csr, zatca_private_key, etc.) are set by the registration CLI.
// Uses INSERT ... ON DUPLICATE KEY UPDATE for upsert.

func (h *handler) UpdateBranchZatcaConfig(c *gin.Context) {
	BranchID, err := strconv.Atoi(c.Param("id"))
	branchID := uint32(BranchID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return
	}

	var req db.UpdateZatcaBranchConfigParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	// Verify branch exists
	var exists int
	h.DB.QueryRow("SELECT COUNT(*) FROM branches WHERE id = ?", branchID).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "branch not found"})
		return
	}

	// Defaults
	if req.CsrCountry == "" {
		req.CsrCountry = "SA"
	}
	if req.BusinessCategory == "" {
		req.BusinessCategory = "Supply activities"
	}

	// Upsert: INSERT or UPDATE user-editable fields only
	// Credential fields (zatca_csr, zatca_private_key, etc.) are NOT touched here.
	err = h.queries.UpdateZatcaBranchConfig(c.Request.Context(), req)
	if err != nil {
		log.Printf("ERROR UpdateBranchZatcaConfig: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to save ZATCA config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

func (h *handler) OnboardBranchZatca(c *gin.Context) {
	BranchID, err := strconv.Atoi(c.Param("id"))
	branchID := uint32(BranchID)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return
	}

	var req OTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "OTP required"})
		return
	}

	otp := strings.TrimSpace(req.OTP)
	if len(otp) != 6 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "OTP must be 6 numbers"})
		return
	}

	if err := h.pub.OnboadBranch(int64(branchID), req.OTP); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
}
