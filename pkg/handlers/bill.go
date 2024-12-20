package handlers

import (
	"database/sql"
	"fmt"
	"log"
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
		log.Panic(err)
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

func (h *handler) AddBill(c *gin.Context) {

}
