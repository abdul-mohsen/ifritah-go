package handlers

import (
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SupplierRequest struct {
	Name        string `json:"name"`
	Address     string `json:"address"`
	PhoneNumber string `json:"phone_number"`
	Number      string `json:"number"`
	VatNumber   string `json:"vat_number"`
	BankAccount string `json:"bank_account"`
}

func (h *handler) GetAllSupplier(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var id int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&id); err != nil {
		log.Panic(err)

	}

	rows, err := h.DB.Query("SELECT * where company_id = ? and is_deleted = FALSE", id)

	if err != nil {
		log.Panic(err)
	}
	var suppliers []model.Supplier
	for rows.Next() {
		var supplier model.Supplier

		if err := rows.Scan(&supplier.Id, &supplier.Copany_id, &supplier.Name, &supplier.Address, &supplier.PhoneNumber, &supplier.Number, &supplier.VatNumber, &supplier.IsDeleted, &supplier.BankAccount); err != nil {
			log.Panic(err)
		}

		suppliers = append(suppliers, supplier)
	}
	defer rows.Close()
	c.IndentedJSON(http.StatusOK, suppliers)
}

func (h *handler) AddSupplier(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var id int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&id); err != nil {
		log.Panic(err)
	}

	var request SupplierRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}

	if _, err := h.DB.Exec(
		"INSERT INTO supplier (company_id, name, address, phone_number, number, vat_number, bank_account) VALUES (?, ?, ?, ?, ?, ?, ?)", id, request.Name, request.Address, request.PhoneNumber, request.Number, request.VatNumber, request.BankAccount); err != nil {
		log.Panic(err)
	}

	c.Status(http.StatusCreated)

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

	var id string = c.Param("id")
	if _, err := h.DB.Exec(
		"UPDATE supplier SET name=?, address=?, phone_number=?, number=?, vat_number=? bank_account=? where company_id=? and id=?;", request.Name, request.Address, request.PhoneNumber, request.Number, request.VatNumber, request.BankAccount, companyId, id); err != nil {
		log.Panic(err)
	}

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
