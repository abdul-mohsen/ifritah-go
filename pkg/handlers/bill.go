package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	db "ifritah/web-service-gin/pkg/db/gen"
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
)

type BillBase struct {
	Id             int           `json:"id"`
	EffectiveDate  sql.NullTime  `json:"effective_date"`
	PaymentDueDate *sql.NullTime `json:"payment_due_date"`
	State          int           `json:"state"`
	SubTotal       float64       `json:"subtotal"`
	Total          float64       `json:"total"`
	TotalVAT       float64       `json:"total_vat"`
	TotalBeforeVAT float64       `json:"total_before_vat"`
	Discount       float64       `json:"discount"`
	Vat            float64       `json:"vat"`
	SequenceNumber int           `json:"sequence_number"`
	Type           bool          `json:"type"`
	CreditState    *int          `json:"credit_state"`
}

type BillRequestFilter struct {
	StoreIds  []int      `json:"store_ids"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	Page      int        `json:"page_number"`
	PageSize  int        `json:"page_size"`
	Query     string     `json:"query"`
}

func (h *handler) GetBills(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var storeIds []int
	for _, value := range h.getStores(userSession) {
		storeIds = append(storeIds, value.Id)
	}

	request := BillRequestFilter{
		StoreIds: storeIds,
		Page:     0,
		PageSize: 10,
		Query:    "",
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

	if request.Query != "" {
		request.Query = "%" + request.Query + "%"
	}

	args := db.GetAllBillParams{
		UserPhoneNumber:   &request.Query,
		UserPhoneNumber_2: &request.Query,
		Limit:             int32(request.PageSize),
		Offset:            int32(request.Page) * int32(request.PageSize),
	}
	bills, err := h.queries.GetAllBill(c.Request.Context(), args)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}
	// , , pageSize, page*pageSize)
	c.JSON(http.StatusOK, bills)
}

type AddBillRequest struct {
	StoreId         int             `json:"store_id" binding:"required"`
	State           int8            `json:"state"`
	PaymentDueDate  *string         `json:"payment_due_date" `
	PaymentDate     *string         `json:"payment_date" `
	Discount        string          `json:"discount" binding:"required"`
	PaidAmount      string          `json:"paidAmount" `
	MaintenanceCost string          `json:"maintenance_cost" binding:"required"`
	PaymentMethod   int8            `json:"payment_method"`
	UserName        *string         `json:"user_name"`
	UserPhoneNumber *string         `json:"user_phone_number"`
	Note            *string         `json:"note"`
	Products        []Product       `json:"products" binding:"required,dive"`
	ManualProducts  []ManualProduct `json:"manual_products" binding:"required,dive"`
}

type Product struct {
	Id          int32  `json:"id" binding:"required"`
	Price       string `json:"price" binding:"required"`
	CostPrice   string `json:"cost_price"`
	ShelfNumber string `json:"shelf_number"`
	Quantity    string `json:"quantity"`
}

type ManualProduct struct {
	PartName string `json:"part_name" binding:"required"`
	Price    string `json:"price" binding:"required"`
	Quantity string `json:"quantity" binding:"required"`
}

type TempProduct struct {
	Id       int     `json:"id" binding:"required"`
	Price    float64 `json:"price" binding:"required"`
	Quantity int64   `json:"quantity" binding:"required"`
}

type TempManualProduct struct {
	PartName string  `json:"part_name" binding:"required"`
	Price    float64 `json:"price" binding:"required"`
	Quantity int64   `json:"quantity" binding:"required"`
}

func (h *handler) AddBill(c *gin.Context) {

	request := AddBillRequest{
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

	args := db.CreateBillParams{
		EffectiveDate:   time.Now(),
		PaymentDueDate:  paymentDueDate,
		State:           int32(request.State),
		Discount:        discount.Text('f', 10),
		StoreID:         int32(request.StoreId),
		SequenceNumber:  int32(squenceNumber),
		MerchantID:      int32(userSession.id),
		MaintenanceCost: maintenanceCost.Text('f', 10),
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

	request := AddBillRequest{
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
	log.Printf("Started")
	_, err := h.DB.Exec(query, time.Now(), paymentDueDate, request.State, subTotal.Text('f', 10), discount.Text('f', 10), vatTotal.Text('f', 10),
		request.StoreId, squenceNumber, userSession.id, maintenanceCost.Text('f', 10), request.Note, request.UserName, nil, request.UserPhoneNumber, billID)
	log.Printf("I update the main row to the product")
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
	log.Printf("Dropped old product ")
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
	log.Printf("in my calc func")
	return nil
}

func (h *handler) addProductToBill(products []Product, billId int64) error {

	query := `insert into bill_product (product_id, price, quantity, bill_id) values (?, ?, ?, ?)`
	for _, product := range products {
		_, err := h.DB.Exec(query, product.Id, product.Price, product.Quantity, billId)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *handler) addManualProductToBill(products []ManualProduct, billId int64) error {

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

type ProductDetails struct {
	Id            int    `json:"id" binding:"required"`
	Price         string `json:"price" binding:"required"`
	Quantity      int64  `json:"quantity" binding:"required"`
	ArticleId     int    `json:"article_id"`
	ArticleNumber string `json:"article_number"`
	Description   string `json:"description"`
}

type Bill struct {
	Id                           int             `json:"id"`
	EffectiveDate                sql.NullTime    `json:"effective_date"`
	PaymentDueDate               *sql.NullTime   `json:"payment_due_date"`
	State                        int             `json:"state"`
	SubTotal                     float64         `json:"subtotal"`
	Discount                     float64         `json:"discount"`
	Vat                          float64         `json:"vat"`
	VatRegistration              string          `json:"vat_registration"`
	Address                      string          `json:"address"`
	StoreName                    string          `json:"store_name"`
	CompanyName                  string          `json:"company_name"`
	SequenceNumber               int             `json:"sequence_number"`
	Type                         bool            `json:"type"`
	StoreId                      int             `json:"store_id"`
	MerchantId                   int             `json:"merchant_id"`
	MaintenanceCost              string          `json:"maintenance_cost"`
	Note                         *string         `json:"note"`
	UserName                     *string         `json:"user_name"`
	UserPhoneNumber              *string         `json:"user_phone_number"`
	Url                          *string         `json:"url"`
	Products                     json.RawMessage `json:"products"`
	ManualProducts               json.RawMessage `json:"manual_products"`
	CreditState                  *int            `json:"credit_state"`
	CreditNote                   *string         `json:"credit_note"`
	QRCode                       *string         `json:"qr_code"`
	TotalBeforeVAT               string          `json:"total_before_vat"`
	TotalVAT                     string          `json:"total_vat"`
	Total                        string          `json:"total"`
	CommercialRegistrationNumber string
}

func (h *handler) getBillDetail(c *gin.Context) db.GetBillByIDRow {

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	bill, err := h.queries.GetBillByID(c.Request.Context(), int32(id))

	return bill
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
		var products []models.Product
		x, err := json.Marshal(bill.Products)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			log.Panic(err)
		}

		var mProduct []TempProduct
		err = json.Unmarshal(x, &mProduct)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			log.Panic(err)
		}

		y, err := json.Marshal(bill.ManualProducts)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			log.Panic(err)
		}
		var mManProduct []TempManualProduct
		err = json.Unmarshal(y, &mManProduct)

		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			log.Panic(err)
		}

		for _, product := range mProduct {
			price := product.Price

			// TODO fix this logic
			product := models.Product{
				Name:      fmt.Sprint(product.Id),
				Quantity:  fmt.Sprint(float64(product.Quantity)),
				UnitPrice: fmt.Sprint(price),
				Discount:  "0.0",
				VATAmount: fmt.Sprintf("%.2f", price*.15),
				Total:     fmt.Sprintf("%.2f", price*1.15),
			}
			products = append(products, product)

		}

		for _, product := range mManProduct {
			// f, _, err := big.ParseFloat(product.Price, 10, 0, big.ToNearestEven)
			// if err != nil {
			// 	log.Panic(err)
			// }
			// price, _ := f.Float64()
			price := product.Price

			// TODO fix this logic
			product := models.Product{
				Name:      product.PartName,
				Quantity:  fmt.Sprint(product.Quantity),
				UnitPrice: fmt.Sprint(price),
				Discount:  "0.0",
				VATAmount: fmt.Sprintf("%.2f", price*.15),
				Total:     fmt.Sprintf("%.2f", price*1.15),
			}
			products = append(products, product)

		}

		f, _, err := big.ParseFloat(bill.MaintenanceCost, 10, 256, big.ToNearestEven)

		if err != nil {
			log.Println(err)
		}
		maintenanceCost, _ := f.Float64()

		if maintenanceCost > 0 {
			product := models.Product{
				Name:      "تكلفة الصيانة",
				Quantity:  "1",
				UnitPrice: fmt.Sprintf("%.2f", maintenanceCost),
				Discount:  "0.0",
				VATAmount: fmt.Sprintf("%.2f", maintenanceCost*.15),
				Total:     fmt.Sprintf("%.2f", maintenanceCost*1.15),
			}
			products = append(products, product)

		}
		qr := ""

		if bill.QrCode != nil {
			qr = *bill.QrCode
		}

		invoice := models.Invoice{
			Title:                        "فاتورة ضريبية مبسطة",
			InvoiceNumber:                fmt.Sprintf("INV%d", bill.SequenceNumber),
			StoreName:                    *bill.StoreName,
			StoreAddress:                 *bill.AddressName,
			Date:                         bill.EffectiveDate.Local().Format(time.DateTime),
			VATRegistrationNo:            *bill.VatRegistrationNumber,
			QRCodeData:                   qr,
			TotalDiscount:                "0.0",
			TotalTaxableAmt:              bill.TotalBeforeVat.Round(2).String(),
			TotalVAT:                     bill.TotalVat.Round(2).String(),
			TotalWithVAT:                 bill.Total.Round(2).String(),
			VATPercentage:                "15",
			CommercialRegistrationNumber: *bill.CommercialRegistrationNumber,
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
			log.Println(err)
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

func (h *handler) getProducts(billId int) []ProductDetails {
	query := `
	select product_id, price, quantity , articles.id, articles.articleNumber, articles.genericArticleDescription from bill_product 
	left join articles on articles.id = product_id where bill_id = ?
	`

	rows, err := h.DB.Query(query, billId)
	if err != nil {
		log.Panic(err)
	}
	var products []ProductDetails
	for rows.Next() {
		var product ProductDetails

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
