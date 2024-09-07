package handlers

import (
	"fmt"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
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

  claims := token.Claims.(jwt.MapClaims)
  fmt.Println(claims["userId"])

  id := 1
  rows, err := h.DB.Query("SELECT name, address, phone_number, number, vat_number FROM parts_provider where company_id = ? and is_deleted = TRUE", id)

  if err != nil {
    log.Fatal(err)
  }
  var partsProviders []model.PartsProvider
  for rows.Next() {
    var partsProvider model.PartsProvider
    if err := rows.Scan(); err != nil {
      log.Fatal(err)
    }
    fmt.Println(partsProvider);
    partsProviders = append(partsProviders, partsProvider)
  }
  defer rows.Close()
  c.IndentedJSON(http.StatusOK, partsProviders)

}

