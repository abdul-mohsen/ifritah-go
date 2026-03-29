package model

import (
	db "ifritah/web-service-gin/pkg/db/gen"
	"time"

	"github.com/shopspring/decimal"
)

type PurchaseBill struct {
	Id                     int32                    `json:"id"`
	EffectiveDate          time.Time                `json:"effective_date"`
	PaymentDueDate         *time.Time               `json:"payment_due_date"`
	State                  int32                    `json:"state"`
	Discount               int64                    `json:"discount"`
	SequenceNumber         int32                    `json:"sequence_number"`
	SupplierId             int32                    `json:"supplier_id"`
	SupplierSequenceNumber int32                    `json:"supplier_sequence_number"`
	StoreId                int32                    `json:"store_id"`
	MerchantId             int                      `json:"merchant_id"`
	Products               []db.PurchaseBillProduct `json:"products"`
	ManualProducts         []db.PurchaseBillProduct `json:"manual_products"`
	TotalBeforeVAT         string                   `json:"total_before_vat"`
	TotalVAT               string                   `json:"total_vat"`
	Total                  string                   `json:"total"`
	Attachments            []string                 `json:"attachments" binding:"required"`
	PDFLink                *string                  `json:"pdf_link"`
}
type AddPurchaseBillRequest struct {
	StoreId                int32                 `json:"store_id" binding:"required"`
	State                  int32                 `json:"state"`
	EffectiveDate          *string               `json:"effective_date" `
	PaymentDueDate         *string               `json:"payment_due_date" `
	PaymentDate            *string               `json:"payment_date" `
	Discount               string                `json:"discount"`
	PaidAmount             string                `json:"paidAmount" `
	Products               []PurchaseBillProduct `json:"products" binding:"required,dive"`
	ManualProducts         []PurchaseBillProduct `json:"manual_products" binding:"required,dive"`
	SupplierId             int32                 `json:"supplier_id" binding:"required"`
	SupplierSequenceNumber int32                 `json:"supplier_sequence_number" binding:"required"`
	Attachments            []string              `json:"attachments"`
	PDFLink                *string               `json:"pdf_link"`
	PaymentMethod          int32                 `json:"payment_method" binding:"required"`
	DeliverDate            *time.Time            `json:"deliver_date"`
}

type UploadFileResponse struct {
	FileKey      string `json:"file_key"`
	OriginalName string `json:"original_name"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mime_type"`
	DownloadURL  string `json:"download_url"`
}

type PurchaseBillProduct struct {
	ProductId   *int32          `json:"product_id"`
	Price       decimal.Decimal `json:"price" binding:"required"`
	Name        string          `json:"name" binding:"required"`
	CostPrice   decimal.Decimal `json:"cost_price" binding:"required"`
	ShelfNumber *string         `json:"shelf_number"`
	Quantity    decimal.Decimal `json:"quantity" binding:"required"`
}
