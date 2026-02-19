package handlers

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"slices"

	"github.com/gin-gonic/gin"
)

type AddQuantityRequest struct {
	StoreId  int          `json:"store_id" binding:"required"`
	Products []AddProduct `json:"products" binding:"required,dive"`
}

type AddProduct struct {
	Id          int    `json:"product_id" binding:"required"`
	Quantity    int    `json:"quantity" binding:"required"`
	Price       int    `json:"price" binding:"required"`
	ShelfNumber string `json:"shelf_number" binding:"required"`
	CostPrice   int    `json:"cost_price" binding:"required"`
}

func (h *handler) AddQuantity(c *gin.Context) {

	raw, err := httputil.DumpRequest(c.Request, true)
	if err != nil {
		fmt.Println(raw)

	}
	var request AddQuantityRequest
	if err := c.BindJSON(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}
	log.Print(request)

	storeIds := h.getStoreIds(c)

	if len(request.Products) == 0 {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("ERR: missing required value"))
		log.Panic("ERR: missing required value")
	}

	for _, value := range request.Products {
		if value.Quantity <= 0 {
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("ERR: quantity can't be 0 or less"))
			log.Panic("ERR: quantity can't be 0 or less")
		}
	}

	if !slices.Contains(storeIds, request.StoreId) {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("ERR: store id does not match"))
		log.Panic("ERR: store id does not match")
	}

	query := `
	INSERT INTO product (article_id, quantity, price, cost_price ,shelf_number, store_id) VALUES (?,?,?,?,?,?)
	`

	for _, value := range request.Products {
		if _, err := h.DB.Exec(query, value.Id, value.Quantity, value.Price, value.CostPrice, value.ShelfNumber, request.StoreId); err != nil {
			log.Panic(err)
		}
	}

	c.Status(http.StatusOK)

}

func (h *handler) GetAllProducts(c *gin.Context) {
	user := GetSessionInfo(c)
	query := `
	select  p.article_id, p.price, p.quantity, p.cost_price, p.shelf_number
	from user
	join store s on s.company_id = user.company_id
	join product p on p.store_id = s.id
	where user.id = ?
	`

	rows, err := h.DB.Query(query, user.id)
	if err != nil {
		fmt.Println("Error in query", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var products []Product
	for rows.Next() {
		var product Product
		if rows.Scan(&product.Id, &product.Price, &product.Quantity, &product.CostPrice, &product.ShelfNumber); err != nil {
			fmt.Println("Error in query", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		products = append(products, product)
	}
	defer rows.Close()

	c.JSON(http.StatusOK, products)

}
