package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type BillCredit struct {
	BillId int    `json:"bill_id" binding:"required"`
	Note   string `json:"note" binding:"required"`
}

func (h *handler) CreditBill(c *gin.Context) {

	// Need to check the user if he is auth to do that
	// userSession := GetSessionInfo(c)

	var request BillCredit

	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
		c.Status(http.StatusBadRequest)
	}

	query := `
	insert into credit_note (bill_id, state, note)
	values (?, ?, ?); 
	`
	_, err := h.DB.Exec(query, request.BillId, 1, request.Note)
	if err != nil {
		log.Panic(err)
	}

	c.Status(http.StatusCreated)
}
