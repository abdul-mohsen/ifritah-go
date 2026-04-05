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
	"github.com/shopspring/decimal"
)

func (h *handler) GetClient(c *gin.Context) {
	// user := GetSessionInfo(c)
	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	id := uint64(Id)

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
	Name                   string           `json:"name" binding:"required"`
	CompanyName            *string          `json:"company_name" binding:"required"`
	Email                  *string          `json:"email" binding:"required"`
	Phone                  *string          `json:"phone" binding:"required"`
	Address                *string          `json:"address" binding:"required"`
	VatNumber              string           `json:"vat_number" binding:"required"`
	Number                 *string          `json:"number"`
	BankAccount            *string          `json:"bank_account"`
	PreferredPaymentMethod int8             `json:"preferred_payment_method"`
	CreditLimit            *decimal.Decimal `json:"credit_limit"`
	PaymentTermsDays       int32            `json:"payment_terms_days"`
	ShortAddress           *string          `json:"short_address"`
	CommercialRegistration *string          `json:"commercial_registration"`
}

func (h *handler) CreateClient(c *gin.Context) {

	var request CreateClientRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}

	if request.CreditLimit == nil {
		request.CreditLimit = &decimal.Zero
	}
	query := db.CreateClientParams{
		Name:                   request.Name,
		CompanyName:            request.CompanyName,
		Email:                  request.Email,
		Address:                request.Address,
		Phone:                  request.Phone,
		VatNumber:              request.VatNumber,
		Number:                 request.Number,
		BankAccount:            request.BankAccount,
		PreferredPaymentMethod: request.PreferredPaymentMethod,
		CreditLimit:            *request.CreditLimit,
		PaymentTermsDays:       request.PaymentTermsDays,
		ShortAddress:           request.ShortAddress,
		CommercialRegistration: request.CommercialRegistration,
	}

	err := h.queries.CreateClient(c.Request.Context(), query)
	if err != nil {
		if IsDuplicate(err) {
			c.JSON(http.StatusBadRequest, fmt.Errorf("Client vat number already exists in this store"))
		} else {
			c.AbortWithError(http.StatusInternalServerError, err)
		}
		log.Panic("Error in query", err)
	}

	c.Status(http.StatusCreated)
}

func (h *handler) UpdateClient(c *gin.Context) {

	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	id := uint64(Id)

	var request CreateClientRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}

	if request.CreditLimit == nil {
		request.CreditLimit = &decimal.Zero
	}

	query := db.UpdateClientParams{
		Name:                   request.Name,
		CompanyName:            request.CompanyName,
		Email:                  request.Email,
		Address:                request.Address,
		Phone:                  request.Phone,
		VatNumber:              request.VatNumber,
		ID:                     uint32(id),
		Number:                 request.Number,
		BankAccount:            request.BankAccount,
		PreferredPaymentMethod: request.PreferredPaymentMethod,
		CreditLimit:            *request.CreditLimit,
		PaymentTermsDays:       request.PaymentTermsDays,
		ShortAddress:           request.ShortAddress,
		CommercialRegistration: request.CommercialRegistration,
	}
	err = h.queries.UpdateClient(c.Request.Context(), query)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic("Error in query", err)
	}

	c.Status(http.StatusCreated)
}

func (h *handler) DeleteClient(c *gin.Context) {
	// user := GetSessionInfo(c)
	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	id := uint64(Id)

	err = h.queries.DeleteClient(c.Request.Context(), uint32(id))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	c.Status(http.StatusOK)

}

func IsDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
