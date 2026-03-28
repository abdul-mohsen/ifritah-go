package handlers

import (
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
)

func (h *handler) getPurchaseBills(c *gin.Context, page int32, pageSize int32, userID int32) []db.PurchaseBillTotal {

	args := db.GetAllPurchaseBillParams{
		ID:     userID,
		Limit:  pageSize,
		Offset: pageSize * page,
	}

	bills, err := h.queries.GetAllPurchaseBill(c.Request.Context(), args)

	if err != nil {
		log.Panic(err)
	}

	return bills
}

func (h *handler) UpdatePurchaseBill(c *gin.Context) {

	id, err := strconv.ParseInt(c.Param("id"), 10, 32)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}
	request := model.AddPurchaseBillRequest{
		State:         1,
		PaymentMethod: 0,
		PaidAmount:    "0.0",
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": BindError(err)})
		log.Panic(err)
	}

	userSession := GetSessionInfo(c)

	storeIds := h.getStoreIds(c)

	if !slices.Contains(storeIds, request.StoreId) {
		c.Status(http.StatusBadRequest)
		log.Panic("invalid store id")
	}

	var paymentDueDate *time.Time
	if request.PaymentDueDate != nil {
		parsedTime, err := time.Parse(time.RFC3339, *request.PaymentDueDate)
		paymentDueDate = &parsedTime
		if err != nil {
			log.Panic("Error parsing date:", err)
		}
	}

	effectiveDate := time.Now()
	if request.EffectiveDate != nil {
		parsedTime, err := time.Parse(time.DateOnly, *request.EffectiveDate)
		effectiveDate = parsedTime
		if err != nil {
			log.Panic("Error parsing date:", err)
		}
	}

	tx, err := h.DB.Begin()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	args := db.UpdatePurchaseBillParams{
		EffectiveDate:  effectiveDate,
		PaymentDueDate: paymentDueDate,
		State:          request.State,
		Discount:       0,
		StoreID:        request.StoreId,
		MerchantID:     int32(userSession.id),
		SupplierID:     request.SupplierId,
		SequenceNumber: request.SupplierSequenceNumber,
		ID:             int32(id),
	}

	err = qtx.UpdatePurchaseBill(c.Request.Context(), args)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	// ── Stock tracking: reverse old stock before deleting products ──
	enforcement := h.getStockEnforcementMode()
	if enforcement != model.StockEnforcementDisable {
		if err := h.reversePurchaseMovements(tx, int32(id), int32(userSession.id)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"detail": err.Error(),
				"type":   "stock_error",
			})
			return
		}
	}

	if err = qtx.DeleteProductPurchaseBill(c.Request.Context(), int32(id)); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	// TODO @ssda work around when the frontend send id when he should not
	for i := range request.ManualProducts {
		request.ManualProducts[i].ProductId = nil
	}

	products := append(request.Products, request.ManualProducts...)

	err = addProductToBillPurchase(qtx, c, products, int32(id))
	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	// ── Stock tracking: add new stock for updated products ──
	if enforcement != model.StockEnforcementDisable && request.State > 0 {
		if err := recordPurchaseMovements(
			tx, int32(id), int32(request.StoreId),
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
		log.Panic(err)
	}

	c.Status(http.StatusOK)

}

func (h *handler) AddPurchaseBill(c *gin.Context) {

	request := model.AddPurchaseBillRequest{
		State:         1,
		PaymentMethod: 0,
		PaidAmount:    "0.0",
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": BindError(err)})
		log.Panic(err)
	}

	userSession := GetSessionInfo(c)

	storeIds := h.getStoreIds(c)

	if !slices.Contains(storeIds, request.StoreId) {
		c.Status(http.StatusBadRequest)
		log.Panic("invalid store id")
	}

	var paymentDueDate *time.Time
	if request.PaymentDueDate != nil {
		parsedTime, err := time.Parse(time.RFC3339, *request.PaymentDueDate)
		paymentDueDate = &parsedTime
		if err != nil {
			log.Panic("Error parsing date:", err)
		}
	}

	effectiveDate := time.Now()
	if request.EffectiveDate != nil {
		parsedTime, err := time.Parse(time.DateOnly, *request.EffectiveDate)
		effectiveDate = parsedTime
		if err != nil {
			log.Panic("Error parsing date:", err)
		}
	}

	_, success1 := stringToBigFloat(request.Discount)
	_, success2 := stringToBigFloat(request.PaidAmount)

	if !(success1 && success2) {
		c.Status(http.StatusBadRequest)
		log.Panic("big float are bad")
	}

	tx, err := h.DB.Begin()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	args := db.AddPurchaseBillParams{
		EffectiveDate:  effectiveDate,
		PaymentDueDate: paymentDueDate,
		State:          request.State,
		Discount:       0,
		StoreID:        int32(request.StoreId),
		MerchantID:     int32(userSession.id),
		SupplierID:     request.SupplierId,
		SequenceNumber: request.SupplierSequenceNumber,
		PdfLink:        request.PDFLink,
	}
	res, err := qtx.AddPurchaseBill(c.Request.Context(), args)
	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	id, err := res.LastInsertId()

	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	// TODO @ssda work around when the frontend send id when he should not
	for i := range request.ManualProducts {
		request.ManualProducts[i].ProductId = nil
	}

	products := append(request.Products, request.ManualProducts...)

	err = addProductToBillPurchase(qtx, c, products, int32(id))
	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	// ── Stock tracking: add stock for catalog products ──
	enforcement := h.getStockEnforcementMode()
	if enforcement != model.StockEnforcementDisable && request.State > 0 {
		if err := recordPurchaseMovements(
			tx, int32(id), int32(request.StoreId),
			request.Products, request.SupplierSequenceNumber,
			enforcement, int32(userSession.id),
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"detail": err.Error(),
				"type":   "stock_error",
			})
			log.Panic(err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	var attachment []string
	for _, a := range request.Attachments {
		attachment = append(attachment, a)

	}
	if err := h.SavePurchaseBillAttachments(h.DB, id, *request.PDFLink, attachment); err != nil {
		// TODO @ssda review this to force pdf upload now it is blocked by the ui
		// c.AbortWithError(http.StatusInternalServerError, err)
		// log.Panic(err)
	}

	c.Status(http.StatusCreated)

}

func addProductToBillPurchase(tx *db.Queries, c *gin.Context, products []model.PurchaseBillProduct, billId int32) error {

	for _, product := range products {

		args := db.AddProductToBillPurchaseParams{
			ProductID: product.ProductId,
			Name:      &product.Name,
			Price:     product.Price,
			Quantity:  product.Quantity,
			BillID:    billId,
		}
		err := tx.AddProductToBillPurchase(c.Request.Context(), args)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": BindError(err)})
		log.Panic(err)
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

	id, err := strconv.ParseInt(c.Param("id"), 10, 32)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	args := db.GetPurchaseBillDetailParams{
		ID:   int32(userSession.id),
		ID_2: int32(id),
	}

	b, err := h.queries.GetPurchaseBillDetail(c.Request.Context(), args)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	xProducts, err := h.queries.GetPurchaseBillProducts(c.Request.Context(), int32(id))

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	products := make(map[int8][]db.PurchaseBillProduct)
	for _, p := range xProducts {
		products[p.Type] = append(products[p.Type], p)
	}

	a, err := h.queries.GetPurchaseBillAttachments(c.Request.Context(), int32(id))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
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
		SupplierSequenceNumber: b.SequenceNumber,
		EffectiveDate:          b.EffectiveDate,
		PaymentDueDate:         b.PaymentDueDate,
		State:                  b.State,
		Discount:               b.Discount,
		SequenceNumber:         b.SequenceNumber,
		StoreId:                b.StoreID,
		MerchantId:             int(b.MerchantID),
		Products:               products[0],
		ManualProducts:         products[1],
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
	pbID, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userSession := GetSessionInfo(c)

	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}
	defer tx.Rollback()

	// ── Stock tracking: reverse stock BEFORE deleting (need PB data intact) ──
	if err := h.reversePurchaseMovements(tx, int32(pbID), int32(userSession.id)); err != nil {
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
		log.Panic(err)
	}

	affectedRows, err := res.RowsAffected()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	if affectedRows == 0 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
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
