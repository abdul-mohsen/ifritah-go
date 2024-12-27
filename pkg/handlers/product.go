package handlers

import (
	"log"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

type AddQuentityRequest struct {
	StoreId  int          `json:"store_id" binding:"required"`
	Products []AddProduct `json:"products" binding:"required,dive"`
}

type AddProduct struct {
	Id       int `json:"product_id" binding:"required"`
	Quantity int `json:"quantity" binding:"required"`
}

func (h *handler) AddQuentity(c *gin.Context) {

	var request AddQuentityRequest
	if err := c.BindJSON(&request); err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	storeIds := h.getStoreIds(c)

	if len(request.Products) == 0 {
		c.Status(http.StatusBadRequest)
		log.Panic("ERR: missing required value")
	}

	for _, value := range request.Products {
		if value.Quantity <= 0 {
			log.Panic("ERR: quantity can't be 0 or less")
			c.Status(http.StatusBadRequest)
		}
	}

	if !slices.Contains(storeIds, request.StoreId) {
		c.Status(http.StatusBadRequest)
		log.Panic("ERR: store id does not match")
	}

	query := `
	update product
	set quantity = COALESCE(quantity, 0) + ?
	where id = ? and store_id = ?
	`

	for _, value := range request.Products {
		if _, err := h.DB.Exec(query, value.Quantity, value.Id, request.StoreId); err != nil {
			log.Panic(err)
		}
	}

	c.Status(http.StatusOK)

}
