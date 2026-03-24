package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Store struct {
	Id        int     `json:"id"`
	AddressId *int    `json:"address_id"`
	Name      *string `json:"name"`
}

func (h *handler) getStores(user userSession) []Store {

	rows, err := h.DB.Query(`select store.id, addressId, store.name from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id`, user.id)

	if err != nil {
		log.Panic(err)
	}

	var stores []Store

	for rows.Next() {
		var store Store
		if err := rows.Scan(&store.Id, &store.AddressId, &store.Name); err != nil {
			log.Panic(err)
		}
		stores = append(stores, store)
	}

	return stores

}

func (h *handler) getStoreIds(c *gin.Context) []int32 {

	userSession := GetSessionInfo(c)
	rows, err := h.DB.Query(`select store.id from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id`, userSession.id)

	if err != nil {
		log.Panic(err)
	}

	var ids []int32

	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err != nil {
			log.Panic(err)
		}
		ids = append(ids, id)
	}

	return ids
}

func (h *handler) GetStores(c *gin.Context) {
	c.JSON(http.StatusOK, h.getStores(GetSessionInfo(c)))
}

// ── GET /api/v2/store/:id ───────────────────────────────────────────────────
// Returns a single store with branch info and national address fields.

func (h *handler) GetStore(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid store ID"})
		return
	}

	var s struct {
		ID             int    `json:"id"`
		Name           string `json:"name"`
		Status         string `json:"status"`
		CompanyID      *int   `json:"company_id"`
		BranchID       *int   `json:"branch_id"`
		BranchName     string `json:"branch_name"`
		AddressName    string `json:"address_name"`
		BuildingNumber string `json:"building_number"`
		StreetName     string `json:"street_name"`
		District       string `json:"district"`
		City           string `json:"city"`
		Region         string `json:"region"`
		PostalCode     string `json:"postal_code"`
		AdditionalNum  string `json:"additional_number"`
		UnitNumber     string `json:"unit_number"`
		Country        string `json:"country"`
		CreatedAt      string `json:"created_at"`
		UpdatedAt      string `json:"updated_at"`
	}

	err = h.DB.QueryRow(`
		SELECT s.id, s.name, COALESCE(s.status,''), s.company_id,
		       s.branch_id, COALESCE(b.name,'') AS branch_name,
		       COALESCE(s.address_name,''),
		       COALESCE(s.building_number,''), COALESCE(s.street_name,''),
		       COALESCE(s.district,''), COALESCE(s.city,''),
		       COALESCE(s.region,''), COALESCE(s.postal_code,''),
		       COALESCE(s.additional_number,''), COALESCE(s.unit_number,''),
		       COALESCE(s.country,'SA'),
		       s.created_at, s.updated_at
		FROM store s
		LEFT JOIN branches b ON b.id = s.branch_id
		WHERE s.id = ?
	`, id).Scan(&s.ID, &s.Name, &s.Status, &s.CompanyID,
		&s.BranchID, &s.BranchName, &s.AddressName,
		&s.BuildingNumber, &s.StreetName, &s.District, &s.City,
		&s.Region, &s.PostalCode, &s.AdditionalNum, &s.UnitNumber,
		&s.Country, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "store not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": s})
}

// ── POST /api/v2/store ──────────────────────────────────────────────────────
// Creates a new store, optionally linked to a branch.

func (h *handler) CreateStore(c *gin.Context) {
	var req struct {
		Name           string `json:"name" binding:"required"`
		BranchID       *int   `json:"branch_id"`
		AddressName    string `json:"address_name"`
		BuildingNumber string `json:"building_number"`
		StreetName     string `json:"street_name"`
		District       string `json:"district"`
		City           string `json:"city"`
		Region         string `json:"region"`
		PostalCode     string `json:"postal_code"`
		AdditionalNum  string `json:"additional_number"`
		UnitNumber     string `json:"unit_number"`
		Country        string `json:"country"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "الاسم مطلوب"})
		return
	}

	// Validate branch_id if provided
	if req.BranchID != nil {
		var exists int
		h.DB.QueryRow("SELECT COUNT(*) FROM branches WHERE id = ?", *req.BranchID).Scan(&exists)
		if exists == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "الفرع غير موجود"})
			return
		}
	}

	// Get company_id from authenticated user's session (set in JWT claims)
	companyID, err := h.getUserCompany(c)
	if err != nil {
		log.Printf("ERROR CreateBranch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to create branch"})
		return
	}

	country := req.Country
	if country == "" {
		country = "SA"
	}

	result, err := h.DB.Exec(`
		INSERT INTO store (name, company_id, branch_id, address_name,
		       building_number, street_name, district, city, region,
		       postal_code, additional_number, unit_number, country)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, req.Name, companyID, req.BranchID, req.AddressName,
		req.BuildingNumber, req.StreetName, req.District, req.City,
		req.Region, req.PostalCode, req.AdditionalNum, req.UnitNumber, country)
	if err != nil {
		log.Printf("ERROR CreateStore: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to create store"})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"detail": gin.H{
			"id":   id,
			"name": req.Name,
		},
	})
}

// ── PUT /api/v2/store/:id ───────────────────────────────────────────────────
// Updates an existing store.

func (h *handler) UpdateStore(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid store ID"})
		return
	}

	var req struct {
		Name           string `json:"name" binding:"required"`
		BranchID       *int   `json:"branch_id"`
		AddressName    string `json:"address_name"`
		BuildingNumber string `json:"building_number"`
		StreetName     string `json:"street_name"`
		District       string `json:"district"`
		City           string `json:"city"`
		Region         string `json:"region"`
		PostalCode     string `json:"postal_code"`
		AdditionalNum  string `json:"additional_number"`
		UnitNumber     string `json:"unit_number"`
		Country        string `json:"country"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	// Get company_id from authenticated user's session (set in JWT claims)
	companyID, err := h.getUserCompany(c)
	if err != nil {
		log.Printf("ERROR CreateBranch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to create branch"})
		return
	}

	// Validate branch_id if provided
	if req.BranchID != nil {
		var exists int
		h.DB.QueryRow("SELECT COUNT(*) FROM branches WHERE id = ?", *req.BranchID).Scan(&exists)
		if exists == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "الفرع غير موجود"})
			return
		}
	}

	country := req.Country
	if country == "" {
		country = "SA"
	}

	res, err := h.DB.Exec(`
		UPDATE store SET name=?, company_id=?, branch_id=?, address_name=?,
		       building_number=?, street_name=?, district=?, city=?, region=?,
		       postal_code=?, additional_number=?, unit_number=?, country=?
		WHERE id=?
	`, req.Name, companyID, req.BranchID, req.AddressName,
		req.BuildingNumber, req.StreetName, req.District, req.City,
		req.Region, req.PostalCode, req.AdditionalNum, req.UnitNumber,
		country, id)
	if err != nil {
		log.Printf("ERROR UpdateStore: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update store"})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "store not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

// ── DELETE /api/v2/store/:id ────────────────────────────────────────────────
// Deletes a store. Prevents deletion if it has linked bills or products.

func (h *handler) DeleteStore(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid store ID"})
		return
	}

	// Check for referenced bills
	var billCount int
	h.DB.QueryRow("SELECT COUNT(*) FROM bill WHERE store_id = ?", id).Scan(&billCount)
	if billCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"detail": "لا يمكن حذف المستودع — يوجد فواتير مرتبطة به",
		})
		return
	}

	// Check for referenced products
	var prodCount int
	h.DB.QueryRow("SELECT COUNT(*) FROM product WHERE store_id = ? AND is_deleted = 0", id).Scan(&prodCount)
	if prodCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"detail": "لا يمكن حذف المستودع — يوجد منتجات مرتبطة به",
		})
		return
	}

	res, err := h.DB.Exec("DELETE FROM store WHERE id = ?", id)
	if err != nil {
		log.Printf("ERROR DeleteStore: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to delete store"})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "store not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}
