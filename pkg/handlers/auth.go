package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
  Username string `json:"username"`
  Password string `json:"password"`
  Address string `json:"address"`
  PhoneNumber string `json:"phone_number"`
  Email string `json:"Email"`
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
