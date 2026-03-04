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

	fmt.Println("request:", request)

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
	// discount, _:= decimal.NewFromString(request.Discount)
	// paidAmount, success2 := stringToBigFloat(request.PaidAmount)
	maintenanceCost, err := decimal.NewFromString(request.MaintenanceCost)

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		log.Panic(err)
	}

	squenceNumber := 0
	if request.State > 0 {
		squenceNumber = h.getNextSquenceNumber(userSession.id)
	}

	args := db.CreateBillParams{
		EffectiveDate:   time.Now(),
		PaymentDueDate:  paymentDueDate,
		State:           int32(request.State),
		Discount:        request.Discount,
		StoreID:         int32(request.StoreId),
		SequenceNumber:  int32(squenceNumber),
		MerchantID:      int32(userSession.id),
		MaintenanceCost: maintenanceCost,
		Note:            request.Note,
		Username:        request.UserName,
		BuyerID:         nil,
		UserPhoneNumber: request.UserPhoneNumber,
	}

	res, err := h.queries.CreateBill(c.Request.Context(), args)
	if err != nil {
		log.Panic(err)
	}

	id, err := res.LastInsertId()

	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	if err := h.addProductToBill(request.Products, id); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}
	if err := h.addManualProductToBill(request.ManualProducts, id); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}

	c.JSON(http.StatusCreated, id)

}

func (h *handler) SubmitDraftBill(c *gin.Context) {
	var billID string = c.Param("id")

	request := model.AddBillRequest{
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
	discount, success1 := stringToBigFloat(request.Discount)
	paidAmount, success2 := stringToBigFloat(request.PaidAmount)
	maintenanceCost, success3 := stringToBigFloat(request.MaintenanceCost)

	if !(success1 && success2 && success3) {
		c.Status(http.StatusBadRequest)
		log.Panic("big float are bad")
	}

	subTotal := zeroBigFloat()

	totalWithOutVat := new(big.Float).Sub(new(big.Float).Add(subTotal, maintenanceCost), discount)
	vatTotal := new(big.Float).Mul(totalWithOutVat, big.NewFloat(.15))
	total := new(big.Float).Add(totalWithOutVat, vatTotal)

	if paidAmount.Cmp(total) == 1 {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid paid ammount"))
	}
	squenceNumber := 0
	if request.State > 0 {
		squenceNumber = h.getNextSquenceNumber(userSession.id)
	}

	query := `
	UPDATE bill SET
	effective_date = ?,
	payment_due_date = ?,
	state = ?,
	sub_total = ?,
	discount = ?,
	vat = ?,
	store_id = ?,
	sequence_number = ?,
	merchant_id = ?,
	maintenance_cost = ?,
	note = ?,
	userName = ?,
	buyer_id = ?,
	user_phone_number = ?
	WHERE id = ?;
	`
	_, err := h.DB.Exec(query, time.Now(), paymentDueDate, request.State, subTotal.Text('f', 10), discount.Text('f', 10), vatTotal.Text('f', 10),
		request.StoreId, squenceNumber, userSession.id, maintenanceCost.Text('f', 10), request.Note, request.UserName, nil, request.UserPhoneNumber, billID)
	if err != nil {
		log.Panic(err)
	}

	id, err := strconv.ParseInt(billID, 10, 64)

	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	query = `DELETE FROM bill_product where bill_id = ?;`
	if _, err = h.DB.Exec(query, billID); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}
	query = `DELETE FROM bill_manual_product where bill_id = ?;`
	if _, err = h.DB.Exec(query, billID); err != nil {
		log.Panic(err)
		c.AbortWithError(http.StatusBadRequest, err)
	}
	if err := h.addProductToBill(request.Products, id); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}
	if err := h.addManualProductToBill(request.ManualProducts, id); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
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

func (h *handler) addProductToBill(products []model.Product, billId int64) error {

	query := `insert into bill_product (product_id, price, quantity, bill_id) values (?, ?, ?, ?)`
	for _, product := range products {
		_, err := h.DB.Exec(query, product.Id, product.Price, product.Quantity, billId)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *handler) addManualProductToBill(products []model.ManualProduct, billId int64) error {

	query := `insert into bill_manual_product (part_name, price, quantity, bill_id) values (?, ?, ?, ?)`
	for _, product := range products {
		_, err := h.DB.Exec(query, product.PartName, product.Price, product.Quantity, billId)
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

func (h *handler) getBillDetail(c *gin.Context) model.Bill {

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	bill, err := h.queries.GetBillPDFByID(c.Request.Context(), int32(id))
	products, err := h.queries.GetBillProductByBillID(c.Request.Context(), bill.ID)
	manualProducts, err := h.queries.GetBillManualProductByBillID(c.Request.Context(), bill.ID)
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

	var xProducts []models.Product
	for _, product := range products {
		product := models.Product{
			Name:      fmt.Sprint(product.ID),
			Quantity:  product.Quantity[:len(product.Quantity)-4],
			UnitPrice: product.Price,
			Discount:  "0.0",
			VATAmount: product.VatTotal.Round(2).String(),
			Total:     product.TotalIncludingVat.Round(2).String(),
		}
		xProducts = append(xProducts, product)
	}

	var xManualProducts []models.Product
	for _, product := range manualProducts {
		product := models.Product{
			Name:      product.PartName,
			Quantity:  product.Quantity[:len(product.Quantity)-4],
			UnitPrice: product.Price,
			Discount:  "0.0",
			VATAmount: product.VatTotal.Round(2).String(),
			Total:     product.TotalIncludingVat.Round(2).String(),
		}
		xManualProducts = append(xManualProducts, product)
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
		MaintenanceCost:              bill.MaintenanceCost.Round(2).String(),
		Note:                         bill.Note,
		UserName:                     bill.Username,
		UserPhoneNumber:              bill.UserPhoneNumber,
		Products:                     xProducts,
		ManualProducts:               xManualProducts,
		CreditState:                  bill.CreditState,
		CreditNote:                   bill.CreditNote,
		QRCode:                       bill.QrCode,
		TotalBeforeVAT:               bill.TotalBeforeVat.Round(2).String(),
		TotalVAT:                     bill.TotalVat.Round(2).String(),
		Total:                        bill.Total.Round(2).String(),
		CommercialRegistrationNumber: CommercialRegistrationNumber,
	}
}

func (h *handler) GetBillDetail(c *gin.Context) {

	bill := h.getBillDetail(c)

	c.JSON(http.StatusOK, bill)
}

func (h *handler) GetBillPDF(c *gin.Context) {

	// userSession := GetSessionInfo(c) // to allow users to use this feature

	var id string = c.Param("id")

	filename := filepath.Join("/var", "www", "html", "downloads", id+".pdf")
	// Check if the file exists
	// _, err := os.Stat(filename)
	if true {
		bill := h.getBillDetail(c)
		products := append(bill.Products, bill.ManualProducts...)

		maintenanceCost, err := decimal.NewFromString(bill.MaintenanceCost)

		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			log.Panic(err)
		}

		if maintenanceCost.GreaterThan(decimal.NewFromInt(0)) {
			product := models.Product{
				Name:      "تكلفة الصيانة",
				Quantity:  "1",
				UnitPrice: maintenanceCost.Round(2).String(),
				Discount:  "0.0",
				VATAmount: maintenanceCost.Mul(decimal.NewFromFloat(.15)).Round(2).String(),
				Total:     maintenanceCost.Mul(decimal.NewFromFloat(1.15)).Round(2).String(),
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
