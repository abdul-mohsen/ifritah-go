package handlers

// ============================================================================
// COMPLETE FILE: pkg/handlers/bill.go (with stock tracking integrated)
// ============================================================================
// Copy this file to replace: pkg/handlers/bill.go
//
// Stock changes vs original:
//   1. AddBill        — calls recordSaleMovements() before tx.Commit
//   2. SubmitDraftBill — calls recordSaleMovements() before tx.Commit
//   3. DeleteBillDetail — wrapped in tx, calls reverseSaleMovements() before soft-delete
//
// All other functions are unchanged from the original.
// ============================================================================

import (
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/invoice"
	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/models"
	"github.com/abdul-mohsen/go-arabic-pdf-lib/pkg/pdf"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func (h *handler) GetBills(c *gin.Context) {

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

	if request.Query != nil {
		data := "%" + *request.Query + "%"
		request.Query = &data
	}

	args := db.GetAllBillParams{
		Phonenumber: request.Query,
		Limit:       int32(request.PageSize),
		Offset:      int32(request.Page) * int32(request.PageSize),
	}
	bills, err := h.queries.GetAllBill(c.Request.Context(), args)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}
	c.JSON(http.StatusOK, bills)
}

func (h *handler) AddBill(c *gin.Context) {

	request := model.AddBillRequest{
		State:         1,
		PaymentMethod: 0,
		PaidAmount:    decimal.NewFromInt(0),
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

	effectiveDate := time.Now()
	if request.EffectiveDate != nil {
		parsedTime, err := time.Parse(time.RFC3339, *request.EffectiveDate)
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

	var squenceNumber uint64 = 0

	if request.State > 0 {
		squenceNumber = getNextSquenceNumber(qtx, c)
	}

	args := db.CreateBillParams{
		EffectiveDate:   effectiveDate,
		PaymentDueDate:  paymentDueDate,
		State:           int32(request.State),
		Discount:        request.Discount,
		StoreID:         int32(request.StoreId),
		SequenceNumber:  &squenceNumber,
		MerchantID:      int32(userSession.id),
		MaintenanceCost: request.MaintenanceCost,
		Note:            request.Note,
		Username:        request.UserName,
		ClientID:        request.ClientID,
		UserPhoneNumber: request.UserPhoneNumber,
		PaymentMethod:   request.PaymentMethod,
		DeliverDate:     request.DeliverDate,
		BranchID:        &request.BranchID,
	}

	res, err := qtx.CreateBill(c.Request.Context(), args)
	if err != nil {
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

	if request.MaintenanceCost.GreaterThan(decimal.Zero) {
		product := model.BillProduct{
			Name:     model.MaintenanceCost,
			Price:    request.MaintenanceCost,
			Quantity: decimal.NewFromInt(1),
		}
		products = append(products, product)
	}

	if err := addProductToBill(qtx, c, products, uint64(id)); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	// ── Stock tracking: deduct stock for catalog products ──
	enforcement := h.getStockEnforcementMode(c)
	if enforcement != model.StockEnforcementDisable && request.State > 0 {
		warnings, err := recordSaleMovements(
			qtx, c, uint64(id), int32(request.StoreId),
			request.Products, int32(squenceNumber),
			enforcement, int32(userSession.id),
		)
		if err != nil {
			// enforce mode: block if insufficient stock
			c.JSON(http.StatusBadRequest, gin.H{
				"detail": err.Error(),
				"type":   "stock_insufficient",
			})
			return
		}
		if len(warnings) > 0 {
			// warn mode: set header with stock warnings
			c.Header("X-Stock-Warning", "true")
		}
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}
	c.JSON(http.StatusCreated, id)

}

func (h *handler) SubmitDraftBill(c *gin.Context) {
	BillID, err := strconv.ParseInt(c.Param("id"), 10, 64)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	billID := uint64(BillID)

	request := model.AddBillRequest{
		State:         1,
		PaymentMethod: 0,
		PaidAmount:    decimal.NewFromInt(0),
	}

	log.Print(request)

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

	tx, err := h.DB.Begin()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	var squenceNumber uint64 = 0
	if request.State > 0 {
		squenceNumber = getNextSquenceNumber(qtx, c)
	}

	args := db.UpdateBillByIDParams{
		EffectiveDate:   time.Now(),
		PaymentDueDate:  paymentDueDate,
		State:           request.State,
		Discount:        request.Discount,
		StoreID:         request.StoreId,
		SequenceNumber:  &squenceNumber,
		MerchantID:      int32(userSession.id),
		MaintenanceCost: request.MaintenanceCost,
		Note:            request.Note,
		Username:        request.UserName,
		ClientID:        request.ClientID,
		UserPhoneNumber: request.UserPhoneNumber,
		ID:              billID,
		PaymentMethod:   request.PaymentMethod,
		DeliverDate:     request.DeliverDate,
		BranchID:        &request.BranchID,
	}

	err = qtx.UpdateBillByID(c.Request.Context(), args)

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	if err = qtx.DeleteProductToBill(c.Request.Context(), billID); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	// TODO @ssda work around when the frontend send id when he should not
	for i := range request.ManualProducts {
		request.ManualProducts[i].ProductId = nil
	}

	products := append(request.Products, request.ManualProducts...)
	if request.MaintenanceCost.GreaterThan(decimal.Zero) {
		product := model.BillProduct{
			Name:     model.MaintenanceCost,
			Price:    request.MaintenanceCost,
			Quantity: decimal.NewFromInt(1),
		}
		products = append(products, product)
	}

	if err := addProductToBill(qtx, c, products, billID); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	// ── Stock tracking: deduct stock for catalog products ──
	enforcement := h.getStockEnforcementMode(c)
	if enforcement != model.StockEnforcementDisable && request.State > 0 {
		warnings, err := recordSaleMovements(
			qtx, c, billID, int32(request.StoreId),
			request.Products, int32(squenceNumber),
			enforcement, int32(userSession.id),
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"detail": err.Error(),
				"type":   "stock_insufficient",
			})
			return
		}
		if len(warnings) > 0 {
			c.Header("X-Stock-Warning", "true")
		}
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	c.Status(http.StatusCreated)

}

func CalSubtotal(subTotal *big.Float, price string, quantity int) error {
	_price, success := stringToBigFloat(price)
	if !success || quantity <= 0 {
		return fmt.Errorf("invalid product")
	}
	_quantity := big.NewFloat(float64(quantity))
	cost := new(big.Float).Mul(_price, _quantity)
	subTotal = subTotal.Add(cost, subTotal)
	return nil
}

func addProductToBill(qtx *db.Queries, c *gin.Context, products []model.BillProduct, billId uint64) error {
	for _, product := range products {

		args := db.AddProductToBillParams{
			Name:      &product.Name,
			ProductID: product.ProductId,
			Price:     product.Price,
			Quantity:  product.Quantity,
			PartName:  &product.PartName,
			BillID:    billId,
		}
		err := qtx.AddProductToBill(c.Request.Context(), args)
		if err != nil {
			return err
		}
	}

	return nil
}

func getNextSquenceNumber(qtx *db.Queries, c *gin.Context) uint64 {

	var maxSequenceNumber int64
	maxSequenceNumber, err := qtx.GetMaxSequenceNumber(c.Request.Context())
	if err != nil {
		log.Panic(err)
	}

	return uint64(maxSequenceNumber) + 1
}

func (h *handler) getBillDetail(c *gin.Context) (model.Bill, []model.BillProductResponse) {

	Id, err := strconv.ParseUint(c.Param("id"), 10, 64)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	id := uint64(Id)

	bill, err := h.queries.GetBillPDFByID(c.Request.Context(), id)
	dbProducts, err := h.queries.GetBillProductByBillID(c.Request.Context(), bill.ID)
	var xProducts []model.BillProductResponse
	for _, product := range dbProducts {
		name := "ERR"
		if product.Name != nil {
			name = *product.Name
		} else if product.ProductID != nil {
			name = fmt.Sprint(product.ProductID)
		} else {
			log.Panic("ERR")
		}

		product := model.BillProductResponse{
			Name:           name,
			PartName:       *product.PartName,
			Quantity:       product.Quantity.Round(1).String(),
			Price:          product.Price.Round(2).String(),
			Discount:       product.Discount.Round(2).String(),
			TotalBeforeVAT: product.TotalBeforeVat.Round(2).String(),
			TotalVAT:       product.VatTotal.Round(2).String(),
			Total:          product.TotalIncludingVat.Round(2).String(),
			Type:           product.Type,
		}
		xProducts = append(xProducts, product)
	}

	products := make(map[int8][]model.BillProductResponse)
	for _, p := range xProducts {
		products[p.Type] = append(products[p.Type], p)
	}

	// TODO: @ssda Review it
	VatRegistrationNumber := ""
	if bill.VatRegistrationNumber != nil {
		VatRegistrationNumber = *bill.VatRegistrationNumber
	}

	// TODO @ssda fix this
	AddressName := "ﺍﻟﻤﺒﺮﺯ, ﺣﻲ ﺍﻟﺮﺍﺷﺪﻳﺔ ﺍﻟﺜﺎﻟﺚ, Abdullah Ibn Muaeqil"
	// if bill.AddressName != nil {
	// 	AddressName = *bill.AddressName
	// }

	StoreName := ""
	if bill.StoreName != nil {
		StoreName = *bill.StoreName
	}
	CommercialRegistrationNumber := ""

	if bill.CommercialRegistrationNumber != nil {
		CommercialRegistrationNumber = *bill.CommercialRegistrationNumber
	}

	maintenanceCost := "0.0"
	if len(products[2]) != 0 {
		maintenanceCost = products[2][0].Price

	}
	var client *db.Client = nil

	if bill.ClientID != nil {
		Client, _ := h.queries.GetClientByID(c.Request.Context(), uint32(*bill.ClientID))
		client = &Client
	}
	return model.Bill{
		Id:                           bill.ID,
		EffectiveDate:                bill.EffectiveDate,
		PaymentDueDate:               bill.PaymentDueDate,
		State:                        bill.State,
		VatRegistration:              VatRegistrationNumber,
		Address:                      AddressName,
		StoreName:                    StoreName,
		CompanyName:                  bill.CompanyName,
		SequenceNumber:               bill.SequenceNumber,
		StoreId:                      bill.StoreID,
		MerchantId:                   bill.MerchantID,
		MaintenanceCost:              maintenanceCost,
		Note:                         bill.Note,
		UserName:                     bill.Username,
		UserPhoneNumber:              bill.UserPhoneNumber,
		Products:                     products[0],
		ManualProducts:               products[1],
		CreditState:                  bill.CreditState,
		CreditNote:                   bill.CreditNote,
		QRCode:                       bill.QrCode,
		Discount:                     bill.Discount.Round(2).String(),
		TotalBeforeVAT:               bill.TotalBeforeVat.Round(2).String(),
		TotalVAT:                     bill.TotalVat.Round(2).String(),
		Total:                        bill.Total.Round(2).String(),
		CommercialRegistrationNumber: CommercialRegistrationNumber,
		CreditID:                     bill.CreditID,
		Client:                       client,
		PaymentMethod:                bill.PaymentMethod,
		DeliverDate:                  bill.DeliverDate,
		BranchID:                     bill.BranchID,
	}, xProducts
}

func (h *handler) GetBillDetail(c *gin.Context) {

	bill, _ := h.getBillDetail(c)

	c.JSON(http.StatusOK, bill)
}

func (h *handler) GetBillPDF(c *gin.Context) {

	var id string = c.Param("id")

	filename := filepath.Join("/var", "www", "html", "downloads", id+".pdf")
	if true {
		bill, products := h.getBillDetail(c)
		for _, p := range products {
			if p.Name == model.MaintenanceCost {
				p.Name = "تكلفة الصيانة"
			}
		}

		var invoice invoice.Invoice
		if bill.Client == nil {
			invoice = b2cInvoice(true, models.PaperThermal, bill, products).
				WithType(models.InvoiceTypeB2C).
				Build()
		} else {
			invoice = b2bInvoice(true, models.PaperA4, bill, products, *bill.Client).
				WithType(models.InvoiceTypeB2B).
				Build()
		}

		fontDir := "fonts"
		pdfBytes, err := pdf.GenerateInvoiceBytes(invoice, fontDir)
		if err != nil {
			log.Panic(err)
		}

		if err := os.WriteFile(filename, pdfBytes, 0644); err != nil {
			c.Header("X-Cache-Warning", err.Error())
		}

		c.Header("X-Cache", "MISS")
	}
	c.Header("X-Cache", "HIT")

	c.File(filename)

}

func (h *handler) GetBillCreditDetail(c *gin.Context) {

	Id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	id := uint64(Id)

	bill, err := h.queries.GetCreditBillByID(c.Request.Context(), id)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	c.JSON(http.StatusOK, bill)
}

func langBuilder(arabic bool) *invoice.Builder {
	b := invoice.NewBuilder()
	if arabic {
		b.WithArabic()
	} else {
		b.WithEnglish()
	}
	return b
}

func b2cInvoice(arabic bool, paper models.PaperSize, bill model.Bill, products []model.BillProductResponse) *invoice.Builder {
	var qrCode string = ""
	if bill.QRCode != nil {
		qrCode = *bill.QRCode
	}

	b := langBuilder(arabic).
		WithDate(bill.EffectiveDate).
		WithDateFormat("2006-01-02 15:04").
		WithPaper(paper).
		WithTitle("فاتورة ضريبية مبسطة").
		WithInvoiceNumber("INV-"+fmt.Sprint(*bill.SequenceNumber)).
		WithSeller(bill.CompanyName, bill.Address, bill.VatRegistration, bill.CommercialRegistrationNumber).
		WithVATPercentage("15.0").
		WithQRCode(qrCode).
		WithTotals(bill.Discount, bill.TotalBeforeVAT, bill.TotalVAT, bill.Total).
		WithStoreAddress(bill.Address).
		WithStoreName(bill.StoreName)

	for _, p := range products {
		b.AddProduct(p.Name, p.Quantity, p.Price, p.Discount, p.TotalVAT, p.Total)
	}

	return b
}

func b2bInvoice(arabic bool, paper models.PaperSize, bill model.Bill, products []model.BillProductResponse, client db.Client) *invoice.Builder {
	address := ""
	commercialRegistration := ""
	if client.Address != nil {
		address = *client.Address
	}
	if client.CommercialRegistration != nil {
		commercialRegistration = *client.CommercialRegistration
	}
	name := client.Name
	if client.CompanyName != nil {
		name = *client.CompanyName
	}

	return b2cInvoice(arabic, paper, bill, products).
		WithBuyer(name, address, client.VatNumber, commercialRegistration)

}

func (h *handler) getProducts(billId uint64) []model.ProductDetails {
	query := `
	select product_id, price, quantity , articles.id, articles.articleNumber, articles.genericArticleDescription from bill_product
	left join articles on articles.id = product_id where bill_id = ?
	`
	rows, err := h.DB.Query(query, billId)
	if err != nil {
		log.Panic(err)
	}
	var products []model.ProductDetails
	for rows.Next() {
		var product model.ProductDetails

		if err := rows.Scan(&product.Id, &product.Price, &product.Quantity, &product.ArticleId, &product.ArticleNumber, &product.Description); err != nil {
			log.Panic(err)
		}

		products = append(products, product)
	}

	return products
}

func (h *handler) DeleteBillDetail(c *gin.Context) {

	idStr := c.Param("id")
	BillID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	billID := uint64(BillID)

	userSession := GetSessionInfo(c)

	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	// ── Stock tracking: restore stock BEFORE deleting the bill (need bill data intact) ──
	if err := h.reverseSaleMovements(qtx, c, billID, int32(userSession.id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"detail": err.Error(),
			"type":   "stock_error",
		})
		return
	}

	// Soft-delete the bill
	res, err := tx.Exec("UPDATE bill SET state = -1 WHERE id = ?", billID)
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
