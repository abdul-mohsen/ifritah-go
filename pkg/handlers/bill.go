package handlers

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
	// , , pageSize, page*pageSize)
	c.JSON(http.StatusOK, bills)
}

func (h *handler) AddBill(c *gin.Context) {

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
	// discount, _:= decimal.NewFromString(request.Discount)
	// paidAmount, success2 := stringToBigFloat(request.PaidAmount)

	squenceNumber := 0
	if request.State > 0 {
		squenceNumber = h.getNextSquenceNumber(userSession.id)
	}

	tx, err := h.DB.Begin()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	args := db.CreateBillParams{
		EffectiveDate:   time.Now(),
		PaymentDueDate:  paymentDueDate,
		State:           int32(request.State),
		Discount:        request.Discount.String(),
		StoreID:         int32(request.StoreId),
		SequenceNumber:  int32(squenceNumber),
		MerchantID:      int32(userSession.id),
		MaintenanceCost: request.MaintenanceCost,
		Note:            request.Note,
		Username:        request.UserName,
		BuyerID:         nil,
		UserPhoneNumber: request.UserPhoneNumber,
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

	if err := addProductToBill(qtx, c, products, int32(id)); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}
	c.JSON(http.StatusCreated, id)

}

func (h *handler) SubmitDraftBill(c *gin.Context) {
	billID, err := strconv.ParseInt(c.Param("id"), 10, 32)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

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

	squenceNumber := 0
	if request.State > 0 {
		squenceNumber = h.getNextSquenceNumber(userSession.id)
	}

	tx, err := h.DB.Begin()

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	args := db.UpdateBillByIDParams{
		EffectiveDate:   time.Now(),
		PaymentDueDate:  paymentDueDate,
		State:           request.State,
		Discount:        request.Discount.String(),
		StoreID:         request.StoreId,
		SequenceNumber:  int32(squenceNumber),
		MerchantID:      int32(userSession.id),
		MaintenanceCost: request.MaintenanceCost,
		Note:            request.Note,
		Username:        request.UserName,
		BuyerID:         nil,
		UserPhoneNumber: request.UserPhoneNumber,
		ID:              int32(billID),
	}

	err = qtx.UpdateBillByID(c.Request.Context(), args)

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	if err = qtx.DeleteProductToBill(c.Request.Context(), int32(billID)); err != nil {
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

	if err := addProductToBill(qtx, c, products, int32(billID)); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
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

func addProductToBill(qtx *db.Queries, c *gin.Context, products []model.BillProduct, billId int32) error {
	for _, product := range products {

		args := db.AddProductToBillParams{
			Name:      &product.Name,
			ProductID: product.ProductId,
			Price:     product.Price,
			Quantity:  product.Quantity,
			BillID:    billId,
		}
		err := qtx.AddProductToBill(c.Request.Context(), args)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *handler) getNextSquenceNumber(id int64) int {

	query := `
	select COALESCE(max(sequence_number), 1) from bill
	join store on store.id = bill.store_id
	join company on store.company_id = company.id
	join user on user.company_id = company.id and user.id = ?
	`
	var maxSequenceNumber int
	if err := h.DB.QueryRow(query, id).Scan(&maxSequenceNumber); err != nil {
		log.Panic(err)
	}

	return maxSequenceNumber + 1
}

func (h *handler) getBillDetail(c *gin.Context) (model.Bill, []model.BillProductResponse) {

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	bill, err := h.queries.GetBillPDFByID(c.Request.Context(), int32(id))
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
			Quantity:       product.Quantity.Round(1).String(),
			Price:          product.Price.Round(2).String(),
			Discount:       "0.0",
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

	AddressName := ""
	if bill.AddressName != nil {
		AddressName = *bill.AddressName
	}

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

	return model.Bill{
		Id:                           bill.ID,
		EffectiveDate:                bill.EffectiveDate,
		PaymentDueDate:               bill.PaymentDueDate,
		State:                        bill.State,
		Discount:                     bill.Discount,
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
		TotalBeforeVAT:               bill.TotalBeforeVat.Round(2).String(),
		TotalVAT:                     bill.TotalVat.Round(2).String(),
		Total:                        bill.Total.Round(2).String(),
		CommercialRegistrationNumber: CommercialRegistrationNumber,
		CreditID:                     bill.CreditID,
	}, xProducts
}

func (h *handler) GetBillDetail(c *gin.Context) {

	bill, _ := h.getBillDetail(c)

	c.JSON(http.StatusOK, bill)
}

func (h *handler) GetBillPDF(c *gin.Context) {

	// userSession := GetSessionInfo(c) // to allow users to use this feature

	var id string = c.Param("id")

	filename := filepath.Join("/var", "www", "html", "downloads", id+".pdf")
	// Check if the file exists
	// _, err := os.Stat(filename)
	if true {
		bill, xProducts := h.getBillDetail(c)
		var products []models.Product
		name := "تكلفة الصيانة"
		for _, p := range xProducts {
			if p.Name == model.MaintenanceCost {
				p.Name = name
			}
			product := models.Product{
				Name:      p.Name,
				Quantity:  p.Quantity,
				UnitPrice: p.Price,
				Discount:  p.Discount,
				VATAmount: p.TotalVAT,
				Total:     p.Total,
			}
			products = append(products, product)

		}
		qr := ""

		if bill.QRCode != nil {
			qr = *bill.QRCode
		}

		invoice := models.Invoice{
			Title:                        "فاتورة ضريبية مبسطة",
			InvoiceNumber:                fmt.Sprintf("INV%d", bill.SequenceNumber),
			StoreName:                    bill.StoreName,
			StoreAddress:                 bill.Address,
			Date:                         bill.EffectiveDate.Local().Format(time.DateTime),
			VATRegistrationNo:            bill.VatRegistration,
			QRCodeData:                   qr,
			TotalDiscount:                "0.0",
			TotalTaxableAmt:              bill.TotalBeforeVAT,
			TotalVAT:                     bill.TotalVAT,
			TotalWithVAT:                 bill.Total,
			VATPercentage:                "15",
			CommercialRegistrationNumber: bill.CommercialRegistrationNumber,
			Labels: models.Labels{
				CommercialRegistrationNumber: "رقم السجل التجاري",
				InvoiceNumber:                "رقم الفاتورة:",
				Date:                         "تاريخ:",
				VATRegistration:              "رقم تسجيل ضريبة القيمة المضافة:",
				TotalTaxable:                 "اجمالي المبلغ الخاضع للضريبة",
				TotalWithVat:                 "المجموع مع الضريبة",
				ProductColumn:                "المنتجات",
				QuantityColumn:               "الكمية",
				UnitPriceColumn:              "سعر الوحدة",
				DiscountColumn:               "الخصم",
				VATAmountColumn:              "ضريبة القيمة المضافة",
				TotalColumn:                  "السعر شامل الضريبة",
				TotalDiscount:                "إجمالي الخصم",
				Footer:                       fmt.Sprintf(">>>>>>>>>>>>>> إغلاق الفاتورة %d <<<<<<<<<<<<<<<", bill.SequenceNumber),
				Vat:                          "ضريبة القيمة المضافة (%51)",
			},
			Language: "ar",
			Products: products,
			IsRTL:    true,
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

	// Upload the file to specific dst.
	c.File(filename)

}

func (h *handler) GetBillCreditDetail(c *gin.Context) {

	// userSession := GetSessionInfo(c) // to allow users to use this feature

	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}
	bill, err := h.queries.GetCreditBillByID(c.Request.Context(), int32(id))

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	c.JSON(http.StatusOK, bill)
}

func (h *handler) getProducts(billId int) []model.ProductDetails {
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

	var id string = c.Param("id")

	// TODO check if the user has right to delete and is the owner of the bill
	res, err := h.DB.Exec("update bill set state = -1 where id = ?", id)
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
