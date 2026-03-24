package handlers

import (
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
		log.Panic(err)
	}
	var companyResponses []CompanyResponse
	for rows.Next() {
		var companyResponse CompanyResponse

		if err := rows.Scan(&companyResponse.Id, &companyResponse.Name); err != nil {
			log.Panic(err)
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
		log.Panic("ERROR CreateBranch: could not get user company: %v", err)
	}
	return companyID

}
