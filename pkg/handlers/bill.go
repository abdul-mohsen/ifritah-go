package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
)

type BillBase struct {
	Id             int           `json:"id"`
	EffectiveDate  sql.NullTime  `json:"effective_date"`
	PaymentDueDate *sql.NullTime `json:"payment_due_date"`
	State          int           `json:"state"`
	SubTotal       float64       `json:"subtotal"`
	Discount       float64       `json:"discount"`
	Vat            float64       `json:"vat"`
	SequenceNumber int           `json:"sequence_number"`
	Type           bool          `json:"type"`
}

type BillRequstFilter struct {
	StoreIds  []int      `json:"store_ids"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	Page      int        `json:"page_number"`
	PageSize  int        `json:"page_size"`
}

func (h *handler) GetBills(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var storeIds []int
	for _, value := range h.getStores(userSession) {
		storeIds = append(storeIds, value.Id)
	}

	request := BillRequstFilter{
		StoreIds: storeIds,
		Page:     0,
		PageSize: 10,
	}

	if err := c.BindJSON(&request); err != nil {
		log.Fatal(err)
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

	bills := h.getBaseBills(request.Page, request.PageSize)
	c.JSON(http.StatusOK, bills)
}

func (h *handler) getBaseBills(page int, pageSize int) []BillBase {

	query := ` Select * from(
	SELECT id, effective_date, payment_due_date, state, sub_total, discount, vat, sequence_number, TRUE as bill_type from bill 
	UNION
	SELECT id, effective_date, payment_due_date, state, sub_total, discount, vat, sequence_number, FALSE as bill_type from purchase_bill_register 
	) AS T ORDER BY effective_date DESC LIMIT ? OFFSET ?`

	rows, err := h.DB.Query(query, pageSize, page)

	if err != nil {
		log.Panic(err)
	}

	var bills []BillBase
	for rows.Next() {
		var bill BillBase

		if err := rows.Scan(&bill.Id, &bill.EffectiveDate, &bill.PaymentDueDate, &bill.State, &bill.SubTotal, &bill.Discount, &bill.Vat, &bill.SequenceNumber, &bill.Type); err != nil {
			log.Panic(err)
		}

		bills = append(bills, bill)
	}

	defer rows.Close()

	return bills
}

type AddBillRequest struct {
	StoreId         int       `json:"store_id" binding:"required"`
	State           int8      `json:"state"`
	PaymentDueDate  *string   `json:"payment_due_date" `
	PaymentDate     *string   `json:"payment_date" `
	Discount        string    `json:"discount" binding:"required"`
	PaidAmount      string    `json:"paidAmount" `
	MaintenanceCost string    `json:"maintenance_cost" binding:"required"`
	PaymentMethod   int8      `json:"payment_method"`
	UserName        *string   `json:"user_name"`
	UserPhoneNumber *string   `json:"user_phone_number"`
	Note            *string   `json:"note"`
	Products        []Product `json:"products" binding:"required,dive"`
}

type Product struct {
	Id       int    `json:"id" binding:"required"`
	Price    string `json:"price" binding:"required"`
	Quantity int64  `json:"quantity" binding:"required"`
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
	for _, product := range request.Products {
		price, success := stringToBigFloat(product.Price)
		if !success || product.Quantity <= 0 {
			c.Status(http.StatusBadRequest)
			log.Panic("invalid product")
		}
		quantity := big.NewFloat(float64(product.Quantity))
		cost := new(big.Float).Mul(price, quantity)
		subTotal = new(big.Float).Add(cost, subTotal)
	}
	totalWithOutVat := new(big.Float).Sub(new(big.Float).Add(subTotal, maintenanceCost), discount)
	vatTotal := new(big.Float).Mul(totalWithOutVat, big.NewFloat(.15))
	total := new(big.Float).Add(totalWithOutVat, vatTotal)

	if paidAmount.Cmp(total) == 1 {
		c.Status(http.StatusBadRequest)
		log.Panic("invalid paid ammount")
	}

	squenceNumber := h.getNextSquenceNumber(userSession.id)

	query := `
	insert into bill (effective_date, payment_due_date, state, sub_total, discount, vat, store_id, sequence_number, merchant_id, maintenance_cost, note, userName, buyer_id, user_phone_number)
	values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`
	res, err := h.DB.Exec(query, time.Now(), paymentDueDate, request.State, subTotal.Text('f', 10), discount.Text('f', 10), vatTotal.Text('f', 10), request.StoreId, squenceNumber, userSession.id,
		maintenanceCost.Text('f', 10), request.Note, request.UserName, nil, request.UserPhoneNumber)
	if err != nil {
		log.Panic(err)
	}

	id, err := res.LastInsertId()

	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	h.addProductToBill(request.Products, id)

	c.Status(http.StatusOK)

}

func (h *handler) addProductToBill(products []Product, billId int64) {

	query := `insert into bill_product (product_id, price, quantity, bill_id) values (?, ?, ?, ?)`
	for _, product := range products {
		h.DB.Exec(query, product.Id, product.Price, product.Quantity, billId)
	}
}

func (h *handler) getNextSquenceNumber(id int64) int {

	query := `
	select max(sequence_number) from bill
	join store on store.id = bill.store_id
	join company on store.company_id = company.id
	join user on user.company_id = company.id and user.id = ?
	`
	var maxSequenceNumber int
	if err := h.DB.QueryRow(query, id).Scan(&maxSequenceNumber); err != nil {
		log.Panic(err)
	}

	println("seq", maxSequenceNumber)

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
	Id              int              `json:"id"`
	EffectiveDate   sql.NullTime     `json:"effective_date"`
	PaymentDueDate  *sql.NullTime    `json:"payment_due_date"`
	State           int              `json:"state"`
	SubTotal        float64          `json:"subtotal"`
	Discount        float64          `json:"discount"`
	Vat             float64          `json:"vat"`
	SequenceNumber  int              `json:"sequence_number"`
	Type            bool             `json:"type"`
	StoreId         int              `json:"store_id"`
	MerchantId      int              `json:"merchant_id"`
	MaintenanceCost string           `json:"maintenance_cost"`
	Note            *string          `json:"note"`
	UserName        *string          `json:"user_name"`
	UserPhoneNumber *string          `json:"user_phone_number"`
	Products        []ProductDetails `json:"products"`
}

func (h *handler) GetBillDetail(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var id string = c.Param("id")

	query := `select effective_date, payment_due_date, b.state, sub_total, discount, vat, store_id, sequence_number, merchant_id, maintenance_cost,
	note, b.userName, user_phone_number from bill as b
	join store on store.id = b.store_id 
	join company on company.id = store.company_id
	join user on user.id= ? and company.id=user.company_id
	where b.id = ? limit 1`

	var bill Bill

	if err := h.DB.QueryRow(query, userSession.id, id).Scan(&bill.EffectiveDate,
		&bill.PaymentDueDate, &bill.State, &bill.SubTotal, &bill.Discount, &bill.Vat, &bill.StoreId, &bill.SequenceNumber, &bill.MerchantId, &bill.MaintenanceCost,
		&bill.Note, &bill.UserName, &bill.UserPhoneNumber); err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	bill.Products = h.getProducts(bill.Id)

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
	query := `DELETE FROM bill where id = ?`

	res, err := h.DB.Exec(query, id)
	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	affectedRows, err := res.RowsAffected()

	if err != nil {
		log.Fatal(err)
	}

	if affectedRows == 0 {
		c.Status(http.StatusBadRequest)
		log.Fatal("no recored has been deleted")
	}

	h.DB.Exec("DELETE bill_product where bill_id = ?")

	c.Status(http.StatusOK)
}

type AddPurchaseBillRequest struct {
	StoreId                int       `json:"store_id" binding:"required"`
	State                  int8      `json:"state"`
	PaymentDueDate         *string   `json:"payment_due_date" `
	PaymentDate            *string   `json:"payment_date" `
	Discount               string    `json:"discount" binding:"required"`
	PaidAmount             string    `json:"paidAmount" `
	PaymentMethod          int8      `json:"payment_method"`
	Products               []Product `json:"products" binding:"required,dive"`
	SupplierId             int       `json:"supplier_id" binding:"required"`
	SupplierSequenceNumber int       `json:"supplier_sequenve_number" binding:"required"`
}

func (h *handler) AddPurchaseBill(c *gin.Context) {

	request := AddPurchaseBillRequest{
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

	if !(success1 && success2) {
		c.Status(http.StatusBadRequest)
		log.Panic("big float are bad")
	}

	subTotal := zeroBigFloat()
	for _, product := range request.Products {
		price, success := stringToBigFloat(product.Price)
		if !success || product.Quantity <= 0 {
			c.Status(http.StatusBadRequest)
			log.Panic("invalid product")
		}
		quantity := big.NewFloat(float64(product.Quantity))
		cost := new(big.Float).Mul(price, quantity)
		subTotal = new(big.Float).Add(cost, subTotal)
	}

	totalWithOutVat := new(big.Float).Sub(subTotal, discount)
	vatTotal := new(big.Float).Mul(totalWithOutVat, big.NewFloat(.15))
	total := new(big.Float).Add(totalWithOutVat, vatTotal)

	if paidAmount.Cmp(total) == 1 {
		c.Status(http.StatusBadRequest)
		log.Panic("invalid paid ammount")
	}

	query := `
	insert into purchase_bill (effective_date, payment_due_date, state, sub_total, discount, vat, store_id, sequence_number, merchant_id, supplier_id, sequence_number)
	values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) 
	`
	res, err := h.DB.Exec(query, time.Now(), paymentDueDate, request.State, subTotal.Text('f', 10), discount.Text('f', 10), vatTotal.Text('f', 10),
		request.StoreId, userSession.id, request.SupplierSequenceNumber, request.SupplierSequenceNumber)
	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	id, err := res.LastInsertId()

	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	h.addProductToBillPurchase(request.Products, id)

	c.Status(http.StatusOK)

}

func (h *handler) addProductToBillPurchase(products []Product, billId int64) {

	query := `insert into purchase_bill_product (product_id, price, quantity, bill_id) values (?, ?, ?, ?)`
	for _, product := range products {
		h.DB.Exec(query, product.Id, product.Price, product.Quantity, billId)
	}
}

type PurchaseBill struct {
	Id             int              `json:"id"`
	EffectiveDate  sql.NullTime     `json:"effective_date"`
	PaymentDueDate *sql.NullTime    `json:"payment_due_date"`
	State          int              `json:"state"`
	SubTotal       float64          `json:"subtotal"`
	Discount       float64          `json:"discount"`
	Vat            float64          `json:"vat"`
	SequenceNumber int              `json:"sequence_number"`
	Type           bool             `json:"type"`
	StoreId        int              `json:"store_id"`
	MerchantId     int              `json:"merchant_id"`
	Products       []ProductDetails `json:"products"`
}

func (h *handler) GetPurchaseBillDetail(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var id string = c.Param("id")

	query := `select effective_date, payment_due_date, b.state, sub_total, discount, vat, store_id, sequence_number, merchant_id
	from purchase_bill as b
	join store on store.id = b.store_id 
	join company on company.id = store.company_id
	join user on user.id= ? and company.id=user.company_id
	where b.id = ? limit 1`
	var bill PurchaseBill

	if err := h.DB.QueryRow(query, userSession.id, id).Scan(&bill.EffectiveDate,
		&bill.PaymentDueDate, &bill.State, &bill.SubTotal, &bill.Discount, &bill.Vat, &bill.StoreId, &bill.SequenceNumber, &bill.MerchantId); err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	bill.Products = h.getProducts(bill.Id)

	c.JSON(http.StatusOK, bill)
}

func (h *handler) DeletePurchaseBillDetail(c *gin.Context) {

	var id string = c.Param("id")

	// TODO check if the user has right to delete and is the owner of the bill
	query := `DELETE FROM purchase_bill where id = ?`

	res, err := h.DB.Exec(query, id)
	if err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
	}

	affectedRows, err := res.RowsAffected()

	if err != nil {
		log.Fatal(err)
	}

	if affectedRows == 0 {
		c.Status(http.StatusBadRequest)
		log.Fatal("no recored has been deleted")
	}

	h.DB.Exec("DELETE purchase_bill_product where bill_id = ?")

	c.Status(http.StatusOK)
}
