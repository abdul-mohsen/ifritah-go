package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
)

type AddQuantityRequest struct {
	StoreId  int32        `json:"store_id" binding:"required"`
	Products []AddProduct `json:"products" binding:"required,dive"`
}

type UpdateProductRequest struct {
	Quantity    decimal.Decimal `json:"quantity" binding:"required"`
	Price       decimal.Decimal `json:"price" binding:"required"`
	CostPrice   decimal.Decimal `json:"cost_price" binding:"required"`
	ShelfNumber string          `json:"shelf_number" binding:"required"`
}
type AddProduct struct {
	Id          int             `json:"product_id" binding:"required"`
	Quantity    decimal.Decimal `json:"quantity" binding:"required"`
	Price       decimal.Decimal `json:"price" binding:"required"`
	CostPrice   decimal.Decimal `json:"cost_price" binding:"required"`
	ShelfNumber string          `json:"shelf_number" binding:"required"`
}

func (h *handler) AddQuantity(c *gin.Context) {

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

	if !slices.Contains(storeIds, request.StoreId) {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("ERR: store id does not match"))
		log.Panic("ERR: store id does not match")
	}

	for _, value := range request.Products {

		args := db.AddProductParams{
			ArticleID:   int32(value.Id),
			Quantity:    value.Quantity,
			Price:       value.Price,
			CostPrice:   value.CostPrice,
			ShelfNumber: &value.ShelfNumber,
			StoreID:     request.StoreId,
		}
		if _, err := h.queries.AddProduct(c.Request.Context(), args); err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				c.AbortWithError(http.StatusBadRequest, fmt.Errorf("Product already exists in this store"))
			}
			log.Panic(err)
		}
	}

	c.Status(http.StatusOK)

}

func (h *handler) UpdateProduct(c *gin.Context) {

	id, err := strconv.ParseInt(c.Param("id"), 10, 32)

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}
	var request UpdateProductRequest
	if err := c.BindJSON(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	args := db.UpdateProductParams{
		Quantity:    request.Quantity,
		Price:       request.Price,
		CostPrice:   request.CostPrice,
		ShelfNumber: &request.ShelfNumber,
		ID:          int32(id),
	}
	if err := h.queries.UpdateProduct(c.Request.Context(), args); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
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
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	err = h.queries.DeleteProduct(c.Request.Context(), int32(id))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic("Error in query", err)
	}

	c.Status(http.StatusOK)

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
