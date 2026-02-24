package handlers

import (
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Pagination struct {
	Query    string `json:"query"`
	Page     int32  `json:"page"`
	PageSize int32  `json:"page_size"`
}

func (h *handler) GetClient(c *gin.Context) {
	// user := GetSessionInfo(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	res, err := h.queries.GetClientByID(c.Request.Context(), uint32(id))
	if err != nil {
		fmt.Println("Error in query", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, res)

}

func (h *handler) GetAllClient(c *gin.Context) {
	request := Pagination{
		Page:     0,
		PageSize: 10,
	}

	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}
	res, err := h.queries.GetClients(c.Request.Context(), db.GetClientsParams{Limit: request.Page, Offset: request.PageSize})
	if err != nil {
		fmt.Println("Error in query", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, res)

}

type CreateClientRequest struct {
	Name        string `json:"name" binding:"required"`
	CompanyName string `json:"company_name" binding:"required"`
	Email       string `json:"email" binding:"required"`
	Phone       string `json:"phone" binding:"required"`
	Address     string `json:"address" binding:"required"`
	VatNumber   string `json:"vat_number" binding:"required"`
}

func (h *handler) CreateClient(c *gin.Context) {

	var request CreateClientRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}
	query := db.CreateClientParams{
		Name:        request.Name,
		CompanyName: NewNullString(request.CompanyName),
		Email:       NewNullString(request.Email),
		Address:     NewNullString(request.Address),
		Phone:       NewNullString(request.Phone),
		VatNumber:   request.VatNumber,
	}
	err := h.queries.CreateClient(c.Request.Context(), query)
	if err != nil {
		fmt.Println("Error in query", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusCreated)
}

func (h *handler) UpdateClient(c *gin.Context) {

	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	var request CreateClientRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}

	query := db.UpdateClientParams{
		Name:        request.Name,
		CompanyName: NewNullString(request.CompanyName),
		Email:       NewNullString(request.Email),
		Address:     NewNullString(request.Address),
		Phone:       NewNullString(request.Phone),
		VatNumber:   request.VatNumber,
		ID:          uint32(id),
	}
	err = h.queries.UpdateClient(c.Request.Context(), query)
	if err != nil {
		fmt.Println("Error in query", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusCreated)
}
