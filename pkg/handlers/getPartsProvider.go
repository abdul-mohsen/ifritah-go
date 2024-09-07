package handlers

import (
	"fmt"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)


func (h * handler) GetPartsProvider(c *gin.Context) {

  fullTokenString := c.Request.Header.Get("Authorization")
  tokenString := strings.Split(fullTokenString, "Bearer ")[1]
  fmt.Println(tokenString)
  token, err := VerifyToken(tokenString)
  if err != nil {
    log.Fatal(err)
  }
  userSession := GetSessionInfo(*token)


  var id int
  if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&id); err != nil {
    log.Fatal(err)
  }
  fmt.Println(id)
  fmt.Println("_id")


  rows, err := h.DB.Query("SELECT * FROM parts_provider where company_id = ? and is_deleted = TRUE", id)

  if err != nil {
    log.Fatal(err)
  }
  var partsProviders []model.PartsProvider
  for rows.Next() {
    var partsProvider model.PartsProvider

    if err := rows.Scan(&model.PartsProvider.Id, &model.PartsProvider.Copany_id, &model.PartsProvider.Name, &model.PartsProvider.Address, &model.PartsProvider.PhoneNumber, &model.PartsProvider.Number, &model.PartsProvider.VatNumber, &model.PartsProvider.IsDeleted); err != nil {
      log.Fatal(err)
    }
    fmt.Println(partsProvider);
    partsProviders = append(partsProviders, partsProvider)
  }
  defer rows.Close()
  c.IndentedJSON(http.StatusOK, partsProviders)

}

