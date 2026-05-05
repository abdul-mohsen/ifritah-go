package handlers

import (
	"context"
	"database/sql"
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const orderListSort = "-created_at"

var validOrderStatus = map[string]db.OrdersStatus{
	"pending":    db.OrdersStatusPending,
	"processing": db.OrdersStatusProcessing,
	"completed":  db.OrdersStatusCompleted,
	"cancelled":  db.OrdersStatusCancelled,
}

// GetOrders lists orders for the calling user's company (POST /api/v2/order/all).
func (h *handler) GetOrders(c *gin.Context) {
	companyID := h.getUserCompany(c)

	var req model.OrderListRequest
	c.ShouldBindJSON(&req)

	listReq := listRequestFromPagination(req.PaginationRequest)
	if err := listReq.Validate(orderListSort); err != nil {
		log.Printf("GetOrders: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	cur, _ := listReq.DecodedCursor()
	cursorCreatedAt, cursorID, ok := cursorDateAndID(cur)
	if !ok {
		log.Printf("GetOrders: malformed cursor")
		c.Status(http.StatusBadRequest)
		return
	}

	limit := listReq.EffectiveLimit()
	queryLike, _ := buildLikeAndDigitsExact(req.Query)

	rows, err := h.queries.GetOrders(c.Request.Context(), db.GetOrdersParams{
		CompanyID:       sql.NullInt64{Int64: int64(companyID), Valid: true},
		QueryLike:       queryLike,
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        nullInt64FromUint64Ptr(cursorID),
		Limit:           int32(limit + 1),
	})
	if err != nil {
		log.Printf("GetOrders: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch orders"})
		return
	}

	envelope := pagination.BuildEnvelope(
		rows,
		limit,
		orderListSort,
		func(o db.GetOrdersRow) []any {
			return []any{o.CreatedAt.UTC().Format(time.RFC3339Nano), int64(o.ID)}
		},
	)
	c.JSON(http.StatusOK, envelope)
}

// GetOrder returns a single order with its items (GET /api/v2/order/:id).
func (h *handler) GetOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid order ID"})
		return
	}

	ctx := c.Request.Context()
	o, err := h.queries.GetOrderByID(ctx, uint32(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "order not found"})
		return
	}

	items, err := h.queries.GetOrderItemsByOrderID(ctx, uint32(id))
	if err != nil || items == nil {
		items = []db.GetOrderItemsByOrderIDRow{}
	}

	c.JSON(http.StatusOK, gin.H{
		"detail": gin.H{
			"id":              o.ID,
			"sequence_number": o.SequenceNumber,
			"client_id":       o.ClientID,
			"customer_name":   o.CustomerName,
			"store_id":        o.StoreID,
			"status":          string(o.Status),
			"total":           o.Total.StringFixed(2),
			"note":            o.Note,
			"created_by":      o.CreatedBy,
			"created_at":      o.CreatedAt,
			"updated_at":      o.UpdatedAt,
			"client_name":     o.ClientName,
			"store_name":      o.StoreName,
			"items":           items,
		},
	})
}

// CreateOrder inserts a new order with optional line items (POST /api/v2/order).
func (h *handler) CreateOrder(c *gin.Context) {
	session := GetSessionInfo(c)
	companyID := h.getUserCompany(c)

	var req model.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "رقم الطلب مطلوب"})
		return
	}
	if req.ClientID == nil && strings.TrimSpace(req.CustomerName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "يجب تحديد العميل أو إدخال اسم العميل"})
		return
	}

	ctx := c.Request.Context()

	if req.ClientID != nil {
		count, _ := h.queries.CountClientForOrder(ctx, *req.ClientID)
		if count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "العميل غير موجود"})
			return
		}
		if req.CustomerName == "" {
			name, _ := h.queries.GetClientNameForOrder(ctx, *req.ClientID)
			req.CustomerName = name
		}
	}

	if req.StoreID != nil {
		count, _ := h.queries.CountStoreForOrder(ctx, db.CountStoreForOrderParams{
			ID: *req.StoreID, CompanyID: int32(companyID),
		})
		if count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "المستودع غير موجود"})
			return
		}
	}

	status, ok := resolveOrderStatus(req.Status)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "حالة الطلب غير صالحة"})
		return
	}

	dupCount, _ := h.queries.CountOrderBySequence(ctx, req.SequenceNumber)
	if dupCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "رقم الطلب مستخدم مسبقاً"})
		return
	}

	total := computeOrderTotal(req.Items)

	tx, err := h.DB.Begin()
	if err != nil {
		log.Printf("CreateOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	res, err := qtx.CreateOrder(ctx, db.CreateOrderParams{
		SequenceNumber: req.SequenceNumber,
		ClientID:       req.ClientID,
		CustomerName:   optionalString(req.CustomerName),
		StoreID:        req.StoreID,
		Status:         status,
		Total:          total,
		Note:           optionalString(req.Note),
		CreatedBy:      int32(session.id),
	})
	if err != nil {
		log.Printf("CreateOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل إنشاء الطلب"})
		return
	}
	orderID, _ := res.LastInsertId()

	if err := insertOrderItems(ctx, qtx, uint32(orderID), req.Items); err != nil {
		log.Printf("CreateOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل إضافة عنصر الطلب"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("CreateOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"detail": gin.H{
			"id":              orderID,
			"sequence_number": req.SequenceNumber,
			"total":           fmt.Sprintf("%s", total.StringFixed(2)),
		},
	})
}

// UpdateOrder modifies an order and replaces its items (PUT /api/v2/order/:id).
func (h *handler) UpdateOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid order ID"})
		return
	}
	companyID := h.getUserCompany(c)

	var req model.UpdateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	ctx := c.Request.Context()

	if _, err := h.queries.GetOrderForCompany(ctx, db.GetOrderForCompanyParams{
		ID: uint32(id), CompanyID: int32(companyID),
	}); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "الطلب غير موجود"})
		return
	}

	if req.ClientID != nil {
		count, _ := h.queries.CountClientForOrder(ctx, *req.ClientID)
		if count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "العميل غير موجود"})
			return
		}
	}

	if req.StoreID != nil {
		count, _ := h.queries.CountStoreForOrder(ctx, db.CountStoreForOrderParams{
			ID: *req.StoreID, CompanyID: int32(companyID),
		})
		if count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "المستودع غير موجود"})
			return
		}
	}

	status, ok := resolveOrderStatus(req.Status)
	if req.Status != "" && !ok {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "حالة الطلب غير صالحة"})
		return
	}

	dupCount, _ := h.queries.CountOrderBySequenceExcludingID(ctx, db.CountOrderBySequenceExcludingIDParams{
		SequenceNumber: req.SequenceNumber, ID: uint32(id),
	})
	if dupCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "رقم الطلب مستخدم مسبقاً"})
		return
	}

	total := computeOrderTotal(req.Items)

	tx, err := h.DB.Begin()
	if err != nil {
		log.Printf("UpdateOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	if err := qtx.UpdateOrder(ctx, db.UpdateOrderParams{
		SequenceNumber: req.SequenceNumber,
		ClientID:       req.ClientID,
		CustomerName:   optionalString(req.CustomerName),
		StoreID:        req.StoreID,
		Status:         status,
		Total:          total,
		Note:           optionalString(req.Note),
		ID:             uint32(id),
	}); err != nil {
		log.Printf("UpdateOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل تحديث الطلب"})
		return
	}

	if err := qtx.DeleteOrderItemsByOrderID(ctx, uint32(id)); err != nil {
		log.Printf("UpdateOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
		return
	}

	if err := insertOrderItems(ctx, qtx, uint32(id), req.Items); err != nil {
		log.Printf("UpdateOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل تحديث عناصر الطلب"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("UpdateOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

// DeleteOrder removes an order; order_items cascade via FK (DELETE /api/v2/order/:id).
func (h *handler) DeleteOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid order ID"})
		return
	}
	companyID := h.getUserCompany(c)
	ctx := c.Request.Context()

	if _, err := h.queries.GetOrderForCompany(ctx, db.GetOrderForCompanyParams{
		ID: uint32(id), CompanyID: int32(companyID),
	}); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "الطلب غير موجود"})
		return
	}

	res, err := h.queries.DeleteOrderByID(ctx, uint32(id))
	if err != nil {
		log.Printf("DeleteOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل حذف الطلب"})
		return
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "الطلب غير موجود"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

func resolveOrderStatus(s string) (db.OrdersStatus, bool) {
	if s == "" {
		return db.OrdersStatusPending, true
	}
	v, ok := validOrderStatus[s]
	return v, ok
}

func computeOrderTotal(items []model.OrderItemRequest) decimal.Decimal {
	total := decimal.Zero
	for _, it := range items {
		line := decimal.NewFromInt(int64(it.Quantity)).Mul(decimal.NewFromFloat(it.UnitPrice))
		total = total.Add(line)
	}
	return total
}

func insertOrderItems(ctx context.Context, qtx *db.Queries, orderID uint32, items []model.OrderItemRequest) error {
	for _, it := range items {
		qty := decimal.NewFromInt(int64(it.Quantity))
		unit := decimal.NewFromFloat(it.UnitPrice)
		if err := qtx.CreateOrderItem(ctx, db.CreateOrderItemParams{
			OrderID:   orderID,
			PartID:    it.PartID,
			PartName:  it.PartName,
			Quantity:  qty,
			UnitPrice: unit,
			LineTotal: qty.Mul(unit),
		}); err != nil {
			return err
		}
	}
	return nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
