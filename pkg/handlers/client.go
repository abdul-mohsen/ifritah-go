package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
)

func (h *handler) GetClient(c *gin.Context) {
	// user := GetSessionInfo(c)
	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	id := uint64(Id)

	res, err := h.queries.GetClientByID(c.Request.Context(), uint32(id))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, res)

}

const clientListSort = "-updated_at"

// errGetAllClient is the log-prefix format used by every error path
// in GetAllClient (Sonar S1192).
const errGetAllClient = "GetAllClient: %v"

func (h *handler) GetAllClient(c *gin.Context) {
	request := model.PaginationRequest{}

	if err := c.BindJSON(&request); err != nil {
		log.Printf(errGetAllClient, err)
		c.Status(http.StatusBadRequest)
		return
	}

	listReq := listRequestFromPagination(request)
	if err := listReq.Validate(clientListSort); err != nil {
		log.Printf(errGetAllClient, err)
		c.Status(http.StatusBadRequest)
		return
	}

	cur, _ := listReq.DecodedCursor()
	cursorUpdatedAt, cursorID, ok := cursorDateAndID(cur)
	if !ok {
		log.Printf("GetAllClient: malformed cursor")
		c.Status(http.StatusBadRequest)
		return
	}
	// Cursor uses sql.NullInt64 in the gen for client (the CAST in the
	// derived-table param wrapper bumps the inferred type).
	var cursorIDNI sql.NullInt64
	if cursorID != nil {
		cursorIDNI = sql.NullInt64{Int64: int64(*cursorID), Valid: true}
	}

	limit := listReq.EffectiveLimit()

	queryLike, _ := buildLikeAndDigitsExact(request.Query)

	phonePrefix := buildPlainPrefixFilter(request.Phone)
	vatPrefix := buildPlainPrefixFilter(request.VatNumber)
	crPrefix := buildPlainPrefixFilter(request.CommercialRegistration)

	res, err := h.queries.GetClients(c.Request.Context(), db.GetClientsParams{
		QueryLike:         queryLike,
		FilterPhonePrefix: phonePrefix,
		FilterVatPrefix:   vatPrefix,
		FilterCrPrefix:    crPrefix,
		CursorUpdatedAt:   cursorUpdatedAt,
		CursorID:          cursorIDNI,
		Limit:             int32(limit + 1),
	})
	if err != nil {
		log.Printf(errGetAllClient, err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	envelope := pagination.BuildEnvelope(
		res,
		limit,
		clientListSort,
		func(cl db.Client) []any {
			return []any{cl.UpdatedAt.UTC().Format(time.RFC3339Nano), cl.ID}
		},
	)
	c.JSON(http.StatusOK, envelope)
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
		return
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
		return
	}

	c.Status(http.StatusCreated)
}

func (h *handler) UpdateClient(c *gin.Context) {

	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	id := uint64(Id)

	var request CreateClientRequest
	if err := c.BindJSON(&request); err != nil {
		return
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
		return
	}

	c.Status(http.StatusCreated)
}

func (h *handler) DeleteClient(c *gin.Context) {
	// user := GetSessionInfo(c)
	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	id := uint64(Id)

	err = h.queries.DeleteClient(c.Request.Context(), uint32(id))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)

}

func IsDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
