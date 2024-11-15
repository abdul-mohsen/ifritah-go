package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
  Username string `json:"username"`
  Password string `json:"password"`
  Address string `json:"address"`
  PhoneNumber string `json:"phone_number"`
  Email string `json:"email"`
  CompanyID int `json:"company_id"`
}

func (h * handler) Register(c *gin.Context) {

  var request RegisterRequest
  if err := c.BindJSON(&request); err != nil {
    log.Panic(err)
  }
  hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), 10)
  if err != nil {
    log.Fatalf("Error hashing password: %v", err)
  }

  if _, err := h.DB.Exec("INSERT INTO user (company_id, username, password ) VALUES (?, ?, ?)", request.CompanyID, request.Username, hashedPassword); err != nil {
    log.Panic(err)
  }

  c.Status(http.StatusCreated)
}


type JWTConfig struct {
	AccessSecretKey     string
	RefreshSecretKey    string
	SigningMethod       string
	AccessExpiration    time.Duration
	RefreshExpiration   time.Duration
}

var JWTSettings = JWTConfig{
	AccessSecretKey:     os.Getenv("ACCESS_SECRET_KEY"),
	RefreshSecretKey:    os.Getenv("REFRESH_SECRET_KEY"),
	SigningMethod:       "HS512",
	AccessExpiration:    time.Minute * 15, // Access token expires in 15 minutes
	RefreshExpiration:   time.Hour * 24 * 7, // Refresh token expires in 7 days
}

type LoginRequest struct {
  Username string `json:"username"`
  Password string `json:"password"`
}

func GenerateAccessToken(username string, userid int) (string, error) {
  token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
    "username": username,
    "id": userid,
    "exp":      time.Now().Add(JWTSettings.AccessExpiration).Unix(),
  })

  return token.SignedString([]byte(JWTSettings.AccessSecretKey))
}

func GenerateRefreshToken(username string, userid int) (string, error) {
  token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
    "username": username,
    "id": userid,
    "exp":      time.Now().Add(JWTSettings.RefreshExpiration).Unix(),
  })

  return token.SignedString([]byte(JWTSettings.RefreshSecretKey))
}

// GenerateTokens generates both access and refresh tokens
func (h * handler) Login(c *gin.Context) {

  var request LoginRequest 
  if err := c.BindJSON(&request); err != nil {
    log.Panic(err)
  }

  hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), 10)
  fmt.Println(hashedPassword)

  var id int
  if err := h.DB.QueryRow("SELECT id FROM user where username = ? and password = ?;", request.Username, hashedPassword).Scan(&id); err != nil {
    log.Panic(err)
  }

  if err != nil {
    log.Fatalf("Error hashing password: %v", err)
  }

  accessToken, err := GenerateAccessToken(request.Username, id)
  if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate access token"})
    return
  }

  refreshToken, err := GenerateRefreshToken(request.Username, id)
  if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate refresh token"})
    return
  }

  c.JSON(http.StatusOK, gin.H{
    "access_token":  accessToken,
    "refresh_token": refreshToken,
  })
}

