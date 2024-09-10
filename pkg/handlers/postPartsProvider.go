package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PartsProviderRequest struct {
  Name string `json:"name"`
  Address string `json:"address"`
  PhoneNumber string `json:"phone_number"`
  Number string `json:"number"`
  VatNumber string `json:"vat_number"`
}

func (h * handler) PostPartsProvider(c *gin.Context) {

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

  var request PartsProviderRequest
  if err := c.BindJSON(&request); err != nil {
    log.Fatal(err)
  }
  fmt.Print(request)

  if _, err := h.DB.Exec(
    "INSERT INTO parts_provider (company_id, name, address, phone_number, number, vat_number) VALUES (?, ?, ?, ?, ?, ?)", id, request.Name, request.Address, request.PhoneNumber, request.Number, request.VatNumber); err != nil {
    log.Fatal(err)
  }

  c.IndentedJSON(http.StatusCreated, nil)

}
