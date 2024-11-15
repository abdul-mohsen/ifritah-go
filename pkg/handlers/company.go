package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CompanyResponse struct {
  Id int `json:"id"`
  Name string `json:"name"`

}

func (h * handler) GetAllCompanies(c *gin.Context) {

  rows, err := h.DB.Query("SELECT id, name  FROM company ")

  if err != nil {
    log.Panic(err)
  }
  var companyResponses[]CompanyResponse
  for rows.Next() {
    var companyResponse CompanyResponse

    if err := rows.Scan(&companyResponse.Id, &companyResponse.Name); err != nil {
      log.Panic(err)
    }

    companyResponses= append(companyResponses, companyResponse)
  }
  defer rows.Close()
  c.IndentedJSON(http.StatusOK, companyResponses)
}
