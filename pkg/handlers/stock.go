package handlers

// ============================================================================
// Stock System Handlers — New Endpoints
// ============================================================================
// Copy this file to: pkg/handlers/stock.go
//
// New endpoints:
//   POST /api/v2/stock/adjust         → StockAdjust
//   POST /api/v2/stock/check          → StockCheck
//   GET  /api/v2/stock/movements/:id  → GetStockMovements
//   GET  /api/v2/stock/enforcement    → GetStockEnforcement
//
// Internal helpers (called from bill/PB/credit handlers):
//   getStockEnforcementMode()  → reads setting from DB
//   recordSaleMovements()      → deducts stock on bill create
//   reverseSaleMovements()     → restores stock on bill delete
//   recordPurchaseMovements()  → adds stock on PB create
//   reversePurchaseMovements() → deducts stock on PB delete
//   recordCreditNoteMovements()→ restores stock on credit note create
// ============================================================================

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"ifritah/web-service-gin/pkg/model"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ── Helper: Get stock enforcement mode from settings ────────────────────────

func (h *handler) getStockEnforcementMode() string {
	var mode string
	err := h.DB.QueryRow(
		"SELECT COALESCE(value, 'disable') FROM settings WHERE setting_key = 'stock_enforcement'",
	).Scan(&mode)
	if err != nil {
		return model.StockEnforcementDisable
	}
	return mode
}

// ── Helper: Insert a stock movement row (within a transaction) ──────────────

func insertStockMovement(tx *sql.Tx, productID, storeID int32, quantity decimal.Decimal,
	movementType, referenceType string, referenceID, itemID *int32,
	reason, note *string, createdBy *int32, createdAt time.Time) error {

	_, err := tx.Exec(
		`INSERT INTO stock_movements
		 (product_id, store_id, quantity, movement_type, reference_type,
		  reference_id, item_id, reason, note, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		productID, storeID, quantity.String(), movementType, referenceType,
		referenceID, itemID, reason, note, createdBy, createdAt,
	)
	return err
}

// ── Helper: Update product quantity (within a transaction) ──────────────────

func updateProductQuantity(tx *sql.Tx, productID int32, delta decimal.Decimal) error {
	_, err := tx.Exec(
		"UPDATE product SET quantity = quantity + ? WHERE id = ?",
		delta.String(), productID,
	)
	return err
}

// ── Helper: Get product stock info ──────────────────────────────────────────

func (h *handler) getProductStock(productID int32) (storeID int32, quantity decimal.Decimal, err error) {
	var qtyStr string
	err = h.DB.QueryRow(
		"SELECT store_id, quantity FROM product WHERE id = ? AND is_deleted = 0",
		productID,
	).Scan(&storeID, &qtyStr)
	if err != nil {
		return 0, decimal.Zero, err
	}
	quantity, err = decimal.NewFromString(qtyStr)
	return storeID, quantity, err
}

// ============================================================================
// recordSaleMovements — Called from AddBill / SubmitDraftBill
// ============================================================================
// For each catalog product in the bill:
//   1. Check stock if enforcement is "enforce" → return error if insufficient
//   2. Deduct product.quantity
//   3. Insert stock_movement (sale, negative qty)
//
// Parameters:
//   tx          — the active transaction (same as bill creation)
//   billID      — the new bill's ID
//   storeID     — bill.store_id
//   products    — list of BillProduct (only catalog items: ProductId != nil)
//   seqNumber   — bill.sequence_number (for the note)
//   enforcement — current enforcement mode
//   userID      — user performing the action
//
// Returns:
//   warnings    — list of products with insufficient stock (for "warn" mode)
//   err         — non-nil if "enforce" mode blocks the operation

type StockWarning struct {
	ProductID int32           `json:"product_id"`
	Available decimal.Decimal `json:"available"`
	Requested decimal.Decimal `json:"requested"`
}

func recordSaleMovements(tx *sql.Tx, billID int32, storeID int32,
	products []model.BillProduct, seqNumber int32,
	enforcement string, userID int32) ([]StockWarning, error) {

	if enforcement == model.StockEnforcementDisable {
		return nil, nil
	}

	var warnings []StockWarning
	now := time.Now()

	for _, p := range products {
		if p.ProductId == nil {
			continue // manual item — skip
		}

		productID := *p.ProductId

		// Check current stock (lock row for update to prevent race conditions)
		var currentQty decimal.Decimal
		var qtyStr string
		err := tx.QueryRow(
			"SELECT quantity FROM product WHERE id = ? FOR UPDATE",
			productID,
		).Scan(&qtyStr)
		if err != nil {
			// Product not found — skip (manual product with fake ID)
			continue
		}
		currentQty, _ = decimal.NewFromString(qtyStr)

		// Check if sufficient
		if currentQty.LessThan(p.Quantity) {
			if enforcement == model.StockEnforcementEnforce {
				return nil, fmt.Errorf(
					"insufficient stock for product %d: available=%s, requested=%s",
					productID, currentQty.String(), p.Quantity.String(),
				)
			}
			// warn mode — collect warning but continue
			warnings = append(warnings, StockWarning{
				ProductID: productID,
				Available: currentQty,
				Requested: p.Quantity,
			})
		}

		// Deduct stock
		if err := updateProductQuantity(tx, productID, p.Quantity.Neg()); err != nil {
			return nil, fmt.Errorf("failed to deduct stock for product %d: %w", productID, err)
		}

		// Record movement
		// NOTE: item_id (bill_product.id) is not yet known at bill creation time
		// because bill_product rows are auto-incremented. We pass nil for item_id.
		// The movement can still be traced via reference_type='bill' + reference_id=billID + product_id.
		refType := model.ReferenceTypeBill
		note := fmt.Sprintf("فاتورة مبيعات #%d", seqNumber)
		uid := userID
		if err := insertStockMovement(tx, productID, storeID,
			p.Quantity.Neg(), model.MovementTypeSale, refType,
			&billID, nil, nil, &note, &uid, now); err != nil {
			return nil, fmt.Errorf("failed to record sale movement for product %d: %w", productID, err)
		}
	}

	return warnings, nil
}

// ============================================================================
// reverseSaleMovements — Called from DeleteBillDetail
// ============================================================================
// Reverses stock deductions when a bill is deleted (state = -1).
// For each catalog item on the bill:
//   1. Restore product.quantity
//   2. Insert stock_movement (deletion, positive qty)

func (h *handler) reverseSaleMovements(tx *sql.Tx, billID int32, userID int32) error {
	enforcement := h.getStockEnforcementMode()
	if enforcement == model.StockEnforcementDisable {
		return nil
	}

	rows, err := h.DB.Query(
		`SELECT bp.id, bp.product_id, bp.quantity, b.store_id, b.sequence_number
		 FROM bill_product bp
		 JOIN bill b ON bp.bill_id = b.id
		 WHERE bp.bill_id = ? AND bp.product_id IS NOT NULL`,
		billID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	now := time.Now()

	for rows.Next() {
		var itemID, productID, storeID, seqNum int32
		var qtyStr string
		if err := rows.Scan(&itemID, &productID, &qtyStr, &storeID, &seqNum); err != nil {
			return err
		}

		qty, _ := decimal.NewFromString(qtyStr)

		// Restore stock
		if err := updateProductQuantity(tx, productID, qty); err != nil {
			return fmt.Errorf("failed to restore stock for product %d: %w", productID, err)
		}

		// Record deletion movement (positive = stock returned)
		refType := model.ReferenceTypeBill
		note := fmt.Sprintf("إلغاء فاتورة #%d", seqNum)
		uid := userID
		if err := insertStockMovement(tx, productID, storeID,
			qty, model.MovementTypeDeletion, refType,
			&billID, &itemID, nil, &note, &uid, now); err != nil {
			return fmt.Errorf("failed to record deletion movement for product %d: %w", productID, err)
		}
	}

	return rows.Err()
}

// ============================================================================
// recordPurchaseMovements — Called from AddPurchaseBill / UpdatePurchaseBill
// ============================================================================
// For each catalog product in the purchase bill:
//   1. Add to product.quantity
//   2. Insert stock_movement (purchase, positive qty)

func recordPurchaseMovements(tx *sql.Tx, pbID int32, storeID int32,
	products []model.PurchaseBillProduct, seqNumber int32,
	enforcement string, userID int32) error {

	if enforcement == model.StockEnforcementDisable {
		return nil
	}

	now := time.Now()

	for _, p := range products {
		if p.ProductId == nil {
			continue // manual item — skip
		}

		productID := *p.ProductId

		// Add stock
		if err := updateProductQuantity(tx, productID, p.Quantity); err != nil {
			return fmt.Errorf("failed to add stock for product %d: %w", productID, err)
		}

		// Record movement
		// NOTE: item_id (purchase_bill_product.id) is not yet known at PB creation time.
		// We pass nil for item_id. The movement is traced via reference_type + reference_id + product_id.
		refType := model.ReferenceTypePurchaseBill
		note := fmt.Sprintf("فاتورة مشتريات #%d", seqNumber)
		uid := userID
		if err := insertStockMovement(tx, productID, storeID,
			p.Quantity, model.MovementTypePurchase, refType,
			&pbID, nil, nil, &note, &uid, now); err != nil {
			return fmt.Errorf("failed to record purchase movement for product %d: %w", productID, err)
		}
	}

	return nil
}

// ============================================================================
// reversePurchaseMovements — Called from DeletePurchaseBillDetail
// ============================================================================
// When a purchase bill is deleted, we need to reverse the stock additions.
// If enforcement is "enforce", we block if product.quantity would go negative.

func (h *handler) reversePurchaseMovements(tx *sql.Tx, pbID int32, userID int32) error {
	enforcement := h.getStockEnforcementMode()
	if enforcement == model.StockEnforcementDisable {
		return nil
	}

	rows, err := h.DB.Query(
		`SELECT pbp.id, pbp.product_id, pbp.quantity, pb.store_id, pb.sequence_number
		 FROM purchase_bill_product pbp
		 JOIN purchase_bill pb ON pbp.bill_id = pb.id
		 WHERE pbp.bill_id = ? AND pbp.product_id IS NOT NULL`,
		pbID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	now := time.Now()

	for rows.Next() {
		var itemID, productID, storeID, seqNum int32
		var qtyStr string
		if err := rows.Scan(&itemID, &productID, &qtyStr, &storeID, &seqNum); err != nil {
			return err
		}

		qty, _ := decimal.NewFromString(qtyStr)

		// In enforce mode, check we won't go negative
		if enforcement == model.StockEnforcementEnforce {
			var currentQtyStr string
			err := tx.QueryRow("SELECT quantity FROM product WHERE id = ? FOR UPDATE", productID).Scan(&currentQtyStr)
			if err != nil {
				continue
			}
			currentQty, _ := decimal.NewFromString(currentQtyStr)
			if currentQty.LessThan(qty) {
				return fmt.Errorf(
					"cannot delete purchase bill: product %d has %s in stock but bill has %s",
					productID, currentQty.String(), qty.String(),
				)
			}
		}

		// Deduct stock
		if err := updateProductQuantity(tx, productID, qty.Neg()); err != nil {
			return fmt.Errorf("failed to deduct stock for product %d: %w", productID, err)
		}

		// Record deletion movement (negative = stock removed)
		refType := model.ReferenceTypePurchaseBill
		note := fmt.Sprintf("إلغاء فاتورة مشتريات #%d", seqNum)
		uid := userID
		if err := insertStockMovement(tx, productID, storeID,
			qty.Neg(), model.MovementTypeDeletion, refType,
			&pbID, &itemID, nil, &note, &uid, now); err != nil {
			return fmt.Errorf("failed to record deletion movement for product %d: %w", productID, err)
		}
	}

	return rows.Err()
}

// ============================================================================
// recordCreditNoteMovements — Called from CreditBill
// ============================================================================
// When a credit note is created (returning entire bill):
//   1. For each catalog item on the original bill → restore stock
//   2. Insert stock_movement (credit_note, positive qty)

func (h *handler) recordCreditNoteMovements(tx *sql.Tx, creditNoteID int32, billID int32, userID int32) error {
	enforcement := h.getStockEnforcementMode()
	if enforcement == model.StockEnforcementDisable {
		return nil
	}

	// Get bill's store_id and sequence_number
	var storeID, seqNum int32
	err := h.DB.QueryRow(
		"SELECT store_id, sequence_number FROM bill WHERE id = ?", billID,
	).Scan(&storeID, &seqNum)
	if err != nil {
		return fmt.Errorf("failed to get bill %d info: %w", billID, err)
	}

	// Get all catalog products from the original bill
	rows, err := h.DB.Query(
		`SELECT bp.id, bp.product_id, bp.quantity
		 FROM bill_product bp
		 WHERE bp.bill_id = ? AND bp.product_id IS NOT NULL AND bp.quantity > 0`,
		billID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	now := time.Now()

	for rows.Next() {
		var itemID, productID int32
		var qtyStr string
		if err := rows.Scan(&itemID, &productID, &qtyStr); err != nil {
			return err
		}

		qty, _ := decimal.NewFromString(qtyStr)

		// Restore stock
		if err := updateProductQuantity(tx, productID, qty); err != nil {
			return fmt.Errorf("failed to restore stock for product %d: %w", productID, err)
		}

		// Record credit note movement (positive = stock returned)
		refType := model.ReferenceTypeCreditNote
		note := fmt.Sprintf("إشعار دائن #%d لفاتورة #%d", creditNoteID, seqNum)
		uid := userID
		if err := insertStockMovement(tx, productID, storeID,
			qty, model.MovementTypeCreditNote, refType,
			&creditNoteID, &itemID, nil, &note, &uid, now); err != nil {
			return fmt.Errorf("failed to record credit note movement for product %d: %w", productID, err)
		}
	}

	return rows.Err()
}

// ============================================================================
// API ENDPOINTS
// ============================================================================

// ── POST /api/v2/stock/adjust ───────────────────────────────────────────────
// Manual stock adjustment (damaged, lost, count_correction, etc.)

func (h *handler) StockAdjust(c *gin.Context) {
	var req model.StockAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request: " + err.Error()})
		return
	}

	enforcement := h.getStockEnforcementMode()
	if enforcement == model.StockEnforcementDisable {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "stock tracking is disabled"})
		return
	}

	userID := GetSessionInfo(c).id

	// Get product info
	storeID, currentQty, err := h.getProductStock(req.ProductID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "product not found"})
		return
	}

	// Check if adjustment would make stock negative (enforce mode)
	newQty := currentQty.Add(req.QuantityChange)
	if enforcement == model.StockEnforcementEnforce && newQty.IsNegative() {
		c.JSON(http.StatusBadRequest, gin.H{
			"detail":    "insufficient stock",
			"available": currentQty.String(),
			"requested": req.QuantityChange.Abs().String(),
		})
		return
	}

	// Transaction: update product + insert movement
	tx, err := h.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "transaction error"})
		log.Panic(err)
	}
	defer tx.Rollback()

	if err := updateProductQuantity(tx, req.ProductID, req.QuantityChange); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update stock"})
		log.Panic(err)
	}

	refType := model.ReferenceTypeManual
	uid := int32(userID)
	reason := req.Reason
	note := req.Note
	if err := insertStockMovement(tx, req.ProductID, storeID,
		req.QuantityChange, model.MovementTypeAdjustment, refType,
		nil, nil, &reason, &note, &uid, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to record movement"})
		log.Panic(err)
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "commit error"})
		log.Panic(err)
	}

	c.JSON(http.StatusOK, gin.H{
		"detail":       "stock adjusted",
		"product_id":   req.ProductID,
		"new_quantity": newQty.String(),
	})
}

// ── POST /api/v2/stock/check ────────────────────────────────────────────────
// Pre-bill stock availability check

func (h *handler) StockCheck(c *gin.Context) {
	var req model.StockCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request: " + err.Error()})
		return
	}

	enforcement := h.getStockEnforcementMode()

	response := model.StockCheckResponse{
		Enforcement:   enforcement,
		AllSufficient: true,
	}

	for _, item := range req.Items {
		storeID, available, err := h.getProductStock(item.ProductID)
		if err != nil {
			// Product not found — treat as 0 available
			response.Items = append(response.Items, model.StockCheckResult{
				ProductID:  item.ProductID,
				StoreID:    0,
				Available:  decimal.Zero,
				Requested:  item.Quantity,
				Sufficient: false,
			})
			response.AllSufficient = false
			continue
		}

		sufficient := available.GreaterThanOrEqual(item.Quantity)
		if !sufficient {
			response.AllSufficient = false
		}

		response.Items = append(response.Items, model.StockCheckResult{
			ProductID:  item.ProductID,
			StoreID:    storeID,
			Available:  available,
			Requested:  item.Quantity,
			Sufficient: sufficient,
		})
	}

	c.JSON(http.StatusOK, response)
}

// ── GET /api/v2/stock/movements/:product_id ─────────────────────────────────
// Paginated movement history for a product

func (h *handler) GetStockMovements(c *gin.Context) {
	productID, err := strconv.ParseInt(c.Param("product_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid product_id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	if page < 0 {
		page = 0
	}

	// Optional type filter
	typeFilter := c.Query("type")

	// Build query
	query := `SELECT id, product_id, store_id, quantity, movement_type,
	           reference_type, reference_id, item_id, reason, note,
	           created_by, created_at
	          FROM stock_movements
	          WHERE product_id = ?`
	args := []interface{}{int32(productID)}

	if typeFilter != "" {
		query += " AND movement_type = ?"
		args = append(args, typeFilter)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, page*pageSize)

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "query failed"})
		log.Panic(err)
	}
	defer rows.Close()

	var movements []model.StockMovementResponse
	for rows.Next() {
		var m model.StockMovementResponse
		var qty string
		var createdAt time.Time
		if err := rows.Scan(
			&m.ID, &m.ProductID, &m.StoreID, &qty, &m.MovementType,
			&m.ReferenceType, &m.ReferenceID, &m.ItemID, &m.Reason, &m.Note,
			&m.CreatedBy, &createdAt,
		); err != nil {
			continue
		}
		m.Quantity = qty
		m.CreatedAt = createdAt.Format(time.RFC3339)
		movements = append(movements, m)
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM stock_movements WHERE product_id = ?"
	countArgs := []interface{}{int32(productID)}
	if typeFilter != "" {
		countQuery += " AND movement_type = ?"
		countArgs = append(countArgs, typeFilter)
	}
	var total int
	h.DB.QueryRow(countQuery, countArgs...).Scan(&total)

	if movements == nil {
		movements = []model.StockMovementResponse{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      movements,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ── GET /api/v2/stock/enforcement ───────────────────────────────────────────
// Returns the current stock enforcement mode

func (h *handler) GetStockEnforcement(c *gin.Context) {
	mode := h.getStockEnforcementMode()
	c.JSON(http.StatusOK, model.StockEnforcementResponse{Mode: mode})
}
