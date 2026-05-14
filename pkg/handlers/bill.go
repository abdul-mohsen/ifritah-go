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
	"context"
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
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

// checkBillStoreAccess returns true when the caller has access to the given
// store; otherwise it writes a 400 response and returns false.
func (h *handler) checkBillStoreAccess(c *gin.Context, storeID int32) bool {
	storeIds := h.getStoreIds(c)
	if !slices.Contains(storeIds, storeID) {
		c.Status(http.StatusBadRequest)
		return false
	}
	return true
}

// effectiveDateOr returns ed when supplied, otherwise time.Now().
func effectiveDateOr(ed *time.Time) time.Time {
	if ed != nil {
		return *ed
	}
	return time.Now()
}

// billListSort is the canonical sort spec for the bill list. Cursor
// validation rejects any client-supplied sort that doesn't match this.
const billListSort = "-effective_date"

// errGetBills is the log-prefix format used by every error path in
// GetBills (Sonar S1192).
const errGetBills = "GetBills: %v"

// billCursorKeys is the keyset extractor BuildEnvelope uses to mint
// the next-page cursor. (effective_date, id, is_credit) — the
// is_credit tiebreaker is required because credit-note variants
// share the bill's id and date in the UNION-ALL output.
func billCursorKeys(r db.GetAllBillRow) []any {
	return []any{
		r.EffectiveDate.UTC().Format(time.RFC3339Nano),
		r.ID,
		r.IsCredit,
	}
}

func (h *handler) GetBills(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var storeIds []int
	for _, value := range h.getStores(userSession) {
		storeIds = append(storeIds, value.Id)
	}

	request := model.BillRequestFilter{}

	if err := c.BindJSON(&request); err != nil {
		log.Printf(errGetBills, err)
		c.Status(http.StatusBadRequest)
		return
	}

	// Build a ListRequest view to hand to the shared validator. It
	// shares the same wire keys (limit/cursor/sort/page_size/...).
	listReq := pagination.ListRequest{
		Limit:      request.Limit,
		Cursor:     request.Cursor,
		Sort:       request.Sort,
		Dir:        request.Dir,
		Query:      request.Query,
		PageNumber: request.Page,
		PageSize:   request.PageSize,
	}
	if err := listReq.Validate(billListSort); err != nil {
		log.Printf(errGetBills, err)
		c.Status(http.StatusBadRequest)
		return
	}

	// store_ids is a filter, not a primary key. When omitted, default to
	// every store the caller can access. Truly no accessible stores means
	// "empty result", not "bad request".
	if len(request.StoreIds) == 0 {
		request.StoreIds = storeIds
	}
	if len(request.StoreIds) == 0 {
		c.JSON(http.StatusOK, pagination.Envelope[db.GetAllBillRow]{Items: []db.GetAllBillRow{}})
		return
	}
	if !allStoresAllowed(request.StoreIds, storeIds) {
		c.Status(http.StatusBadRequest)
		return
	}

	queryNameLike, queryPhoneDigits := buildBillSearchParams(request.Query)
	stateFilter := nonNegativeStateFilter(request.State)
	phonePrefix := buildPlainPrefixFilter(request.Phone)
	seqPrefix := buildPlainPrefixFilter(request.SequenceNumber)

	cur, _ := listReq.DecodedCursor()
	cursorDate, cursorID, cursorIsCredit, ok := decodeBillCursor(cur)
	if !ok {
		log.Printf("GetBills: cursor missing date, id or is_credit")
		c.Status(http.StatusBadRequest)
		return
	}

	limit := listReq.EffectiveLimit()

	args := db.GetAllBillParams{
		StateFilter:      stateFilter,
		QueryNameLike:    queryNameLike,
		QueryPhoneDigits: queryPhoneDigits,
		PhonePrefix:      phonePrefix,
		SeqPrefix:        seqPrefix,
		CursorDate:       cursorDate,
		CursorID:         cursorID,
		CursorIsCredit:   nullInt64FromAny(cursorIsCredit),
		Limit:            int32(limit + 1),
	}
	bills, err := h.queries.GetAllBill(c.Request.Context(), args)
	if err != nil {
		log.Printf(errGetBills, err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	envelope := pagination.BuildEnvelope(bills, limit, billListSort, billCursorKeys)
	c.JSON(http.StatusOK, envelope)
}

func (h *handler) AddBill(c *gin.Context) {

	request := model.AddBillRequest{
		State:         1,
		PaymentMethod: 0,
		PaidAmount:    decimal.NewFromInt(0),
	}

	if err := c.BindJSON(&request); err != nil {
		log.Printf("AddBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	userSession := GetSessionInfo(c)

	if !h.checkBillStoreAccess(c, request.StoreId) {
		return
	}

	paymentDueDate := request.PaymentDueDate
	effectiveDate := effectiveDateOr(request.EffectiveDate)
	tx, err := h.DB.Begin()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
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
		log.Printf("AddBill: %v", err)
		if IsDuplicate(err) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"message": "Supplier bill number already exists for this supplier",
			})
		}
		return
	}

	id, err := res.LastInsertId()

	if err != nil {
		log.Printf("AddBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
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
		return
	}

	if err := qtx.RefreshBillTotals(c.Request.Context(), uint64(id)); err != nil {
		log.Printf("AddBill refresh totals: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
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
			log.Printf("AddBill: %v", err)
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

	if request.State > 0 {
		if err := h.pub.SubmitBill(id, int64(request.BranchID)); err != nil {
			log.Printf("zatca publish failed: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, id)

}

func (h *handler) SubmitDraftBill(c *gin.Context) {
	BillID, err := strconv.ParseInt(c.Param("id"), 10, 64)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	billID := uint64(BillID)

	request := model.AddBillRequest{
		State:         1,
		PaymentMethod: 0,
		PaidAmount:    decimal.NewFromInt(0),
	}

	log.Print(request)

	if err := c.BindJSON(&request); err != nil {
		log.Printf("SubmitDraftBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	userSession := GetSessionInfo(c)

	if !h.checkBillStoreAccess(c, request.StoreId) {
		return
	}

	paymentDueDate := request.PaymentDueDate

	tx, err := h.DB.Begin()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	var squenceNumber uint64 = 0
	if request.State > 0 {
		squenceNumber = getNextSquenceNumber(qtx, c)
	}

	args := db.UpdateBillByIDParams{
		EffectiveDate:   effectiveDateOr(request.EffectiveDate),
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
		return
	}

	if err = qtx.DeleteProductToBill(c.Request.Context(), billID); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
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
		return
	}

	if err := qtx.RefreshBillTotals(c.Request.Context(), billID); err != nil {
		log.Printf("SubmitDraftBill refresh totals: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
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
			log.Printf("SubmitDraftBill: %v", err)
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

	if request.State > 0 {
		if err := h.pub.SubmitBill(BillID, int64(request.BranchID)); err != nil {
			log.Printf("zatca publish failed: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
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
		log.Printf("getNextSquenceNumber: %v", err)
		return 0
	}

	return uint64(maxSequenceNumber) + 1
}

func (h *handler) getBillDetail(c *gin.Context) (model.Bill, []model.BillProductResponse) {

	Id, err := strconv.ParseUint(c.Param("id"), 10, 64)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return model.Bill{}, nil
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
			return model.Bill{}, nil
		}

		u := ""
		if product.PartName == nil {

			product.PartName = &u
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
		ClientID:                     bill.ClientID,
		// `type` was historically the "is the bill issued to a registered
		// client" flag (B2B) vs an anonymous walk-in (B2C). It is fully
		// derivable from the presence of client_id, so do that here instead
		// of leaving the field as the zero value.
		Type:          bill.ClientID != nil,
		PaymentMethod: bill.PaymentMethod,
		DeliverDate:   bill.DeliverDate,
		BranchID:      bill.BranchID,
	}, xProducts
}

func (h *handler) GetBillDetail(c *gin.Context) {

	bill, _ := h.getBillDetail(c)

	c.JSON(http.StatusOK, bill)
}

func (h *handler) GetBillPDF(c *gin.Context) {

	// Validate id is a positive integer before composing a path with it.
	// Path-traversal hardening (Sonar gosecurity:S2083): the param feeds
	// filepath.Join below, so any value containing path separators or
	// `..` segments could escape the downloads directory.
	rawID := c.Param("id")
	billID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || billID == 0 {
		c.Status(http.StatusBadRequest)
		return
	}
	id := strconv.FormatUint(billID, 10)

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
			return
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
		return
	}

	id := uint64(Id)

	bill, err := h.queries.GetCreditBillByID(c.Request.Context(), id)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
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

	bill.Address = "ﺍﻟﻤﺒﺮﺯ, ﺣﻲ ﺍﻟﺮﺍﺷﺪﻳﺔ ﺍﻟﺜﺎﻟﺚ, Abdullah Ibn Muaeqil"

	b := langBuilder(arabic).
		WithDate(bill.EffectiveDate.Local()).
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

	if bill.UserName != nil {
		b.WithBuyer(*bill.UserName+" ", "", "", "")
	}

	for _, p := range products {
		b.AddProduct(p.Name+" "+p.PartName, p.Quantity, p.Price, p.Discount, p.TotalVAT, p.Total)
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
	rows, err := h.queries.GetBillProductsWithArticle(context.Background(), billId)
	if err != nil {
		log.Printf("getBillProducts: %v", err)
		return nil
	}
	products := make([]model.ProductDetails, 0, len(rows))
	for _, r := range rows {
		p := model.ProductDetails{
			Price:    r.Price.String(),
			Quantity: r.Quantity.IntPart(),
		}
		if r.ProductID != nil {
			p.Id = int(*r.ProductID)
		}
		if r.ArticleID.Valid {
			p.ArticleId = int(r.ArticleID.Int64)
		}
		if r.Articlenumber != nil {
			p.ArticleNumber = *r.Articlenumber
		}
		if r.Genericarticledescription != nil {
			p.Description = *r.Genericarticledescription
		}
		products = append(products, p)
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
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	// ── Stock tracking: restore stock BEFORE deleting the bill (need bill data intact) ──
	if err := h.reverseSaleMovements(qtx, c, billID, int32(userSession.id)); err != nil {
		log.Printf("DeleteBillDetail: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"detail": err.Error(),
			"type":   "stock_error",
		})
		return
	}

	// Soft-delete the bill
	res, err := qtx.SoftDeleteBill(c.Request.Context(), billID)
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
