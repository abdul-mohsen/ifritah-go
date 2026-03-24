package handlers

// ============================================================================
// Branch CRUD + ZATCA Config
// ============================================================================
// Copy to: pkg/handlers/handler_branch.go
//
// Endpoints:
//   POST   /api/v2/branch/all         → ListBranches
//   GET    /api/v2/branch/:id         → GetBranch
//   POST   /api/v2/branch             → CreateBranch
//   PUT    /api/v2/branch/:id         → UpdateBranch
//   DELETE /api/v2/branch/:id         → DeleteBranch
//   GET    /api/v2/branch/:id/zatca   → GetBranchZatcaConfig
//   PUT    /api/v2/branch/:id/zatca   → UpdateBranchZatcaConfig
//
// DATABASE:
//   - `branches` table (already exists):
//       id INT UNSIGNED PK, name, address, city, phone,
//       company_id, manager_id, is_active, created_at
//   - `branch_zatca_config` table (separate, created by migration_settings_zatca.sql):
//       branch_id PK/FK → branches(id) ON DELETE CASCADE
//       csr_org_*, business_category, seller_vat, seller_crn,
//       street, building, district, postal_code, zatca_otp,
//       zatca_csr, zatca_private_key, zatca_compliance_*, zatca_production_*
//   - `store` table has branch_id FK → branches(id)
//
// Follows same patterns as handler_stock.go:
//   - (h *handler) method receiver
//   - h.DB for database access
//   - GetSessionInfo(c) for current user
// ============================================================================

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ── POST /api/v2/branch/all ─────────────────────────────────────────────────
// Returns all branches with store count and ZATCA registration status.

func (h *handler) ListBranches(c *gin.Context) {
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
		ORDER BY b.id
	`)
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

	if branches == nil {
		branches = []branchRow{}
	}
	c.JSON(http.StatusOK, gin.H{"data": branches})
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
		CompanyID *int   `json:"company_id"`
		ManagerID *int   `json:"manager_id"`
		IsActive  *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "الاسم مطلوب"})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	result, err := h.DB.Exec(`
		INSERT INTO branches (name, address, city, phone, company_id, manager_id, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, req.Name, req.Address, req.City, req.Phone, req.CompanyID, req.ManagerID, isActive)
	if err != nil {
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
		CompanyID *int   `json:"company_id"`
		ManagerID *int   `json:"manager_id"`
		IsActive  *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	res, err := h.DB.Exec(`
		UPDATE branches SET name=?, address=?, city=?, phone=?,
		       company_id=?, manager_id=?, is_active=?
		WHERE id=?
	`, req.Name, req.Address, req.City, req.Phone,
		req.CompanyID, req.ManagerID, isActive, id)
	if err != nil {
		log.Printf("ERROR UpdateBranch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update branch"})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "branch not found"})
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
	branchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return
	}

	// Verify branch exists
	var branchName string
	err = h.DB.QueryRow("SELECT name FROM branches WHERE id = ?", branchID).Scan(&branchName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "branch not found"})
		return
	}

	// Try to get ZATCA config (may not exist yet)
	var orgID, orgUnit, orgName, csrCountry, csrLoc, bizCat string
	var vat, crn, street, building, district, postal, otp string
	var csrLen, prodLen int
	var registeredAt *string

	err = h.DB.QueryRow(`
		SELECT
			COALESCE(csr_org_identifier,''), COALESCE(csr_org_unit,''),
			COALESCE(csr_org_name,''), COALESCE(csr_country,'SA'),
			COALESCE(csr_location,''), COALESCE(business_category,'Supply activities'),
			COALESCE(seller_vat,''), COALESCE(seller_crn,''),
			COALESCE(street,''), COALESCE(building,''), COALESCE(district,''),
			COALESCE(postal_code,''), COALESCE(zatca_otp,''),
			LENGTH(COALESCE(zatca_csr,'')),
			LENGTH(COALESCE(zatca_production_username,'')),
			zatca_registered_at
		FROM branch_zatca_config WHERE branch_id = ?
	`, branchID).Scan(
		&orgID, &orgUnit, &orgName, &csrCountry, &csrLoc, &bizCat,
		&vat, &crn, &street, &building, &district, &postal, &otp,
		&csrLen, &prodLen, &registeredAt,
	)

	// No config row yet → return empty config with not_registered status
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"detail": gin.H{
				"branch_id":   branchID,
				"branch_name": branchName,
				"config": gin.H{
					"csr_org_identifier": "", "csr_org_unit": "", "csr_org_name": "",
					"csr_country": "SA", "csr_location": "",
					"business_category": "Supply activities",
					"seller_vat":        "", "seller_crn": "",
					"street": "", "building": "", "district": "", "postal_code": "",
					"zatca_otp": "",
				},
				"status":         "not_registered",
				"has_csr":        false,
				"has_production": false,
				"registered_at":  nil,
			},
		})
		return
	}

	status := "not_registered"
	if prodLen > 0 {
		status = "registered"
	} else if csrLen > 0 {
		status = "compliance_only"
	}

	c.JSON(http.StatusOK, gin.H{
		"detail": gin.H{
			"branch_id":   branchID,
			"branch_name": branchName,
			"config": gin.H{
				"csr_org_identifier": orgID, "csr_org_unit": orgUnit,
				"csr_org_name": orgName, "csr_country": csrCountry,
				"csr_location": csrLoc, "business_category": bizCat,
				"seller_vat": vat, "seller_crn": crn,
				"street": street, "building": building,
				"district": district, "postal_code": postal,
				"zatca_otp": otp,
			},
			"status":         status,
			"has_csr":        csrLen > 0,
			"has_production": prodLen > 0,
			"registered_at":  registeredAt,
		},
	})
}

// ── PUT /api/v2/branch/:id/zatca ────────────────────────────────────────────
// Saves ZATCA configuration for a branch. User-editable fields only.
// Credentials (zatca_csr, zatca_private_key, etc.) are set by the registration CLI.
// Uses INSERT ... ON DUPLICATE KEY UPDATE for upsert.

func (h *handler) UpdateBranchZatcaConfig(c *gin.Context) {
	branchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid branch ID"})
		return
	}

	var req struct {
		CsrOrgIdentifier string `json:"csr_org_identifier"`
		CsrOrgUnit       string `json:"csr_org_unit"`
		CsrOrgName       string `json:"csr_org_name"`
		CsrCountry       string `json:"csr_country"`
		CsrLocation      string `json:"csr_location"`
		BusinessCategory string `json:"business_category"`
		SellerVat        string `json:"seller_vat"`
		SellerCrn        string `json:"seller_crn"`
		Street           string `json:"street"`
		Building         string `json:"building"`
		District         string `json:"district"`
		PostalCode       string `json:"postal_code"`
		ZatcaOtp         string `json:"zatca_otp"`
	}
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
	_, err = h.DB.Exec(`
		INSERT INTO branch_zatca_config
			(branch_id, csr_org_identifier, csr_org_unit, csr_org_name,
			 csr_country, csr_location, business_category,
			 seller_vat, seller_crn, street, building, district, postal_code, zatca_otp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			csr_org_identifier = VALUES(csr_org_identifier),
			csr_org_unit = VALUES(csr_org_unit),
			csr_org_name = VALUES(csr_org_name),
			csr_country = VALUES(csr_country),
			csr_location = VALUES(csr_location),
			business_category = VALUES(business_category),
			seller_vat = VALUES(seller_vat),
			seller_crn = VALUES(seller_crn),
			street = VALUES(street),
			building = VALUES(building),
			district = VALUES(district),
			postal_code = VALUES(postal_code),
			zatca_otp = VALUES(zatca_otp)
	`, branchID, req.CsrOrgIdentifier, req.CsrOrgUnit, req.CsrOrgName,
		req.CsrCountry, req.CsrLocation, req.BusinessCategory,
		req.SellerVat, req.SellerCrn, req.Street, req.Building,
		req.District, req.PostalCode, req.ZatcaOtp)
	if err != nil {
		log.Printf("ERROR UpdateBranchZatcaConfig: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to save ZATCA config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}
