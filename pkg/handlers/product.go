package handlers

import (
	"log"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

type AddQuentityRequest struct {
	StoreId  *int          `json:"store_id"`
	Products *[]AddProduct `json:"products" binding:"required"`
}

type AddProduct struct {
	Id       *int `json:"product_id"`
	Quantity *int `json:"quantity"`
}

func (h *handler) AddQuentity(c *gin.Context) {

	userSession := GetSessionInfo(c)
	var storeIds []int
	for _, value := range h.getStores(userSession) {
		storeIds = append(storeIds, value.Id)
	}

	var request AddQuentityRequest
	if err := c.BindJSON(&request); err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	if request.StoreId == nil || request.Products == nil || len(*request.Products) == 0 {
		c.Status(http.StatusBadRequest)
		log.Panic("ERR: missing required value")
	}

	for _, value := range *request.Products {
		if value.Quantity == nil || value.Id == nil {
			c.Status(http.StatusBadRequest)
			log.Panic("ERR: missing required value")
		}

		if *value.Quantity <= 0 {
			log.Panic("ERR: quantity can't be 0 or less")
			c.Status(http.StatusBadRequest)
		}
	}

	if !slices.Contains(storeIds, *request.StoreId) {
		c.Status(http.StatusBadRequest)
		log.Panic("ERR: store id does not match")
	}

	query := `
	update product
	set quantity = COALESCE(quantity, 0) + ?
	where id = ? and store_id = ?
	`

	for _, value := range *request.Products {
		if _, err := h.DB.Exec(query, value.Quantity, value.Id, request.StoreId); err != nil {
			log.Panic(err)
		}
	}

	c.Status(http.StatusOK)

}
