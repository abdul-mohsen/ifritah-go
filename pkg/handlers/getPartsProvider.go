package handlers

import (
	"fmt"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"

	"github.com/dgrijalva/jwt-go"

	"github.com/gin-gonic/gin"
)

func (h * handler) GetPartsProvider(c *gin.Context) {

  tokenString := c.Request.Header.Get("Authorization")
  fmt.Println(tokenString)
  token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {

    sampleSecretKey := []byte("hi")
    if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
      return nil, fmt.Errorf("there's an error with the signing method")
    }
    return sampleSecretKey, nil

  })
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println(token)

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

