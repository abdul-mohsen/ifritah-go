package handlers

import (
	"fmt"
	"log"
	"net/http"

	"ifritah/web-service-gin/pkg/model"

	"github.com/gin-gonic/gin"
)

func (h * handler) GetCarPartDetail(c *gin.Context) {
  id := c.Param("id") 
  rows, err := h.DB.Query("SELECT * FROM articles where id = ?", id)

  if err != nil {
    log.Fatal(err)
  }
  var articales []model.ArticleTable
  for rows.Next() {
    var articale model.ArticleTable
    if err := rows.Scan(&articale); err != nil {
      log.Fatal(err)
    }
    fmt.Println(articale);
    articales = append(articales, articale)
  }
  defer rows.Close()
  c.IndentedJSON(http.StatusOK, articales)

}
