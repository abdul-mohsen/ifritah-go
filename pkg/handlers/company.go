package handlers

import (
	db "ifritah/web-service-gin/pkg/db/gen"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CompanyResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

func (h *handler) GetAllCompanies(c *gin.Context) {

	rows, err := h.DB.Query("SELECT id, name  FROM company ")

	if err != nil {
		return
	}
	var companyResponses []CompanyResponse
	for rows.Next() {
		var companyResponse CompanyResponse

		if err := rows.Scan(&companyResponse.Id, &companyResponse.Name); err != nil {
			return
		}

		companyResponses = append(companyResponses, companyResponse)
	}
	defer rows.Close()
	c.IndentedJSON(http.StatusOK, companyResponses)
}

func (h *handler) getUserCompany(c *gin.Context) int {
	// Get company_id from authenticated user's profile (NOT from request body)
	session := GetSessionInfo(c)
	var companyID int
	err := h.DB.QueryRow("SELECT COALESCE(company_id,1) FROM user WHERE id = ?", session.id).Scan(&companyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to resolve company"})
		log.Printf("ERROR CreateBranch: could not get user company: %v", err)
		return 0
	}
	return companyID

}

func (h *handler) GetCompany(c *gin.Context) {
	cid := h.getUserCompany(c)
	row, err := h.queries.GetCompanyByID(c.Request.Context(), int32(cid))
	if err != nil {
		log.Printf("GetCompany: %v", err)
		c.JSON(404, gin.H{"detail": "company not found"})
		return
	}
	c.JSON(200, row)
}

func (h *handler) UpdateCompany(c *gin.Context) {
	cid := h.getUserCompany(c)
	var req struct {
		Name   string `json:"name"`
		NameAr string `json:"name_ar" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("UpdateCompany: %v", err)
		c.JSON(400, gin.H{"detail": "name_ar is required"})
		return
	}
	if err := h.queries.UpdateCompany(c.Request.Context(),
		db.UpdateCompanyParams{Name: req.Name, NameAr: &req.NameAr, ID: int32(cid)}); err != nil {
		c.JSON(500, gin.H{"detail": "update failed"})
		return
	}
	c.JSON(200, gin.H{"detail": "success"})
}
