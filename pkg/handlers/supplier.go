package handlers

import (
	"ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type SupplierRequest struct {
	Name                   *string `json:"name"`
	Address                *string `json:"address"`
	PhoneNumber            *string `json:"phone_number"`
	Number                 *string `json:"number"`
	VatNumber              *string `json:"vat_number"`
	BankAccount            *string `json:"bank_account"`
	IsPostPaid             bool    `json:"is_postpaid"`
	CreditLimit            string  `json:"credit_limit"`
	PaymentTermsDays       int32   `json:"payment_terms_days"`
	PreferredPaymentMethod int32   `json:"preferred_payment_method"`
	CommercialRegistration *string `json:"commercial_registration"`
	Email                  string  `json:"email"`
	CRN                    string  `json:"crn"`
	ShortAddress           *string `json:"short_address"`
}

func (h *handler) GetAllSupplier(c *gin.Context) {

	request := model.PaginationRequest{
		PageSize: 10,
		Page:     0,
	}

	if err := c.BindJSON(&request); err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}
	args := db.GetAllSupplierParams{
		Limit:  request.PageSize,
		Offset: request.Page * request.PageSize,
	}

	suppliers, err := h.queries.GetAllSupplier(c.Request.Context(), args)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	c.IndentedJSON(http.StatusOK, suppliers)
}

func (h *handler) GetSupplier(c *gin.Context) {

	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}
	userSession := GetSessionInfo(c)

	companyID, err := h.queries.GetCompanyIdByUser(c.Request.Context(), int32(userSession.id))

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	} else if companyID == nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	supplier, err := h.queries.GetSupplier(c.Request.Context(), db.GetSupplierParams{CompanyID: *companyID, ID: id})

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}
	c.IndentedJSON(http.StatusOK, supplier)
}

func (h *handler) AddSupplier(c *gin.Context) {

	userSession := GetSessionInfo(c)
	companyID := h.getUserCompany(c)

	var id int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&id); err != nil {
		log.Panic(err)
	}

	var request SupplierRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
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
			CreditLimit:            request.CreditLimit,
			PaymentTermsDays:       request.PaymentTermsDays,
			CommercialRegistration: request.CommercialRegistration,
			PreferredPaymentMethod: request.PreferredPaymentMethod,
			ShortAddress:           request.ShortAddress,
		}

	res, err := h.queries.AddSupplier(c.Request.Context(), args)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	supplierID, err := res.LastInsertId()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	c.JSON(http.StatusCreated, gin.H{"id": supplierID})

}

func (h *handler) EditSupplier(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var companyId int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&companyId); err != nil {
		log.Panic(err)
	}

	var request SupplierRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}

	res, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}
	row := db.UpdateSupplierParams{
		Name:                   request.Name,
		Address:                request.Address,
		PhoneNumber:            request.PhoneNumber,
		Number:                 request.Number,
		VatNumber:              request.VatNumber,
		BankAccount:            request.BankAccount,
		IsPostpaid:             request.IsPostPaid,
		CreditLimit:            request.CreditLimit,
		PaymentTermsDays:       request.PaymentTermsDays,
		CommercialRegistration: request.CommercialRegistration,
		PreferredPaymentMethod: request.PreferredPaymentMethod,
		ID:                     res,
		ShortAddress:           request.ShortAddress,
	}

	h.queries.UpdateSupplier(c.Request.Context(), row)

	c.Status(http.StatusOK)
}

func (h *handler) DeleteSupplier(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var companyId int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&companyId); err != nil {
		log.Panic(err)
	}

	id := c.Param("id")

	if _, err := h.DB.Exec(
		"UPDATE supplier SET is_deleted=TRUE where company_id=? and id=?;", companyId, id); err != nil {
		log.Panic(err)
	}

	c.Status(http.StatusNoContent)
}
