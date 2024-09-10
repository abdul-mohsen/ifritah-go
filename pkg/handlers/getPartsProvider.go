package handlers

import (
	"fmt"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h * handler) GetPartsProvider(c *gin.Context) {

  token, err := VerifyToken(c)
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


  rows, err := h.DB.Query("SELECT * FROM parts_provider where company_id = ? and is_deleted = FALSE", id)

  if err != nil {
    log.Fatal(err)
  }
  var partsProviders []model.PartsProvider
  for rows.Next() {
    var partsProvider model.PartsProvider

    if err := rows.Scan(&partsProvider.Id, &partsProvider.Copany_id, &partsProvider.Name, &partsProvider.Address, &partsProvider.PhoneNumber, &partsProvider.Number, &partsProvider.VatNumber, &partsProvider.IsDeleted); err != nil {
      log.Fatal(err)
    }
    fmt.Println(partsProvider);
    partsProviders = append(partsProviders, partsProvider)
  }
  defer rows.Close()
  c.IndentedJSON(http.StatusOK, partsProviders)

}

