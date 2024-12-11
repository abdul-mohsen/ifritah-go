package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type BillBase struct {
	Id             int
	EffectiveDate  time.Time
	PaymentDueDate *time.Time
	State          int
	SubTotal       float64
	Discount       float64
	Vat            float64
	SquenceNumber  int
	Type           bool
}

type BillRequstFilter struct {
	StoreId   *[]int
	StartDate *time.Time
	EndDate   *time.Time
}

func (h *handler) GetBills(c *gin.Context) {

	userSession := GetSessionInfo(c)

	var id int
	if err := h.DB.QueryRow("SELECT company_id FROM user where id = ?;", userSession.id).Scan(&id); err != nil {
		log.Panic(err)
	}

	page := c.GetInt("page")
	pageSize := c.GetInt("pageSize")

	var request BillRequstFilter
	c.BindJSON(&request)

	fmt.Printf("%d %d", page, pageSize)
	fmt.Println(request)

	rows, err := h.getWithStoreId(page, pageSize)
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
	fmt.Println(bills)
	c.IndentedJSON(http.StatusOK, bills)
}

func (h *handler) getWithStoreId(page int, pageSize int) (*sql.Rows, error) {

	query := ` Select * from(
	SELECT id, effective_date, payment_due_date, state, sub_total, discount, vat, sequence_number, TRUE as bill_type from bill 
	UNION
	SELECT id, effective_date, payment_due_date, state, sub_total, discount, vat, sequence_number, FALSE as bill_type from purchase_bill_register 
	) AS T LIMIT ? OFFSET ?`
	println(query)

	return h.DB.Query(query, page, pageSize)
	// if storeId == nil {
	//
	// } else {
	// 	query := ` Select * from(
	// 	SELECT id, effective_date, payment_due_date, state, sub_total, discount, vat, sequence_number, TRUE as bill_type from bill
	// 	UNION
	// 	SELECT id, effective_date, payment_due_date, state, sub_total, discount, vat, sequence_number, FALSE as bill_type from purchase_bill_register
	// 	) AS T LIMIT ? OFFSET ?`
	//
	// 	return h.DB.Query(query, page, pageSize)
	// }
}
