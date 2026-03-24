package handlers

// ============================================================================
// Order CRUD — Full CRUD with order items support
// ============================================================================
// Copy to: pkg/handlers/handler_order.go
//
// Endpoints:
//   POST   /api/v2/order/all          → GetOrders (list with pagination + search)
//   GET    /api/v2/order/:id          → GetOrder (single detail with items)
//   POST   /api/v2/order              → CreateOrder (new order + items)
//   PUT    /api/v2/order/:id          → UpdateOrder (modify order + items)
//   DELETE /api/v2/order/:id          → DeleteOrder (cascade deletes items)
//
// DATABASE TABLES:
//   - `orders` table:
//       id, sequence_number, client_id, customer_name, store_id,
//       status (ENUM: pending/processing/completed/cancelled),
//       total (DECIMAL 12,2), note, created_by, created_at, updated_at
//   - `order_items` table:
//       id, order_id, part_id, part_name, quantity, unit_price, line_total,
//       created_at
//   - FK: orders.client_id  → clients(id)
//   - FK: orders.store_id   → stores(id)
//   - FK: orders.created_by → users(id)
//   - FK: order_items.order_id → orders(id) ON DELETE CASCADE
//   - FK: order_items.part_id  → parts(id)  ON DELETE SET NULL
//
// SCHEMA:
//   client_id may be NULL (walk-in customer) — customer_name stores the name
//   sequence_number is UNIQUE (e.g. ORD-001)
//   total is auto-computed from SUM(order_items.line_total) on create/update
//
// Follows same patterns as handler_store.go:
//   - (h *handler) method receiver
//   - h.DB for database access
//   - GetSessionInfo(c) for current user session
// ============================================================================

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ── POST /api/v2/order/all ──────────────────────────────────────────────────
// Lists orders with optional pagination and search.
// Request body (JSON):
//   { "page_number": 1, "page_size": 20, "query": "ORD" }
// All fields optional. Defaults: page 1, size 20, no filter.

func (h *handler) GetOrders(c *gin.Context) {
	companyID := h.getUserCompany(c)

	var req struct {
		PageNumber int    `json:"page_number"`
		PageSize   int    `json:"page_size"`
		Query      string `json:"query"`
	}
	c.ShouldBindJSON(&req)
	if req.PageNumber < 1 {
		req.PageNumber = 1
	}
	if req.PageSize < 1 || req.PageSize > 10000 {
		req.PageSize = 20
	}

	// Build WHERE clause
	where := "WHERE o.store_id IN (SELECT id FROM store WHERE company_id = ?)"
	args := []interface{}{companyID}

	if req.Query != "" {
		where += " AND (o.sequence_number LIKE ? OR o.customer_name LIKE ?)"
		like := "%" + req.Query + "%"
		args = append(args, like, like)
	}

	// Count total
	var total int
	h.DB.QueryRow("SELECT COUNT(*) FROM orders o "+where, args...).Scan(&total)

	// Paginate
	offset := (req.PageNumber - 1) * req.PageSize
	args = append(args, req.PageSize, offset)

	rows, err := h.DB.Query(`
		SELECT o.id, o.sequence_number, o.client_id, COALESCE(o.customer_name,''),
		       o.store_id, o.status, o.total, COALESCE(o.note,''),
		       o.created_by, DATE_FORMAT(o.created_at, '%Y-%m-%dT%H:%i:%s') AS created_at,
		       DATE_FORMAT(o.updated_at, '%Y-%m-%dT%H:%i:%s') AS updated_at,
		       COALESCE(c.name, o.customer_name, '') AS client_name
		FROM orders o
		LEFT JOIN client c ON c.id = o.client_id
		`+where+`
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		log.Printf("ERROR GetOrders: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch orders"})
		return
	}
	defer rows.Close()

	type orderRow struct {
		ID             int     `json:"id"`
		SequenceNumber string  `json:"sequence_number"`
		ClientID       *int    `json:"client_id"`
		CustomerName   string  `json:"customer_name"`
		StoreID        *int    `json:"store_id"`
		Status         string  `json:"status"`
		Total          float64 `json:"total"`
		Note           string  `json:"note"`
		CreatedBy      int     `json:"created_by"`
		CreatedAt      string  `json:"created_at"`
		UpdatedAt      string  `json:"updated_at"`
		ClientName     string  `json:"client_name"`
	}

	var orders []orderRow
	for rows.Next() {
		var o orderRow
		if err := rows.Scan(&o.ID, &o.SequenceNumber, &o.ClientID, &o.CustomerName,
			&o.StoreID, &o.Status, &o.Total, &o.Note,
			&o.CreatedBy, &o.CreatedAt, &o.UpdatedAt, &o.ClientName); err != nil {
			log.Printf("ERROR GetOrders scan: %v", err)
			continue
		}
		orders = append(orders, o)
	}
	if orders == nil {
		orders = []orderRow{}
	}

	totalPages := total / req.PageSize
	if total%req.PageSize > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        orders,
		"total":       total,
		"page":        req.PageNumber,
		"page_size":   req.PageSize,
		"total_pages": totalPages,
	})
}

// ── GET /api/v2/order/:id ───────────────────────────────────────────────────
// Returns a single order with its line items.

func (h *handler) GetOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid order ID"})
		return
	}

	var o struct {
		ID             int     `json:"id"`
		SequenceNumber string  `json:"sequence_number"`
		ClientID       *int    `json:"client_id"`
		CustomerName   string  `json:"customer_name"`
		StoreID        *int    `json:"store_id"`
		Status         string  `json:"status"`
		Total          float64 `json:"total"`
		Note           string  `json:"note"`
		CreatedBy      int     `json:"created_by"`
		CreatedAt      string  `json:"created_at"`
		UpdatedAt      string  `json:"updated_at"`
		ClientName     string  `json:"client_name"`
		StoreName      string  `json:"store_name"`
	}

	err = h.DB.QueryRow(`
		SELECT o.id, o.sequence_number, o.client_id, COALESCE(o.customer_name,''),
		       o.store_id, o.status, o.total, COALESCE(o.note,''),
		       o.created_by,
		       DATE_FORMAT(o.created_at, '%Y-%m-%dT%H:%i:%s'),
		       DATE_FORMAT(o.updated_at, '%Y-%m-%dT%H:%i:%s'),
		       COALESCE(c.name, o.customer_name, '') AS client_name,
		       COALESCE(s.name, '') AS store_name
		FROM orders o
		LEFT JOIN client c ON c.id = o.client_id
		LEFT JOIN store s ON s.id = o.store_id
		WHERE o.id = ?
	`, id).Scan(&o.ID, &o.SequenceNumber, &o.ClientID, &o.CustomerName,
		&o.StoreID, &o.Status, &o.Total, &o.Note,
		&o.CreatedBy, &o.CreatedAt, &o.UpdatedAt,
		&o.ClientName, &o.StoreName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "order not found"})
		return
	}

	// Fetch order items
	type orderItem struct {
		ID        int     `json:"id"`
		PartID    *int    `json:"part_id"`
		PartName  string  `json:"part_name"`
		Quantity  int     `json:"quantity"`
		UnitPrice float64 `json:"unit_price"`
		LineTotal float64 `json:"line_total"`
	}

	var items []orderItem
	itemRows, err := h.DB.Query(`
		SELECT id, part_id, part_name, quantity, unit_price, line_total
		FROM order_items
		WHERE order_id = ?
		ORDER BY id ASC
	`, id)
	if err == nil {
		defer itemRows.Close()
		for itemRows.Next() {
			var it orderItem
			if itemRows.Scan(&it.ID, &it.PartID, &it.PartName, &it.Quantity,
				&it.UnitPrice, &it.LineTotal) == nil {
				items = append(items, it)
			}
		}
	}
	if items == nil {
		items = []orderItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"detail": gin.H{
			"id":              o.ID,
			"sequence_number": o.SequenceNumber,
			"client_id":       o.ClientID,
			"customer_name":   o.CustomerName,
			"store_id":        o.StoreID,
			"status":          o.Status,
			"total":           fmt.Sprintf("%.2f", o.Total),
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

// ── POST /api/v2/order ──────────────────────────────────────────────────────
// Creates a new order with optional line items.
// Request body (JSON):
//   {
//     "sequence_number": "ORD-006",
//     "client_id": 1,            // optional — NULL for walk-in
//     "customer_name": "اسم",     // required if no client_id
//     "store_id": 1,
//     "status": "pending",
//     "note": "ملاحظات",
//     "items": [
//       { "part_id": 10, "part_name": "فلتر زيت", "quantity": 2, "unit_price": 45.00 },
//       { "part_name": "شمعات", "quantity": 4, "unit_price": 25.00 }
//     ]
//   }

func (h *handler) CreateOrder(c *gin.Context) {
	session := GetSessionInfo(c)
	companyID := h.getUserCompany(c)

	type itemReq struct {
		PartID    *int    `json:"part_id"`
		PartName  string  `json:"part_name" binding:"required"`
		Quantity  int     `json:"quantity" binding:"required,gte=1"`
		UnitPrice float64 `json:"unit_price" binding:"gte=0"`
	}

	var req struct {
		SequenceNumber string    `json:"sequence_number" binding:"required"`
		ClientID       *int      `json:"client_id"`
		CustomerName   string    `json:"customer_name"`
		StoreID        *int      `json:"store_id"`
		Status         string    `json:"status"`
		Note           string    `json:"note"`
		Items          []itemReq `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "رقم الطلب مطلوب"})
		return
	}

	// Validate: must have client_id or customer_name
	if req.ClientID == nil && strings.TrimSpace(req.CustomerName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "يجب تحديد العميل أو إدخال اسم العميل"})
		return
	}

	// Validate client_id if provided
	if req.ClientID != nil {
		var exists int
		h.DB.QueryRow("SELECT COUNT(*) FROM client WHERE id = ? AND is_deleted = 0",
			*req.ClientID).Scan(&exists)
		if exists == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "العميل غير موجود"})
			return
		}
		// Fetch client name if customer_name not provided
		if req.CustomerName == "" {
			h.DB.QueryRow("SELECT COALESCE(name,'') FROM client WHERE id = ?",
				*req.ClientID).Scan(&req.CustomerName)
		}
	}

	// Validate store_id if provided
	if req.StoreID != nil {
		var exists int
		h.DB.QueryRow("SELECT COUNT(*) FROM store WHERE id = ? AND company_id = ?",
			*req.StoreID, companyID).Scan(&exists)
		if exists == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "المستودع غير موجود"})
			return
		}
	}

	// Default status
	if req.Status == "" {
		req.Status = "pending"
	}
	validStatuses := map[string]bool{"pending": true, "processing": true, "completed": true, "cancelled": true}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "حالة الطلب غير صالحة"})
		return
	}

	// Check unique sequence_number
	var dupCount int
	h.DB.QueryRow("SELECT COUNT(*) FROM orders WHERE sequence_number = ?",
		req.SequenceNumber).Scan(&dupCount)
	if dupCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "رقم الطلب مستخدم مسبقاً"})
		return
	}

	// Calculate total from items
	total := 0.0
	for i := range req.Items {
		lineTotal := float64(req.Items[i].Quantity) * req.Items[i].UnitPrice
		req.Items[i].UnitPrice = req.Items[i].UnitPrice // keep as-is
		total += lineTotal
	}

	// Start transaction
	tx, err := h.DB.Begin()
	if err != nil {
		log.Printf("ERROR CreateOrder begin tx: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
		return
	}
	defer tx.Rollback()

	// Insert order
	result, err := tx.Exec(`
		INSERT INTO orders (sequence_number, client_id, customer_name, store_id,
		                     status, total, note, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, req.SequenceNumber, req.ClientID, req.CustomerName, req.StoreID,
		req.Status, total, req.Note, session.id)
	if err != nil {
		log.Printf("ERROR CreateOrder insert: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل إنشاء الطلب"})
		return
	}

	orderID, _ := result.LastInsertId()

	// Insert order items
	if len(req.Items) > 0 {
		stmt, err := tx.Prepare(`
			INSERT INTO order_items (order_id, part_id, part_name, quantity, unit_price, line_total)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			log.Printf("ERROR CreateOrder prepare items: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
			return
		}
		defer stmt.Close()

		for _, item := range req.Items {
			lineTotal := float64(item.Quantity) * item.UnitPrice
			_, err := stmt.Exec(orderID, item.PartID, item.PartName,
				item.Quantity, item.UnitPrice, lineTotal)
			if err != nil {
				log.Printf("ERROR CreateOrder insert item: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل إضافة عنصر الطلب"})
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR CreateOrder commit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"detail": gin.H{
			"id":              orderID,
			"sequence_number": req.SequenceNumber,
			"total":           fmt.Sprintf("%.2f", total),
		},
	})
}

// ── PUT /api/v2/order/:id ───────────────────────────────────────────────────
// Updates an existing order and replaces its items.

func (h *handler) UpdateOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid order ID"})
		return
	}

	companyID := h.getUserCompany(c)

	type itemReq struct {
		PartID    *int    `json:"part_id"`
		PartName  string  `json:"part_name" binding:"required"`
		Quantity  int     `json:"quantity" binding:"required,gte=1"`
		UnitPrice float64 `json:"unit_price" binding:"gte=0"`
	}

	var req struct {
		SequenceNumber string    `json:"sequence_number" binding:"required"`
		ClientID       *int      `json:"client_id"`
		CustomerName   string    `json:"customer_name"`
		StoreID        *int      `json:"store_id"`
		Status         string    `json:"status"`
		Note           string    `json:"note"`
		Items          []itemReq `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	// Verify order exists and belongs to user's company
	var existingID int
	err = h.DB.QueryRow(`
		SELECT o.id FROM orders o
		WHERE o.id = ? AND o.store_id IN (SELECT id FROM store WHERE company_id = ?)
	`, id, companyID).Scan(&existingID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "الطلب غير موجود"})
		return
	}

	// Validate client_id
	if req.ClientID != nil {
		var exists int
		h.DB.QueryRow("SELECT COUNT(*) FROM client WHERE id = ? AND is_deleted = 0",
			*req.ClientID).Scan(&exists)
		if exists == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "العميل غير موجود"})
			return
		}
	}

	// Validate store_id
	if req.StoreID != nil {
		var exists int
		h.DB.QueryRow("SELECT COUNT(*) FROM store WHERE id = ? AND company_id = ?",
			*req.StoreID, companyID).Scan(&exists)
		if exists == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "المستودع غير موجود"})
			return
		}
	}

	validStatuses := map[string]bool{"pending": true, "processing": true, "completed": true, "cancelled": true}
	if req.Status != "" && !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "حالة الطلب غير صالحة"})
		return
	}

	// Check sequence_number uniqueness (excluding self)
	var dupCount int
	h.DB.QueryRow("SELECT COUNT(*) FROM orders WHERE sequence_number = ? AND id != ?",
		req.SequenceNumber, id).Scan(&dupCount)
	if dupCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "رقم الطلب مستخدم مسبقاً"})
		return
	}

	// Calculate total
	total := 0.0
	for _, item := range req.Items {
		total += float64(item.Quantity) * item.UnitPrice
	}

	// Transaction: update order + replace items
	tx, err := h.DB.Begin()
	if err != nil {
		log.Printf("ERROR UpdateOrder begin tx: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE orders SET sequence_number=?, client_id=?, customer_name=?,
		       store_id=?, status=?, total=?, note=?
		WHERE id=?
	`, req.SequenceNumber, req.ClientID, req.CustomerName,
		req.StoreID, req.Status, total, req.Note, id)
	if err != nil {
		log.Printf("ERROR UpdateOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل تحديث الطلب"})
		return
	}

	// Delete old items, insert new ones
	tx.Exec("DELETE FROM order_items WHERE order_id = ?", id)

	if len(req.Items) > 0 {
		stmt, err := tx.Prepare(`
			INSERT INTO order_items (order_id, part_id, part_name, quantity, unit_price, line_total)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			log.Printf("ERROR UpdateOrder prepare items: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
			return
		}
		defer stmt.Close()

		for _, item := range req.Items {
			lineTotal := float64(item.Quantity) * item.UnitPrice
			_, err := stmt.Exec(id, item.PartID, item.PartName,
				item.Quantity, item.UnitPrice, lineTotal)
			if err != nil {
				log.Printf("ERROR UpdateOrder insert item: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل تحديث عناصر الطلب"})
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR UpdateOrder commit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}

// ── DELETE /api/v2/order/:id ────────────────────────────────────────────────
// Deletes an order. order_items are cascade-deleted by FK.

func (h *handler) DeleteOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid order ID"})
		return
	}

	companyID := h.getUserCompany(c)

	// Verify ownership via store → company
	var existingID int
	err = h.DB.QueryRow(`
		SELECT o.id FROM orders o
		WHERE o.id = ? AND o.store_id IN (SELECT id FROM store WHERE company_id = ?)
	`, id, companyID).Scan(&existingID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"detail": "الطلب غير موجود"})
		return
	}

	// order_items cascade-deleted by FK constraint
	res, err := h.DB.Exec("DELETE FROM orders WHERE id = ?", id)
	if err != nil {
		log.Printf("ERROR DeleteOrder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل حذف الطلب"})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "الطلب غير موجود"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}
