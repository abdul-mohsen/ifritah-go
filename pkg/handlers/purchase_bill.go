package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

func (h *handler) getPurchaseBills(c *gin.Context, args db.GetAllPurchaseBillParams) ([]db.PurchaseBill, error) {
	return h.queries.GetAllPurchaseBill(c.Request.Context(), args)
}

// purchaseBillSetup is the result of beginPurchaseBillTx — everything both
// Create and Update flows need before they shape their own DB params.
type purchaseBillSetup struct {
	request        model.AddPurchaseBillRequest
	session        userSession
	paymentDueDate *time.Time
	effectiveDate  time.Time
	tx             *sql.Tx
	qtx            *db.Queries
}

// beginPurchaseBillTx binds the JSON payload, validates store access, parses
// dates, and opens a transaction. On any failure it writes the appropriate
// 4xx/5xx response and returns ok=false; callers just `return`.
func (h *handler) beginPurchaseBillTx(c *gin.Context, defaultState int32) (*purchaseBillSetup, bool) {
	req := model.AddPurchaseBillRequest{
		State:         defaultState,
		PaymentMethod: 0,
		PaidAmount:    decimal.Zero,
	}
	if err := c.BindJSON(&req); err != nil {
		log.Printf("beginPurchaseBillTx: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": BindError(err)})
		return nil, false
	}
	if !slices.Contains(h.getStoreIds(c), req.StoreId) {
		c.Status(http.StatusBadRequest)
		return nil, false
	}
	eff := time.Now()
	if req.EffectiveDate != nil {
		eff = *req.EffectiveDate
	}
	tx, err := h.DB.Begin() //NOSONAR — caller defers tx.Rollback after consuming setup
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return nil, false
	}
	return &purchaseBillSetup{
		request:        req,
		session:        GetSessionInfo(c),
		paymentDueDate: req.PaymentDueDate,
		effectiveDate:  eff,
		tx:             tx,
		qtx:            h.queries.WithTx(tx),
	}, true
}

func purchaseBillID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return 0, false
	}
	return uint64(id), true
}

func purchaseBillProducts(request *model.AddPurchaseBillRequest) []model.PurchaseBillProduct {
	// The UI may send IDs for manually entered lines; they must never become
	// inventory-linked or stock-tracked.
	for i := range request.ManualProducts {
		request.ManualProducts[i].ProductId = nil
		request.ManualProducts[i].TrackStock = false
	}
	return append(request.Products, request.ManualProducts...)
}

func updatePurchaseBillError(c *gin.Context, err error) {
	if IsDuplicate(err) {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"message": "Supplier bill number already exists for this supplier",
		})
		return
	}
	c.AbortWithError(http.StatusBadRequest, err)
}

func addPurchaseBillError(c *gin.Context, err error) {
	log.Printf("AddPurchaseBill: %v", err)
	if IsDuplicate(err) {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"message": "Supplier bill number already exists for this supplier",
		})
		return
	}
	c.Status(http.StatusBadRequest)
}

func updatePurchaseBillParams(request model.AddPurchaseBillRequest, setup *purchaseBillSetup, id uint64) db.UpdatePurchaseBillParams {
	return db.UpdatePurchaseBillParams{
		EffectiveDate:          setup.effectiveDate,
		PaymentDueDate:         setup.paymentDueDate,
		State:                  request.State,
		Discount:               request.Discount,
		StoreID:                request.StoreId,
		MerchantID:             int32(setup.session.id),
		SupplierID:             request.SupplierId,
		SupplierSequenceNumber: &request.SupplierSequenceNumber,
		ID:                     id,
		PaymentMethod:          request.PaymentMethod,
		DeliverDate:            request.DeliverDate,
	}
}

func addPurchaseBillParams(request model.AddPurchaseBillRequest, setup *purchaseBillSetup) db.AddPurchaseBillParams {
	return db.AddPurchaseBillParams{
		EffectiveDate:          setup.effectiveDate,
		PaymentDueDate:         setup.paymentDueDate,
		State:                  request.State,
		Discount:               request.Discount,
		StoreID:                int32(request.StoreId),
		MerchantID:             int32(setup.session.id),
		SupplierID:             request.SupplierId,
		SupplierSequenceNumber: &request.SupplierSequenceNumber,
		PdfLink:                request.PDFLink,
		PaymentMethod:          request.PaymentMethod,
		DeliverDate:            request.DeliverDate,
	}
}

func (h *handler) savePurchaseBillAttachments(id uint64, request model.AddPurchaseBillRequest) {
	var attachments []string
	for _, attachment := range request.Attachments {
		attachments = append(attachments, attachment)
	}
	if request.PDFLink != nil {
		_ = h.SavePurchaseBillAttachments(h.DB, id, *request.PDFLink, attachments)
	}
}

func recordPurchaseBillStockMovements(qtx *db.Queries, c *gin.Context, id uint64,
	request model.AddPurchaseBillRequest, enforcement string, userID int32) error {
	if enforcement == model.StockEnforcementDisable || request.State <= 0 {
		return nil
	}
	return recordPurchaseMovements(
		qtx, c, id, request.StoreId,
		request.SupplierSequenceNumber,
		enforcement, userID,
	)
}

func finalizePurchaseBill(c *gin.Context, setup *purchaseBillSetup, id uint64,
	request model.AddPurchaseBillRequest, enforcement, operation string) bool {
	if err := addProductToBillPurchase(setup.qtx, c, purchaseBillProducts(&request), id, request.StoreId,
		enforcement != model.StockEnforcementDisable && request.State > 0); err != nil {
		log.Printf("%s: %v", operation, err)
		c.Status(http.StatusBadRequest)
		return false
	}
	if err := setup.qtx.RefreshPurchaseBillTotals(c.Request.Context(), id); err != nil {
		log.Printf("%s refresh totals: %v", operation, err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return false
	}
	if err := recordPurchaseBillStockMovements(setup.qtx, c, id, request, enforcement, int32(setup.session.id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error(), "type": "stock_error"})
		return false
	}
	if err := setup.tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return false
	}
	return true
}

func (h *handler) UpdatePurchaseBill(c *gin.Context) {
	id, valid := purchaseBillID(c)
	if !valid {
		return
	}
	setup, ok := h.beginPurchaseBillTx(c, 3)
	if !ok {
		return
	}
	request := setup.request
	defer setup.tx.Rollback()
	enforcement := h.getStockEnforcementMode(c)

	if err := setup.qtx.UpdatePurchaseBill(c.Request.Context(), updatePurchaseBillParams(request, setup, id)); err != nil {
		updatePurchaseBillError(c, err)
		return
	}
	if enforcement != model.StockEnforcementDisable {
		if err := h.reversePurchaseMovements(setup.qtx, c, id, int32(setup.session.id)); err != nil {
			log.Printf("UpdatePurchaseBill: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error(), "type": "stock_error"})
			return
		}
	}
	if err := setup.qtx.DeleteProductPurchaseBill(c.Request.Context(), id); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if !finalizePurchaseBill(c, setup, id, request, enforcement, "UpdatePurchaseBill") {
		return
	}
	c.Status(http.StatusOK)
}

func (h *handler) AddPurchaseBill(c *gin.Context) {
	setup, ok := h.beginPurchaseBillTx(c, 1)
	if !ok {
		return
	}
	request := setup.request
	defer setup.tx.Rollback()
	enforcement := h.getStockEnforcementMode(c)

	res, err := setup.qtx.AddPurchaseBill(c.Request.Context(), addPurchaseBillParams(request, setup))
	if err != nil {
		addPurchaseBillError(c, err)
		return
	}
	insertedID, err := res.LastInsertId()
	if err != nil {
		log.Printf("AddPurchaseBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}
	id := uint64(insertedID)
	if !finalizePurchaseBill(c, setup, id, request, enforcement, "AddPurchaseBill") {
		return
	}
	h.savePurchaseBillAttachments(id, request)
	c.Status(http.StatusCreated)
}

func (h *handler) CheckPurchaseBillDuplicate(c *gin.Context) {
	request := model.PurchaseBillDuplicateCheckRequest{}
	if err := c.BindJSON(&request); err != nil {
		log.Printf("CheckPurchaseBillDuplicate: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": BindError(err)})
		return
	}

	userSession := GetSessionInfo(c)
	purchaseBillID, err := h.queries.GetPurchaseBillDuplicateBySupplierSequence(c.Request.Context(), db.GetPurchaseBillDuplicateBySupplierSequenceParams{
		UserID:                 int32(userSession.id),
		SupplierID:             request.SupplierId,
		SupplierSequenceNumber: &request.SupplierSequenceNumber,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, model.PurchaseBillDuplicateCheckResponse{Exists: false})
			return
		}

		log.Printf("CheckPurchaseBillDuplicate: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, model.PurchaseBillDuplicateCheckResponse{
		Exists:         true,
		PurchaseBillId: &purchaseBillID,
	})
}

func addProductToBillPurchase(tx *db.Queries, c *gin.Context, products []model.PurchaseBillProduct,
	billID uint64, storeID int32, recordStockMovement bool) error {

	for _, product := range products {
		inventoryProductID, err := purchaseInventoryProduct(tx, c, product, storeID, recordStockMovement)
		if err != nil {
			return err
		}
		if err := tx.AddProductToBillPurchase(c.Request.Context(), db.AddProductToBillPurchaseParams{
			ProductID:   inventoryProductID,
			Name:        &product.Name,
			Price:       product.Price,
			CostPrice:   product.CostPrice,
			ShelfNumber: product.ShelfNumber,
			Quantity:    product.Quantity,
			BillID:      billID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func purchaseInventoryProduct(tx *db.Queries, c *gin.Context, product model.PurchaseBillProduct,
	storeID int32, recordStockMovement bool) (*uint64, error) {
	if !product.TrackStock {
		return nil, nil
	}
	if product.ProductId != nil && *product.ProductId > 0 {
		return updatePurchaseInventoryProduct(tx, c, product, storeID, recordStockMovement)
	}
	return createPurchaseInventoryProduct(tx, c, product, storeID, recordStockMovement)
}

func updatePurchaseInventoryProduct(tx *db.Queries, c *gin.Context, product model.PurchaseBillProduct,
	storeID int32, recordStockMovement bool) (*uint64, error) {
	existing, err := tx.GetProduct(c.Request.Context(), uint64(*product.ProductId))
	if err != nil {
		return nil, fmt.Errorf("store product %d not found: %w", *product.ProductId, err)
	}
	if existing.StoreID != storeID {
		return nil, fmt.Errorf("store product %d does not belong to the selected store", *product.ProductId)
	}
	quantity := existing.Quantity
	if !recordStockMovement {
		quantity = quantity.Add(product.Quantity)
	}
	shelfNumber := product.ShelfNumber
	if shelfNumber == nil {
		shelfNumber = existing.ShelfNumber
	}
	if err := tx.UpdateProduct(c.Request.Context(), db.UpdateProductParams{
		ID:          existing.ID,
		Quantity:    quantity,
		Price:       existing.Price,
		CostPrice:   product.CostPrice,
		ShelfNumber: shelfNumber,
	}); err != nil {
		return nil, fmt.Errorf("update store product %d: %w", existing.ID, err)
	}
	productID := existing.ID
	return &productID, nil
}

func createPurchaseInventoryProduct(tx *db.Queries, c *gin.Context, product model.PurchaseBillProduct,
	storeID int32, recordStockMovement bool) (*uint64, error) {
	quantity := decimal.Zero
	if !recordStockMovement {
		quantity = product.Quantity
	}
	result, err := tx.AddProduct(c.Request.Context(), db.AddProductParams{
		ArticleID:   nil,
		Quantity:    quantity,
		Price:       product.Price,
		CostPrice:   product.CostPrice,
		ShelfNumber: product.ShelfNumber,
		StoreID:     storeID,
		Name:        &product.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("create store product %q: %w", product.Name, err)
	}
	productID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	id := uint64(productID)
	return &id, nil
}

const purchaseBillListSort = "-effective_date"

// purchaseBillSortCmp removed: sort over the current page is now
// FE-driven (BE returns rows in canonical keyset order only).

func (h *handler) GetAllPurchaseBill(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var storeIds []int
	for _, value := range h.getStores(userSession) {
		storeIds = append(storeIds, value.Id)
	}

	request := model.BillRequestFilter{}

	if err := c.BindJSON(&request); err != nil {
		log.Printf("GetAllPurchaseBill: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": BindError(err)})
		return
	}

	listReq := pagination.ListRequest{
		Limit:      request.Limit,
		Cursor:     request.Cursor,
		Sort:       request.Sort,
		Dir:        request.Dir,
		Query:      request.Query,
		PageNumber: request.Page,
		PageSize:   request.PageSize,
	}
	if err := listReq.Validate(purchaseBillListSort); err != nil {
		log.Printf("GetAllPurchaseBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	if len(request.StoreIds) == 0 {
		request.StoreIds = storeIds
	}
	if len(request.StoreIds) == 0 {
		c.JSON(http.StatusOK, pagination.Envelope[db.PurchaseBill]{Items: []db.PurchaseBill{}})
		return
	}
	if !allStoresAllowed(request.StoreIds, storeIds) {
		c.Status(http.StatusBadRequest)
		return
	}

	cur, _ := listReq.DecodedCursor()
	cursorDate, cursorID, ok := cursorDateAndID(cur)
	if !ok {
		log.Printf("GetAllPurchaseBill: malformed cursor")
		c.Status(http.StatusBadRequest)
		return
	}

	limit := listReq.EffectiveLimit()

	queryLike, querySeqExact := buildLikeAndDigitsExact(request.Query)
	stateFilter := nonNegativeStateFilter(request.State)

	seqPrefix := buildPlainPrefixFilter(request.SequenceNumber)
	supplierSeqPrefix := buildPlainPrefixFilter(request.SupplierSequenceNumber)
	phonePrefix := buildPlainPrefixFilter(request.Phone)

	bill, err := h.getPurchaseBills(c, db.GetAllPurchaseBillParams{
		ID:                int32(userSession.id),
		StateFilter:       stateFilter,
		QueryLike:         queryLike,
		QuerySeqExact:     querySeqExact,
		SeqPrefix:         seqPrefix,
		SupplierSeqPrefix: supplierSeqPrefix,
		PhonePrefix:       phonePrefix,
		CursorDate:        cursorDate,
		CursorID:          cursorID,
		Limit:             int32(limit + 1),
	})
	if err != nil {
		log.Printf("GetAllPurchaseBill: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	envelope := pagination.BuildEnvelope(
		bill,
		limit,
		purchaseBillListSort,
		func(b db.PurchaseBill) []any {
			return []any{b.EffectiveDate.UTC().Format(time.RFC3339Nano), b.ID}
		},
	)
	c.JSON(http.StatusOK, envelope)
}

func (h *handler) GetPurchaseBillDetail(c *gin.Context) {

	userSession := GetSessionInfo(c)

	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	id := uint64(Id)

	args := db.GetPurchaseBillDetailParams{
		ID:   int32(userSession.id),
		ID_2: id,
	}

	b, err := h.queries.GetPurchaseBillDetail(c.Request.Context(), args)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	xProducts, err := h.queries.GetPurchaseBillProducts(c.Request.Context(), id)

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	products := make(map[int8][]db.PurchaseBillProduct)
	for _, p := range xProducts {
		products[p.Type] = append(products[p.Type], p)
	}

	a, err := h.queries.GetPurchaseBillAttachments(c.Request.Context(), id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	var attachments []string
	for _, x := range a {
		attachments = append(attachments, "/api/v2/files/"+x.FileKey)
	}

	if b.PdfLink != nil {
		*b.PdfLink = "/api/v2/files/" + *b.PdfLink
	}

	bill := model.PurchaseBill{
		Id:                     b.ID,
		SupplierId:             b.SupplierID,
		SupplierSequenceNumber: b.SupplierSequenceNumber,
		EffectiveDate:          b.EffectiveDate,
		PaymentDueDate:         b.PaymentDueDate,
		State:                  3,
		StoreId:                b.StoreID,
		MerchantId:             int(b.MerchantID),
		Products:               products[0],
		ManualProducts:         products[1],
		Discount:               b.Discount.Round(2).String(),
		TotalBeforeVAT:         b.TotalBeforeVat.Round(2).String(),
		TotalVAT:               b.TotalVat.Round(2).String(),
		Total:                  b.Total.Round(2).String(),
		PDFLink:                b.PdfLink,
		Attachments:            attachments,
	}
	c.JSON(http.StatusOK, bill)
}

func (h *handler) DeletePurchaseBillDetail(c *gin.Context) {

	idStr := c.Param("id")
	PbID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	pbID := uint64(PbID)

	userSession := GetSessionInfo(c)

	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	// ── Stock tracking: reverse stock BEFORE deleting (need PB data intact) ──
	if err := h.reversePurchaseMovements(qtx, c, pbID, int32(userSession.id)); err != nil {
		log.Printf("DeletePurchaseBillDetail: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"detail": err.Error(),
			"type":   "stock_error",
		})
		return
	}

	// Soft-delete the purchase bill
	res, err := qtx.SoftDeletePurchaseBill(c.Request.Context(), pbID)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	affectedRows, err := res.RowsAffected()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if affectedRows == 0 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)
}

func BindError(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		first := ve[0]
		return fmt.Sprintf("%s, is %s", first.Field(), first.Tag())
	}

	msg := err.Error()
	if idx := strings.IndexByte(msg, '\n'); idx != -1 {
		msg = msg[:idx]
	}
	return msg
}
