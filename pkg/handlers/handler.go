package handlers

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

type handler struct {
  DB *sql.DB
}

type userSession struct {
  id float64 
  username string
  exp float64
}

func New(db *sql.DB) handler {
  return handler{db}
}

func VerifyToken(c *gin.Context) (*jwt.Token, error) {
  fullTokenString := c.Request.Header.Get("Authorization")
  tokenString := strings.Split(fullTokenString, "Bearer ")[1]
  fmt.Println(tokenString)
  // Parse the token with the secret key
  fmt.Println(tokenString)
  token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    key := os.Getenv("JWT_SECRET_KEY")
    fmt.Println(key)
    fmt.Println("_0")
    return []byte(key), nil
  })
  fmt.Println(token)

  // Check for verification errors
  if err != nil {
    return nil, err
  }

  // Check if the token is valid
  if !token.Valid {
    return nil, fmt.Errorf("invalid token")
  }

  // Return the verified token
  return token, nil
}

func GetSessionInfo(token jwt.Token ) userSession {

  claims := token.Claims.(jwt.MapClaims)
  user := userSession{
    id: claims["userId"].(float64),
    username: claims["username"].(string),
    exp : claims["exp"].(float64),
  }
  return user
}
