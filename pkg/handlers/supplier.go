package handlers

import (
	"database/sql"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type SupplierRequest struct {
	Name                   *string          `json:"name"`
	Address                *string          `json:"address"`
	PhoneNumber            *string          `json:"phone_number"`
	Number                 *string          `json:"number"`
	VatNumber              *string          `json:"vat_number"`
	BankAccount            *string          `json:"bank_account"`
	IsPostPaid             bool             `json:"is_postpaid"`
	CreditLimit            *decimal.Decimal `json:"credit_limit"`
	PaymentTermsDays       int32            `json:"payment_terms_days"`
	PreferredPaymentMethod int32            `json:"preferred_payment_method"`
	CommercialRegistration *string          `json:"commercial_registration"`
	CRN                    string           `json:"crn"`
	ShortAddress           *string          `json:"short_address"`
	Email                  *string          `json:"email"`
}

const supplierListSort = "-id"

// errGetAllSupplier is the log-prefix format used by every error path
// in GetAllSupplier (Sonar S1192).
const errGetAllSupplier = "GetAllSupplier: %v"

func (h *handler) GetAllSupplier(c *gin.Context) {

	request := model.PaginationRequest{}

	if err := c.BindJSON(&request); err != nil {
		log.Printf(errGetAllSupplier, err)
		c.Status(http.StatusBadRequest)
		return
	}

	listReq := listRequestFromPagination(request)
	if err := listReq.Validate(supplierListSort); err != nil {
		log.Printf(errGetAllSupplier, err)
		c.Status(http.StatusBadRequest)
		return
	}

	cur, _ := listReq.DecodedCursor()
	cursorID := cursorIDOnly(cur)
	var cursorIDNullable sql.NullInt64
	if cursorID != nil {
		cursorIDNullable = sql.NullInt64{Int64: int64(*cursorID), Valid: true}
	}

	limit := listReq.EffectiveLimit()

	queryLike, _ := buildLikeAndDigitsExact(request.Query)

	phonePrefix := buildPlainPrefixFilter(request.Phone)
	vatPrefix := buildPlainPrefixFilter(request.VatNumber)
	crPrefix := buildPlainPrefixFilter(request.CommercialRegistration)

	args := db.GetAllSupplierParams{
		QueryLike:   queryLike,
		PhonePrefix: phonePrefix,
		VatPrefix:   vatPrefix,
		CrPrefix:    crPrefix,
		CursorID:    cursorIDNullable,
		Limit:       int32(limit + 1),
	}

	suppliers, err := h.queries.GetAllSupplier(c.Request.Context(), args)
	if err != nil {
		log.Printf(errGetAllSupplier, err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	envelope := pagination.BuildEnvelope(
		suppliers,
		limit,
		supplierListSort,
		func(s db.Supplier) []any { return []any{s.ID} },
	)
	c.IndentedJSON(http.StatusOK, envelope)
}

func (h *handler) GetSupplier(c *gin.Context) {

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userSession := GetSessionInfo(c)

	companyID, err := h.queries.GetCompanyIdByUser(c.Request.Context(), int32(userSession.id))

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	} else if companyID == nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	supplier, err := h.queries.GetSupplier(c.Request.Context(), id)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	c.IndentedJSON(http.StatusOK, supplier)
}

func (h *handler) AddSupplier(c *gin.Context) {

	userSession := GetSessionInfo(c)
	companyID := h.getUserCompany(c)

	var id int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&id); err != nil {
		return
	}

	var request SupplierRequest
	if err := c.BindJSON(&request); err != nil {
		return
	}

	if request.CreditLimit == nil {
		request.CreditLimit = &decimal.Zero
	}

	args :=
		db.AddSupplierParams{
			CompanyID:              int32(companyID),
			Name:                   request.Name,
			Address:                request.Address,
			PhoneNumber:            request.PhoneNumber,
			Number:                 request.Number,
			VatNumber:              request.VatNumber,
			BankAccount:            request.BankAccount,
			IsPostpaid:             request.IsPostPaid,
			CreditLimit:            *request.CreditLimit,
			PaymentTermsDays:       request.PaymentTermsDays,
			CommercialRegistration: request.CommercialRegistration,
			PreferredPaymentMethod: request.PreferredPaymentMethod,
			ShortAddress:           request.ShortAddress,
			Email:                  request.Email,
		}

	res, err := h.queries.AddSupplier(c.Request.Context(), args)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	supplierID, err := res.LastInsertId()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": supplierID})

}

func (h *handler) EditSupplier(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var companyId int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&companyId); err != nil {
		return
	}

	var request SupplierRequest
	if err := c.BindJSON(&request); err != nil {
		return
	}

	res, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if request.CreditLimit != nil {
		request.CreditLimit = &decimal.Zero
	}

	row := db.UpdateSupplierParams{
		Name:                   request.Name,
		Address:                request.Address,
		PhoneNumber:            request.PhoneNumber,
		Number:                 request.Number,
		VatNumber:              request.VatNumber,
		BankAccount:            request.BankAccount,
		IsPostpaid:             request.IsPostPaid,
		CreditLimit:            *request.CreditLimit,
		PaymentTermsDays:       request.PaymentTermsDays,
		CommercialRegistration: request.CommercialRegistration,
		PreferredPaymentMethod: request.PreferredPaymentMethod,
		ID:                     res,
		ShortAddress:           request.ShortAddress,
		Email:                  request.Email,
	}

	if err := h.queries.UpdateSupplier(c.Request.Context(), row); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)
}

func (h *handler) DeleteSupplier(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var companyId int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&companyId); err != nil {
		return
	}

	id := c.Param("id")

	if _, err := h.DB.Exec(
		"UPDATE supplier SET is_deleted=TRUE where company_id=? and id=?;", companyId, id); err != nil {
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *handler) SearchSuppliers(c *gin.Context) {
	userSession := GetSessionInfo(c)

	companyID, err := h.queries.GetCompanyIdByUser(c.Request.Context(), int32(userSession.id))
	if err != nil || companyID == nil {
		log.Printf("SearchSuppliers: %v", err)
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	q := c.Query("q")
	queryLike, _ := buildLikeAndDigitsExact(&q)

	const maxResults = 100
	suppliers, err := h.queries.SearchSuppliers(c.Request.Context(), db.SearchSuppliersParams{
		CompanyID: *companyID,
		QueryLike: queryLike,
		Limit:     maxResults,
	})
	if err != nil {
		log.Printf("SearchSuppliers: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Map to response model
	results := make([]model.SearchSupplierResponse, len(suppliers))
	for i, s := range suppliers {
		name := ""
		code := ""
		if s.Name != nil {
			name = *s.Name
		}
		if s.Code != nil {
			code = *s.Code
		}
		results[i] = model.SearchSupplierResponse{
			ID:   s.ID,
			Name: name,
			Code: code,
		}
	}

	c.IndentedJSON(http.StatusOK, results)
}
