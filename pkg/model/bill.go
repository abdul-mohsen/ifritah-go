package model

import (
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
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
	Query     *string    `json:"query"`
}

type AddBillRequest struct {
	StoreId         int32           `json:"store_id" binding:"required"`
	State           int32           `json:"state"`
	PaymentDueDate  *string         `json:"payment_due_date" `
	PaymentDate     *string         `json:"payment_date" `
	Discount        decimal.Decimal `json:"discount" binding:"required"`
	PaidAmount      decimal.Decimal `json:"paidAmount" `
	MaintenanceCost decimal.Decimal `json:"maintenance_cost" binding:"required"`
	PaymentMethod   int8            `json:"payment_method"`
	UserName        *string         `json:"user_name"`
	UserPhoneNumber *string         `json:"user_phone_number"`
	Note            *string         `json:"note"`
	Products        []BillProduct   `json:"products" binding:"required,dive"`
	ManualProducts  []BillProduct   `json:"manual_products" binding:"required,dive"`
}

type BillProduct struct {
	ProductId *int32          `json:"product_id"`
	Name      string          `json:"name"`
	PartName  string          `json:"part_name"` // ← add this
	Price     decimal.Decimal `json:"price" binding:"required"`
	Quantity  decimal.Decimal `json:"quantity" binding:"required"`
}

type BillProductResponse struct {
	ProductId      *int32 `json:"product_id" binding:"required"`
	Name           string `json:"name"`
	Price          string `json:"price" binding:"required"`
	Discount       string `json:"discount"`
	Quantity       string `json:"quantity" binding:"required"`
	TotalBeforeVAT string `json:"total_before_vat"`
	TotalVAT       string `json:"total_vat"`
	Total          string `json:"total"`
	Type           int8
}

type AddProductToBillParams struct {
	ProductID *int32 `json:"product_id"`
	Price     string `json:"price"`
	Quantity  string `json:"quantity"`
	BillID    int32  `json:"bill_id"`
}

type TempManualProduct struct {
	PartName string `json:"part_name" binding:"required"`
	Price    string `json:"price" binding:"required"`
	Quantity string `json:"quantity" binding:"required"`
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
	Id                           int32                 `json:"id"`
	EffectiveDate                time.Time             `json:"effective_date"`
	PaymentDueDate               *time.Time            `json:"payment_due_date"`
	State                        int32                 `json:"state"`
	Discount                     string                `json:"discount"`
	VatRegistration              string                `json:"vat_registration"`
	Address                      string                `json:"address"`
	StoreName                    string                `json:"store_name"`
	CompanyName                  string                `json:"company_name"`
	SequenceNumber               int32                 `json:"sequence_number"`
	Type                         bool                  `json:"type"`
	StoreId                      int32                 `json:"store_id"`
	MerchantId                   int32                 `json:"merchant_id"`
	MaintenanceCost              string                `json:"maintenance_cost"`
	Note                         *string               `json:"note"`
	UserName                     *string               `json:"user_name"`
	UserPhoneNumber              *string               `json:"user_phone_number"`
	Products                     []BillProductResponse `json:"products"`
	ManualProducts               []BillProductResponse `json:"manual_products"`
	CreditState                  *int32                `json:"credit_state"`
	CreditNote                   *string               `json:"credit_note"`
	QRCode                       *string               `json:"qr_code"`
	TotalBeforeVAT               string                `json:"total_before_vat"`
	TotalVAT                     string                `json:"total_vat"`
	Total                        string                `json:"total"`
	CreditID                     *int32
	CommercialRegistrationNumber string
}
