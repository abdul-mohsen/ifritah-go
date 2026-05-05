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
	"ifritah/web-service-gin/pkg/pagination"

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
	ShelfNumber string          `json:"shelf_number"`
}
type AddProduct struct {
	Id          int32           `json:"product_id" binding:"required"`
	Quantity    decimal.Decimal `json:"quantity" binding:"required"`
	Price       decimal.Decimal `json:"price" binding:"required"`
	CostPrice   decimal.Decimal `json:"cost_price" binding:"required"`
	ShelfNumber string          `json:"shelf_number"`
}

func (h *handler) AddQuantity(c *gin.Context) {

	var request AddQuantityRequest
	if err := c.BindJSON(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	storeIds := h.getStoreIds(c)

	if len(request.Products) == 0 {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("ERR: missing required value"))
		return
	}

	if !slices.Contains(storeIds, request.StoreId) {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("ERR: store id does not match"))
		return
	}

	for _, value := range request.Products {

		args := db.AddProductParams{
			ArticleID:   &value.Id,
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
			return
		}
	}

	c.Status(http.StatusOK)

}

func (h *handler) UpdateProduct(c *gin.Context) {

	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	id := uint64(Id)

	var request UpdateProductRequest
	if err := c.BindJSON(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	args := db.UpdateProductParams{
		Quantity:    request.Quantity,
		Price:       request.Price,
		CostPrice:   request.CostPrice,
		ShelfNumber: &request.ShelfNumber,
		ID:          id,
	}
	if err := h.queries.UpdateProduct(c.Request.Context(), args); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.Status(http.StatusOK)

}

const productListSort = "-id"

// errGetAllProducts is the log-prefix format used by every error path
// in GetAllProducts (Sonar S1192).
const errGetAllProducts = "GetAllProducts: %v"

// applyProductStockFilter is the in-memory implementation of the
// FE round-2 P0 #2 "stock" filter. It runs against the bounded
// page slice because adding a new sqlc narg would explode the
// generated query branches.
//
//	"in"  → quantity > 0
//	"out" → quantity == 0
//	"low" → 0 < quantity <= min_stock
//	""    → no filter (returns input unchanged)
//	any other value → no-op (degrades to "no filter")
func applyProductStockFilter(products []db.Product, stock string) []db.Product {
	if stock == "" {
		return products
	}
	out := make([]db.Product, 0, len(products))
	for _, p := range products {
		if productMatchesStock(p, stock) {
			out = append(out, p)
		}
	}
	return out
}

func productMatchesStock(p db.Product, stock string) bool {
	switch stock {
	case "in":
		return p.Quantity.GreaterThan(decimal.Zero)
	case "out":
		return p.Quantity.IsZero()
	case "low":
		lowThr := decimal.NewFromInt(int64(p.MinStock))
		return p.Quantity.GreaterThan(decimal.Zero) && p.Quantity.LessThanOrEqual(lowThr)
	default:
		// Unknown value — treat as "no filter" to keep the response
		// non-empty rather than silently discarding the page.
		return true
	}
}

func (h *handler) GetAllProducts(c *gin.Context) {
	user := GetSessionInfo(c)

	request := model.PaginationRequest{}

	if err := c.BindJSON(&request); err != nil {
		log.Printf(errGetAllProducts, err)
		c.Status(http.StatusBadRequest)
		return
	}

	listReq := listRequestFromPagination(request)
	if err := listReq.Validate(productListSort); err != nil {
		log.Printf(errGetAllProducts, err)
		c.Status(http.StatusBadRequest)
		return
	}

	cur, _ := listReq.DecodedCursor()
	cursorID := cursorIDOnly(cur)
	limit := listReq.EffectiveLimit()

	// Search sentinels (FE §1): %term% LIKE wrapper plus an exact-PK
	// match when the query is all digits, so users can paste a
	// sticker id and get a direct hit.
	queryLike, queryIDMatch := buildLikeAndDigitsExact(request.Query)

	args := db.GetAllProductParams{
		ID:           int32(user.id),
		QueryLike:    queryLike,
		QueryIDMatch: nullInt64FromUint64Ptr(queryIDMatch),
		CursorID:     nullInt64FromUint64Ptr(cursorID),
		Limit:        int32(limit + 1),
	}

	products, err := h.queries.GetAllProduct(c.Request.Context(), args)
	if err != nil {
		log.Printf(errGetAllProducts, err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	products = applyProductStockFilter(products, request.Stock)

	envelope := pagination.BuildEnvelope(
		products,
		limit,
		productListSort,
		func(p db.Product) []any { return []any{p.ID} },
	)
	c.JSON(http.StatusOK, envelope)
}

func (h *handler) DeleteProduct(c *gin.Context) {
	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	id := uint64(Id)

	err = h.queries.DeleteProduct(c.Request.Context(), id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)

}

func (h *handler) GetProduct(c *gin.Context) {
	// user := GetSessionInfo(c)
	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	id := uint64(Id)

	res, err := h.queries.GetProduct(c.Request.Context(), id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, res)

}

type SearchProductRequest struct {
	Query string `json:"query" binding:"required"`
	Limit int32  `json:"limit"`
}

func (h *handler) SearchProducts(c *gin.Context) {
	user := GetSessionInfo(c)

	var request SearchProductRequest
	if err := c.BindJSON(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if request.Limit <= 0 || request.Limit > 50 {
		request.Limit = 30
	}

	args := db.SearchProductParams{
		ID:       int32(user.id),
		CONCAT:   request.Query,
		CONCAT_2: request.Query,
		Limit:    request.Limit,
	}

	products, err := h.queries.SearchProduct(c.Request.Context(), args)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, products)
}
