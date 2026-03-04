package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"slices"
	"strconv"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

type AddQuantityRequest struct {
	StoreId  int32        `json:"store_id" binding:"required"`
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

	_, err := httputil.DumpRequest(c.Request, true)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}
	var request AddQuantityRequest
	if err := c.BindJSON(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

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
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				c.AbortWithError(http.StatusBadRequest, fmt.Errorf("Product already exists in this store"))
			}
			log.Panic(err)
		}
	}

	c.Status(http.StatusOK)

}

func (h *handler) GetAllProducts(c *gin.Context) {
	user := GetSessionInfo(c)

	request := model.PaginationRequest{
		Page:     0,
		PageSize: 10,
	}

	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}

	args := db.GetAllProductParams{
		ID:     int32(user.id),
		Limit:  request.PageSize,
		Offset: request.PageSize * request.Page,
	}

	products, err := h.queries.GetAllProduct(c.Request.Context(), args)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic("Error in query", err)
	}

	c.JSON(http.StatusOK, products)
}

func (h *handler) GetProduct(c *gin.Context) {
	// user := GetSessionInfo(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	res, err := h.queries.GetProduct(c.Request.Context(), int32(id))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic("Error in query", err)
	}

	c.JSON(http.StatusOK, res)

}

func (h *handler) DeleteProduct(c *gin.Context) {
	// user := GetSessionInfo(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	res, err := h.queries.GetProduct(c.Request.Context(), int32(id))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic("Error in query", err)
	}

	c.JSON(http.StatusOK, res)

}
