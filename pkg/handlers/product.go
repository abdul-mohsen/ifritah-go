package handlers

import (
	"log"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

type AddQuentityRequest struct {
	StoreId  int       `json:"store_id"`
	Products []Product `json:"products"`
}

type Product struct {
	Id       int `json:"product_id"`
	Quentity int `json:"quentity"`
}

func (h *handler) AddQuentity(c *gin.Context) {

	userSession := GetSessionInfo(c)
	var storeIds []int
	for _, value := range h.getStores(userSession) {
		storeIds = append(storeIds, value.Id)
	}

	var request AddQuentityRequest
	if err := c.BindJSON(&request); err != nil {
		log.Fatal(err)
		c.Status(http.StatusBadRequest)
	}

	if !slices.Contains(storeIds, request.StoreId) {
		log.Fatal("store id does not match")
		c.Status(http.StatusBadRequest)
	}

	query := `
	update product
	set quantity = COALESCE(quantity, 0) + ?
	where id = ? and store_id = ?
	`

	for _, value := range request.Products {
		if _, err := h.DB.Exec(query, value.Quentity, value.Id, request.StoreId); err != nil {
			log.Panic(err)
		}
	}

	c.Status(http.StatusCreated)

}
