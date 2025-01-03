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
	Id             int
	EffectiveDate  sql.NullTime
	PaymentDueDate *sql.NullTime
	State          int
	SubTotal       float64
	Discount       float64
	Vat            float64
	SquenceNumber  int
	Type           bool
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

		if err := rows.Scan(&bill.Id, &bill.EffectiveDate, &bill.PaymentDueDate, &bill.State, &bill.SubTotal, &bill.Discount, &bill.Vat, &bill.SquenceNumber, &bill.Type); err != nil {
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
		PaymentMethod: 1,
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

	if _, err := h.DB.Exec(query, time.Now(), paymentDueDate, request.State, subTotal.Text('f', 10), discount.Text('f', 10), vatTotal.Text('f', 10), request.StoreId, squenceNumber, userSession.id,
		maintenanceCost.Text('f', 10), request.Note, request.UserName, nil, request.UserPhoneNumber); err != nil {
		log.Panic(err)
	}

	var id int
	h.DB.QueryRow(`select id from bill where store_id = ? and sequence_number = ?`, request.StoreId, squenceNumber).Scan(&id)

	h.addProductToBill(request.Products, id)

	c.Status(http.StatusOK)

}

func (h *handler) addProductToBill(products []Product, billId int) {

	query := `insert into bill_produect (product_id, price, quantity, bill_id) values (?, ?, ?, ?)`
	for _, product := range products {
		h.DB.Exec(query, product.Id, product.Price, product.Quantity, billId)
	}

}

func (h *handler) getNextSquenceNumber(id int64) int {

	query := `
	select max(sequence_number) from bill
	join store on store.id == bill.store.id
	join compnay where store.company_id == company.id
	join user where user.company_id == company.id and user.id == ?
	`
	var maxSequenceNumber int
	h.DB.QueryRow(query, id).Scan(&maxSequenceNumber)

	return maxSequenceNumber + 1
}
