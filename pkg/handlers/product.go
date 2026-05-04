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

func (h *handler) GetAllProducts(c *gin.Context) {
	user := GetSessionInfo(c)

	request := model.PaginationRequest{}

	if err := c.BindJSON(&request); err != nil {
		log.Printf("GetAllProducts: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	listReq := listRequestFromPagination(request)
	if err := listReq.Validate(productListSort); err != nil {
		log.Printf("GetAllProducts: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	cur, _ := listReq.DecodedCursor()
	cursorID := cursorIDOnly(cur)
	limit := listReq.EffectiveLimit()

	// Search sentinels (FE §1):
	//   - QueryLike: %term% used against name + shelf_number.
	//   - QueryIDMatch: when q is all-digits, the integer value for an
	//     exact PK match (so users can paste a sticker id and get a
	//     direct hit). NULL otherwise.
	var queryLike *string
	var queryIDMatch *uint64
	if request.Query != nil && *request.Query != "" {
		s := "%" + *request.Query + "%"
		queryLike = &s
		if id, err := strconv.ParseUint(*request.Query, 10, 64); err == nil && id > 0 {
			queryIDMatch = &id
		}
	}

	args := db.GetAllProductParams{
		ID:           int32(user.id),
		QueryLike:    queryLike,
		QueryIDMatch: queryIDMatch,
		CursorID:     cursorID,
		Limit:        int32(limit + 1),
	}

	products, err := h.queries.GetAllProduct(c.Request.Context(), args)
	if err != nil {
		log.Printf("GetAllProducts: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Stock filter (FE round-2 P0 #2). Applied in-memory because the
	// SQL is sqlc-generated and adding a new narg would explode the
	// query branches. With FE driving page_size up the bounded slice
	// here is what the user already sees on screen.
	//   "in"  → quantity > 0
	//   "out" → quantity == 0
	//   "low" → 0 < quantity <= min_stock
	if request.Stock != "" {
		filtered := make([]db.Product, 0, len(products))
		for _, p := range products {
			gtZero := p.Quantity.GreaterThan(decimal.Zero)
			eqZero := p.Quantity.IsZero()
			lowThr := decimal.NewFromInt(int64(p.MinStock))
			low := gtZero && p.Quantity.LessThanOrEqual(lowThr)
			switch request.Stock {
			case "in":
				if gtZero {
					filtered = append(filtered, p)
				}
			case "out":
				if eqZero {
					filtered = append(filtered, p)
				}
			case "low":
				if low {
					filtered = append(filtered, p)
				}
			default:
				filtered = append(filtered, p)
			}
		}
		products = filtered
	}

	// Server-driven table sort (FE round-2 §1). part_name isn't a
	// column on the catalog product table — it lives on bill_product
	// and order_items rows. Sorting by part_name therefore falls back
	// to product.name (the user-facing label on the catalog list).
	applyListSort(products, listReq.Sort, listReq.Dir, func(a, b db.Product, k string) (int, bool) {
		switch k {
		case "id":
			return uint64Cmp(a.ID, b.ID), true
		case "part_name", "name":
			return strPtrCmp(a.Name, b.Name), true
		case "price":
			return decCmp(a.Price, b.Price), true
		case "quantity":
			return decCmp(a.Quantity, b.Quantity), true
		case "status":
			return int32Cmp(a.Status, b.Status), true
		}
		return 0, false
	})

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
