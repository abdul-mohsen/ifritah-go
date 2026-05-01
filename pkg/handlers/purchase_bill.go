package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
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

func (h *handler) getPurchaseBills(c *gin.Context, page int32, pageSize int32, userID int32) []db.PurchaseBill {

	args := db.GetAllPurchaseBillParams{
		ID:     userID,
		Limit:  pageSize,
		Offset: pageSize * page,
	}

	bills, err := h.queries.GetAllPurchaseBill(c.Request.Context(), args)

	if err != nil {
		log.Printf("getPurchaseBills: %v", err)
		return nil
	}

	return bills
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

func (h *handler) UpdatePurchaseBill(c *gin.Context) {

	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	id := uint64(Id)

	setup, ok := h.beginPurchaseBillTx(c, 3)
	if !ok {
		return
	}
	request, userSession := setup.request, setup.session
	paymentDueDate, effectiveDate := setup.paymentDueDate, setup.effectiveDate
	tx, qtx := setup.tx, setup.qtx

	defer tx.Rollback()

	args := db.UpdatePurchaseBillParams{
		EffectiveDate:          effectiveDate,
		PaymentDueDate:         paymentDueDate,
		State:                  request.State,
		Discount:               request.Discount,
		StoreID:                request.StoreId,
		MerchantID:             int32(userSession.id),
		SupplierID:             request.SupplierId,
		SupplierSequenceNumber: &request.SupplierSequenceNumber,
		ID:                     id,
		PaymentMethod:          request.PaymentMethod,
		DeliverDate:            request.DeliverDate,
	}

	err = qtx.UpdatePurchaseBill(c.Request.Context(), args)
	if err != nil {
		if IsDuplicate(err) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"message": "Supplier bill number already exists for this supplier",
			})
		} else {
			c.AbortWithError(http.StatusBadRequest, err)
		}

		return
	}

	// ── Stock tracking: reverse old stock before deleting products ──
	enforcement := h.getStockEnforcementMode(c)
	if enforcement != model.StockEnforcementDisable {
		if err := h.reversePurchaseMovements(qtx, c, id, int32(userSession.id)); err != nil {
			log.Printf("UpdatePurchaseBill: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"detail": err.Error(),
				"type":   "stock_error",
			})
			return
		}
	}

	if err = qtx.DeleteProductPurchaseBill(c.Request.Context(), id); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// TODO @ssda work around when the frontend send id when he should not
	for i := range request.ManualProducts {
		request.ManualProducts[i].ProductId = nil
	}

	products := append(request.Products, request.ManualProducts...)

	err = addProductToBillPurchase(qtx, c, products, id, request.StoreId)
	if err != nil {
		log.Printf("UpdatePurchaseBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	// ── Stock tracking: add new stock for updated products ──
	if enforcement != model.StockEnforcementDisable && request.State > 0 {
		if err := recordPurchaseMovements(
			qtx, c, id, request.StoreId,
			request.Products, request.SupplierSequenceNumber,
			enforcement, int32(userSession.id),
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"detail": err.Error(),
				"type":   "stock_error",
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)

}

func (h *handler) AddPurchaseBill(c *gin.Context) {

	setup, ok := h.beginPurchaseBillTx(c, 1)
	if !ok {
		return
	}
	request, userSession := setup.request, setup.session
	paymentDueDate, effectiveDate := setup.paymentDueDate, setup.effectiveDate
	tx, qtx := setup.tx, setup.qtx

	defer tx.Rollback()

	args := db.AddPurchaseBillParams{
		EffectiveDate:          effectiveDate,
		PaymentDueDate:         paymentDueDate,
		State:                  request.State,
		Discount:               request.Discount,
		StoreID:                int32(request.StoreId),
		MerchantID:             int32(userSession.id),
		SupplierID:             request.SupplierId,
		SupplierSequenceNumber: &request.SupplierSequenceNumber,
		PdfLink:                request.PDFLink,
		PaymentMethod:          request.PaymentMethod,
		DeliverDate:            request.DeliverDate,
	}

	res, err := qtx.AddPurchaseBill(c.Request.Context(), args)
	if err != nil {
		log.Printf("AddPurchaseBill: %v", err)
		if IsDuplicate(err) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"message": "Supplier bill number already exists for this supplier",
			})
		} else {
			c.Status(http.StatusBadRequest)
		}
		return
	}

	Id, err := res.LastInsertId()
	id := uint64(Id)

	if err != nil {
		log.Printf("AddPurchaseBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	// TODO @ssda work around when the frontend send id when he should not
	for i := range request.ManualProducts {
		request.ManualProducts[i].ProductId = nil
	}

	products := append(request.Products, request.ManualProducts...)

	err = addProductToBillPurchase(qtx, c, products, id, request.StoreId)
	if err != nil {
		log.Printf("AddPurchaseBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	// ── Stock tracking: add stock for catalog products ──
	enforcement := h.getStockEnforcementMode(c)
	if enforcement != model.StockEnforcementDisable && request.State > 0 {
		if err := recordPurchaseMovements(
			qtx, c, id, request.StoreId,
			request.Products, request.SupplierSequenceNumber,
			enforcement, int32(userSession.id),
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"detail": err.Error(),
				"type":   "stock_error",
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	var attachment []string
	for _, a := range request.Attachments {
		attachment = append(attachment, a)

	}
	if request.PDFLink != nil {
		if err := h.SavePurchaseBillAttachments(h.DB, id, *request.PDFLink, attachment); err != nil {
			// TODO @ssda review this to force pdf upload now it is blocked by the ui
			// c.AbortWithError(http.StatusInternalServerError, err)
			// log.Panic(err)
		}
	}

	c.Status(http.StatusCreated)

}

func addProductToBillPurchase(tx *db.Queries, c *gin.Context, products []model.PurchaseBillProduct, billId uint64, storeID int32) error {

	for _, product := range products {
		arg := db.AddProductParams{
			ArticleID:   product.ProductId,
			Quantity:    product.Quantity,
			Price:       product.Price,
			CostPrice:   product.CostPrice,
			ShelfNumber: product.ShelfNumber,
			StoreID:     storeID,
			Name:        &product.Name,
		}
		res, err := tx.AddProduct(c.Request.Context(), arg)

		if err != nil {
			return err
		}

		PID, err := res.LastInsertId()
		pID := uint64(PID)

		if err != nil {
			return err
		}

		args := db.AddProductToBillPurchaseParams{
			ProductID: &pID,
			Name:      &product.Name,
			Price:     product.Price,
			Quantity:  product.Quantity,
			BillID:    billId,
		}
		err = tx.AddProductToBillPurchase(c.Request.Context(), args)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *handler) GetAllPurchaseBill(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var storeIds []int
	for _, value := range h.getStores(userSession) {
		storeIds = append(storeIds, value.Id)
	}

	request := model.BillRequestFilter{
		StoreIds: storeIds,
		Page:     0,
		PageSize: 10,
	}

	if err := c.BindJSON(&request); err != nil {
		log.Printf("GetAllPurchaseBill: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": BindError(err)})
		return
	}

	if request.Page < 0 || request.PageSize <= 0 || request.StoreIds == nil || len(request.StoreIds) == 0 {
		c.Status(http.StatusBadRequest)
		return
	}

	for _, value := range request.StoreIds {
		if !slices.Contains(storeIds, value) {
			c.Status(http.StatusBadRequest)
			return
		}
	}

	bill := h.getPurchaseBills(c, int32(request.Page), int32(request.PageSize), int32(userSession.id))

	c.JSON(http.StatusOK, bill)
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
	res, err := tx.Exec("UPDATE purchase_bill SET state = -1 WHERE id = ?", pbID)
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
