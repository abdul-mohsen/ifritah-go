package handlers

import (
	"fmt"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dgrijalva/jwt-go"

	"github.com/gin-gonic/gin"
)

func verifyToken(tokenString string) (*jwt.Token, error) {
  // Parse the token with the secret key
  fmt.Println(tokenString)
  token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    key := os.Getenv("JWT_SECRET_KEY")
    fmt.Println(key)
    fmt.Println("_0")
    return key, nil
  })
  fmt.Println(token)
  fmt.Println("_1")

  // Check for verification errors
  if err != nil {
    return nil, err
  }
  fmt.Println("_2")

  // Check if the token is valid
  if !token.Valid {
    return nil, fmt.Errorf("invalid token")
  }
  fmt.Println("_3")

  // Return the verified token
  return token, nil
}

func (h * handler) GetPartsProvider(c *gin.Context) {

  fullTokenString := c.Request.Header.Get("Authorization")
  tokenString := strings.Split(fullTokenString, "Bearer ")[1]
  fmt.Println(tokenString)
  token, err := verifyToken(tokenString)
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

