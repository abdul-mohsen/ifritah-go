package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *handler) getPurchaseBills(page int, pageSize int, q string) []BillBase {

	var rows *sql.Rows
	var err error

	query := `
	SELECT id, effective_date, payment_due_date, state, sub_total, discount, vat, sequence_number, FALSE as bill_type, 0 as credit_state, total, total_vat, total_before_vat
	FROM purchase_bill_totals as purchase_bill
	WHERE state >= 0 ORDER BY id DESC LIMIT ? OFFSET ?`
	rows, err = h.DB.Query(query, pageSize, page*pageSize)

	if err != nil {
		log.Panic(err)
	}

	var bills []BillBase
	for rows.Next() {
		var bill BillBase

		if err := rows.Scan(&bill.Id, &bill.EffectiveDate, &bill.PaymentDueDate, &bill.State, &bill.SubTotal, &bill.Discount, &bill.Vat, &bill.SequenceNumber, &bill.Type, &bill.CreditState, &bill.Total, &bill.TotalVAT, &bill.TotalBeforeVAT); err != nil {
			log.Panic(err)
		}

		bills = append(bills, bill)
	}

	defer rows.Close()

	return bills
}

func (h *handler) addManualProductToPurchaseBill(products []ManualProduct, billId string) error {

	query := `insert into bill_manual_purchase_product  (part_name, price, quantity, bill_id) values (?, ?, ?, ?)`
	for _, product := range products {
		_, err := h.DB.Exec(query, product.PartName, product.Price, product.Quantity, billId)
		if err != nil {
			return err
		}
	}
	return nil
}

type AddPurchaseBillRequest struct {
	StoreId                int             `json:"store_id" binding:"required"`
	State                  int8            `json:"state"`
	PaymentDueDate         *string         `json:"payment_due_date" `
	PaymentDate            *string         `json:"payment_date" `
	Discount               string          `json:"discount"`
	PaidAmount             string          `json:"paidAmount" `
	PaymentMethod          int8            `json:"payment_method"`
	Products               []Product       `json:"products" binding:"required,dive"`
	ManualProducts         []ManualProduct `json:"manual_products" binding:"required,dive"`
	SupplierId             int             `json:"supplier_id" binding:"required"`
	SupplierSequenceNumber int             `json:"supplier_sequence_number" binding:"required"`
}

func (h *handler) UpdatePurchaseBill(c *gin.Context) {

	id := c.Param("id")
	request := AddPurchaseBillRequest{
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
		c.AbortWithStatus(http.StatusBadRequest)
		log.Panic("invalid paid ammount")
	}

	if total.Cmp(zeroBigFloat()) == 0 {
		c.AbortWithStatus(http.StatusBadRequest)
		log.Panic("invalid total")
	}

	query := `
	update purchase_bill set effective_date = ?, payment_due_date = ?, state = ?, sub_total = ?, discount = ?, vat = ?, store_id = ?, merchant_id = ?, supplier_id = ?, sequence_number = ? where id = ?
	`
	_, err := h.DB.Exec(query, time.Now(), paymentDueDate, request.State, subTotal.Text('f', 10), discount.Text('f', 10), vatTotal.Text('f', 10),
		request.StoreId, userSession.id, request.SupplierId, request.SupplierSequenceNumber, id)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	query = `DELETE FROM purchase_bill_product where bill_id = ?;`
	if _, err = h.DB.Exec(query, id); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}
	query = `DELETE FROM bill_manual_purchase_product where bill_id = ?;`
	if _, err = h.DB.Exec(query, id); err != nil {
		log.Panic(err)
		c.AbortWithError(http.StatusBadRequest, err)
	}
	log.Printf("Dropped old product ")
	if err := h.updateProductToBillPurchase(request.Products, id); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}
	if err := h.addManualProductToPurchaseBill(request.ManualProducts, id); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Panic(err)
	}

	c.Status(http.StatusOK)

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
		c.AbortWithStatus(http.StatusBadRequest)
		log.Panic("invalid paid ammount")
	}

	if total.Cmp(zeroBigFloat()) == 0 {
		c.AbortWithStatus(http.StatusBadRequest)
		log.Panic("invalid total")
	}

	query := `
	insert into purchase_bill (effective_date, payment_due_date, state, sub_total, discount, vat, store_id, merchant_id, supplier_id, sequence_number)
	values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := h.DB.Exec(query, time.Now(), paymentDueDate, request.State, subTotal.Text('f', 10), discount.Text('f', 10), vatTotal.Text('f', 10),
		request.StoreId, userSession.id, request.SupplierId, request.SupplierSequenceNumber)
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
	h.addManualProductToBillPurchase(request.ManualProducts, id)

	c.Status(http.StatusCreated)

}

func (h *handler) updateProductToBillPurchase(products []Product, billId string) error {

	query := `insert into purchase_bill_product  (product_id, price, quantity, bill_id) values (?, ?, ?, ?)`
	for _, product := range products {
		_, err := h.DB.Exec(query, product.Id, product.Price, product.Quantity, billId)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *handler) addProductToBillPurchase(products []Product, billId int64) error {

	query := `insert into bill_manual_purchase_product  (product_id, price, quantity, bill_id) values (?, ?, ?, ?)`
	for _, product := range products {
		_, err := h.DB.Exec(query, product.Id, product.Price, product.Quantity, billId)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *handler) addManualProductToBillPurchase(products []ManualProduct, billId int64) error {

	query := `insert into bill_manual_purchase_product  (part_name, price, quantity, bill_id) values (?, ?, ?, ?)`
	for _, product := range products {
		_, err := h.DB.Exec(query, product.PartName, product.Price, product.Quantity, billId)
		if err != nil {
			return err
		}
	}
	return nil
}

type PurchaseBill struct {
	Id             int             `json:"id"`
	EffectiveDate  sql.NullTime    `json:"effective_date"`
	PaymentDueDate *sql.NullTime   `json:"payment_due_date"`
	State          int             `json:"state"`
	SubTotal       float64         `json:"subtotal"`
	Discount       float64         `json:"discount"`
	Vat            float64         `json:"vat"`
	SequenceNumber int             `json:"sequence_number"`
	Type           bool            `json:"type"`
	StoreId        int             `json:"store_id"`
	MerchantId     int             `json:"merchant_id"`
	Products       json.RawMessage `json:"products"`
	ManualProducts json.RawMessage `json:"manual_products"`
}

func (h *handler) GetALLPurchaseBillDetail(c *gin.Context) {

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

	bill := h.getPurchaseBills(request.Page, request.PageSize, request.Query)

	c.JSON(http.StatusOK, bill)
}

func (h *handler) GetPurchaseBillDetail(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var id string = c.Param("id")

	query := `select effective_date, payment_due_date, b.state, sub_total, discount, vat, store_id, sequence_number, merchant_id,
			COALESCE(
				(SELECT JSON_ARRAYAGG(
					JSON_OBJECT(
						'product_id', p.product_id,
						'price', p.price,
						'quantity', p.quantity
					)
				)
				FROM purchase_bill_product p
				WHERE p.bill_id = b.id),
				JSON_ARRAY()) AS products,
			COALESCE(
				(SELECT JSON_ARRAYAGG(
					JSON_OBJECT(
						'part_name', m.part_name,
						'price', m.price,
						'quantity', m.quantity
					)
				)
				FROM bill_manual_purchase_product m
				WHERE m.bill_id = b.id),
				JSON_ARRAY()) AS manual_products
	from purchase_bill as b
	join store on store.id = b.store_id
	join company on company.id = store.company_id
	join user on user.id= ? and company.id=user.company_id
	where b.id = ? limit 1`
	var bill PurchaseBill

	if err := h.DB.QueryRow(query, userSession.id, id).Scan(&bill.EffectiveDate,
		&bill.PaymentDueDate, &bill.State, &bill.SubTotal, &bill.Discount, &bill.Vat, &bill.StoreId, &bill.SequenceNumber, &bill.MerchantId, &bill.Products, &bill.ManualProducts); err != nil {
		c.Status(http.StatusBadRequest)
		log.Panic(err)
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
