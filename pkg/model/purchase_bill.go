package model

import (
	"database/sql"
	"encoding/json"
)

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
type AddPurchaseBillRequest struct {
	StoreId                int             `json:"store_id" binding:"required"`
	State                  int32           `json:"state"`
	PaymentDueDate         *string         `json:"payment_due_date" `
	PaymentDate            *string         `json:"payment_date" `
	Discount               string          `json:"discount"`
	PaidAmount             string          `json:"paidAmount" `
	PaymentMethod          int8            `json:"payment_method"`
	Products               []Product       `json:"products" binding:"required,dive"`
	ManualProducts         []ManualProduct `json:"manual_products" binding:"required,dive"`
	SupplierId             int32           `json:"supplier_id" binding:"required"`
	SupplierSequenceNumber int32           `json:"supplier_sequence_number" binding:"required"`
}
