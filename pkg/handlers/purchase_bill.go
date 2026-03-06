package handlers

import (
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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
		c.AbortWithError(http.StatusBadRequest, err)
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

	tx, err := h.DB.Begin()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	args := db.UpdatePurchaseBillParams{
		EffectiveDate:  time.Now(),
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

	if err = qtx.DeleteProductPurchaseBill(c.Request.Context(), int32(id)); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	products := append(request.Products, request.ManualProducts...)

	err = addProductToBillPurchase(qtx, c, products, int32(id))
	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
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
		c.Status(http.StatusBadRequest)
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
		EffectiveDate:  time.Now(),
		PaymentDueDate: paymentDueDate,
		State:          request.State,
		Discount:       0,
		StoreID:        int32(request.StoreId),
		MerchantID:     int32(userSession.id),
		SupplierID:     request.SupplierId,
		SequenceNumber: request.SupplierSequenceNumber,
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

	products := append(request.Products, request.ManualProducts...)

	err = addProductToBillPurchase(qtx, c, products, int32(id))
	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
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
		log.Panic(err)
		c.Status(http.StatusBadRequest)
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

	products := make(map[int8][]db.PurchaseBillProduct)
	for _, p := range xProducts {
		products[p.Type] = append(products[p.Type], p)
	}

	bill := model.PurchaseBill{
		Id:             b.ID,
		EffectiveDate:  b.EffectiveDate,
		PaymentDueDate: b.PaymentDueDate,
		State:          b.State,
		Discount:       b.Discount,
		SequenceNumber: b.SequenceNumber,
		StoreId:        b.StoreID,
		MerchantId:     int(b.MerchantID),
		Products:       products[0],
		ManualProducts: products[2],
		TotalBeforeVAT: b.TotalBeforeVat.Round(2).String(),
		TotalVAT:       b.TotalVat.Round(2).String(),
		Total:          b.Total.Round(2).String(),
	}
	c.JSON(http.StatusOK, bill)
}

func (h *handler) DeletePurchaseBillDetail(c *gin.Context) {

	var id string = c.Param("id")

	// TODO check if the user has right to delete and is the owner of the bill
	res, err := h.DB.Exec("update purchase_bill set state = -1 where id = ?", id)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}

	if res == nil {
		c.AbortWithStatus(http.StatusInternalServerError)
	}

	affectedRows, err := res.RowsAffected()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	if affectedRows == 0 {
		c.AbortWithStatus(http.StatusBadRequest)
	}

	c.Status(http.StatusOK)
}
