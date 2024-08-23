package handlers

import (
  "fmt"
  "log"
  "net/http"
  "github.com/dgrijalva/jwt-go"
  "ifritah/web-service-gin/pkg/model"

  "github.com/gin-gonic/gin"
)

func (h * handler) GetPartsProvider(c *gin.Context) {

  auth := c.Request.Header.Get("Authorization")
  token, err := jwt.Parse(auth, func(token *jwt.Token) (interface{}, error) {
    _, ok := token.Method.(*jwt.SigningMethodECDSA)
    if !ok {
      log.Fatal("fuck")
    }
    fmt.Println(ok)
    return ok, nil
  })
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

func verifyJWT(endpointHandler func(writer http.ResponseWriter, request *http.Request)) http.HandlerFunc {

}
