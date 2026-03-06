package handlers

import (
	"errors"
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

func (h *handler) GetClient(c *gin.Context) {
	// user := GetSessionInfo(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	res, err := h.queries.GetClientByID(c.Request.Context(), uint32(id))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	c.JSON(http.StatusOK, res)

}

func (h *handler) GetAllClient(c *gin.Context) {
	request := model.PaginationRequest{
		Page:     0,
		PageSize: 10,
	}

	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}
	res, err := h.queries.GetClients(c.Request.Context(), db.GetClientsParams{Limit: request.PageSize, Offset: request.Page * request.PageSize})
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		log.Panic("Error in query", err)
	}

	c.JSON(http.StatusOK, res)

}

type CreateClientRequest struct {
	Name        string  `json:"name" binding:"required"`
	CompanyName *string `json:"company_name" binding:"required"`
	Email       *string `json:"email" binding:"required"`
	Phone       *string `json:"phone" binding:"required"`
	Address     *string `json:"address" binding:"required"`
	VatNumber   string  `json:"vat_number" binding:"required"`
}

func (h *handler) CreateClient(c *gin.Context) {

	var request CreateClientRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}
	query := db.CreateClientParams{
		Name:        request.Name,
		CompanyName: request.CompanyName,
		Email:       request.Email,
		Address:     request.Address,
		Phone:       request.Phone,
		VatNumber:   request.VatNumber,
	}
	err := h.queries.CreateClient(c.Request.Context(), query)
	if err != nil {
		if IsDuplicate(err) {
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("Client vat number already exists in this store"))
		} else {
			c.AbortWithError(http.StatusInternalServerError, err)
		}
		log.Panic("Error in query", err)
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
		CompanyName: request.CompanyName,
		Email:       request.Email,
		Address:     request.Address,
		Phone:       request.Phone,
		VatNumber:   request.VatNumber,
		ID:          uint32(id),
	}
	err = h.queries.UpdateClient(c.Request.Context(), query)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic("Error in query", err)
	}

	c.Status(http.StatusCreated)
}

func IsDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
