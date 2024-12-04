package handlers

import (
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PartsProviderRequest struct {
	Name        string `json:"name"`
	Address     string `json:"address"`
	PhoneNumber string `json:"phone_number"`
	Number      string `json:"number"`
	VatNumber   string `json:"vat_number"`
}

func (h *handler) GetAllSupplier(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var id int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&id); err != nil {
		log.Panic(err)

	}

	rows, err := h.DB.Query("SELECT * FROM parts_provider where company_id = ? and is_deleted = FALSE", id)

	if err != nil {
		log.Panic(err)
	}
	var partsProviders []model.PartsProvider
	for rows.Next() {
		var partsProvider model.PartsProvider

		if err := rows.Scan(&partsProvider.Id, &partsProvider.Copany_id, &partsProvider.Name, &partsProvider.Address, &partsProvider.PhoneNumber, &partsProvider.Number, &partsProvider.VatNumber, &partsProvider.IsDeleted); err != nil {
			log.Panic(err)
		}

		partsProviders = append(partsProviders, partsProvider)
	}
	defer rows.Close()
	c.IndentedJSON(http.StatusOK, partsProviders)
}

func (h *handler) AddSupplier(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var id int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&id); err != nil {
		log.Panic(err)
	}

	var request PartsProviderRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}

	if _, err := h.DB.Exec(
		"INSERT INTO parts_provider (company_id, name, address, phone_number, number, vat_number) VALUES (?, ?, ?, ?, ?, ?)", id, request.Name, request.Address, request.PhoneNumber, request.Number, request.VatNumber); err != nil {
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

	var request PartsProviderRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}

	var id string = c.Param("id")
	if _, err := h.DB.Exec(
		"UPDATE parts_provider SET name=?, address=?, phone_number=?, number=?, vat_number=? where company_id=? and id=?;", request.Name, request.Address, request.PhoneNumber, request.Number, request.VatNumber, companyId, id); err != nil {
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
		"DELETE FROM parts_provider where id = ? and company_id = ?", id, companyId); err != nil {
		log.Panic(err)
	}

	c.Status(http.StatusNoContent)
}
