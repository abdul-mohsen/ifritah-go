package handlers

import (
	"ifritah/web-service-gin/pkg/pagination"
	"log"
	"net/http"
	"strconv"
	"strings"

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
		log.Printf("getStores query: %v", err)
		return nil
	}

	var stores []Store

	for rows.Next() {
		var store Store
		if err := rows.Scan(&store.Id, &store.AddressId, &store.Name); err != nil {
			log.Printf("getStores scan: %v", err)
			return nil
		}
		stores = append(stores, store)
	}

	return stores

}

func (h *handler) getStoreIds(c *gin.Context) []int32 {

	userSession := GetSessionInfo(c)
	rows, err := h.DB.Query(`select store.id from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id`, userSession.id)

	if err != nil {
		log.Printf("getStoreIds query: %v", err)
		return nil
	}

	var ids []int32

	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err != nil {
			log.Printf("getStoreIds scan: %v", err)
			return nil
		}
		ids = append(ids, id)
	}

	return ids
}

// GetStores returns the calling user's accessible stores wrapped in
// the standard cursor-pagination envelope. Stores per company are
// bounded (typically <10), so we return the full set in one page —
// no real seek; HasMore is always false. Keeping the envelope shape
// keeps the wire contract uniform across all list endpoints.
//
// Search (FE §1): a free-text `query` is matched case-insensitively
// against `name`. Filter is applied in-memory because the row count
// is bounded and pushing it to SQL would mean teaching the legacy
// `getStores` query about a new arg path.
// storesListRequest is the wire shape for GetStores. POST clients
// send it as a JSON body; GET clients send the same fields via the
// query string and we overlay them in applyStoresQueryStringOverlay.
type storesListRequest struct {
	Query *string `json:"query"`
	Sort  string  `json:"sort"`
	Dir   string  `json:"dir"`
}

// applyStoresQueryStringOverlay copies non-empty `?query=&sort=&dir=`
// query-string params on top of the JSON body so GET and POST share
// one code path. Query-string wins when both are present.
func applyStoresQueryStringOverlay(c *gin.Context, req *storesListRequest) {
	if q := c.Query("query"); q != "" {
		v := q
		req.Query = &v
	}
	if s := c.Query("sort"); s != "" {
		req.Sort = s
	}
	if d := c.Query("dir"); d != "" {
		req.Dir = d
	}
}

// filterStoresByQuery does a case-insensitive substring match on
// Store.Name. Bounded slice → in-memory is the right tool here.
func filterStoresByQuery(stores []Store, query *string) []Store {
	if query == nil || *query == "" {
		return stores
	}
	needle := strings.ToLower(*query)
	out := make([]Store, 0, len(stores))
	for _, s := range stores {
		name := ""
		if s.Name != nil {
			name = strings.ToLower(*s.Name)
		}
		if strings.Contains(name, needle) {
			out = append(out, s)
		}
	}
	return out
}

// storeSortCmp wires the FE table-sort UX for /stores/all.
func storeSortCmp(a, b Store, k string) (int, bool) {
	switch k {
	case "id":
		return int64Cmp(int64(a.Id), int64(b.Id)), true
	case "name":
		return strPtrCmp(a.Name, b.Name), true
	}
	return 0, false
}

func (h *handler) GetStores(c *gin.Context) {
	var req storesListRequest
	// Best-effort JSON bind (POST clients). GET callers send no body.
	_ = c.ShouldBindJSON(&req)
	applyStoresQueryStringOverlay(c, &req)

	stores := h.getStores(GetSessionInfo(c))
	if stores == nil {
		stores = []Store{}
	}
	stores = filterStoresByQuery(stores, req.Query)
	applyListSort(stores, req.Sort, req.Dir, storeSortCmp)

	c.JSON(http.StatusOK, pagination.Envelope[Store]{Items: stores})
}

// ── GET /api/v2/store/:id ───────────────────────────────────────────────────
// Returns a single store with branch info and national address fields.

func (h *handler) GetStore(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("GetStore: %v", err)
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
		log.Printf("GetStore: %v", err)
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
		log.Printf("CreateStore: %v", err)
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
	companyID := h.getUserCompany(c)

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
		log.Printf("UpdateStore: %v", err)
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
		log.Printf("UpdateStore: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	// Get company_id from authenticated user's session (set in JWT claims)
	companyID := h.getUserCompany(c)

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

	_, err = h.DB.Exec(`
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

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

// ── DELETE /api/v2/store/:id ────────────────────────────────────────────────
// Deletes a store. Prevents deletion if it has linked bills or products.

func (h *handler) DeleteStore(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("DeleteStore: %v", err)
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
