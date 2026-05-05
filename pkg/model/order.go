package model

// OrderItemRequest is the body shape for a single line item in create/update order.
type OrderItemRequest struct {
	PartID    *uint64 `json:"part_id"`
	PartName  string  `json:"part_name" binding:"required"`
	Quantity  int     `json:"quantity" binding:"required,gte=1"`
	UnitPrice float64 `json:"unit_price" binding:"gte=0"`
}

// CreateOrderRequest is POST /api/v2/order body.
type CreateOrderRequest struct {
	SequenceNumber string             `json:"sequence_number" binding:"required"`
	ClientID       *uint32            `json:"client_id"`
	CustomerName   string             `json:"customer_name"`
	StoreID        *int32             `json:"store_id"`
	Status         string             `json:"status"`
	Note           string             `json:"note"`
	Items          []OrderItemRequest `json:"items"`
}

// UpdateOrderRequest is PUT /api/v2/order/:id body.
type UpdateOrderRequest struct {
	SequenceNumber string             `json:"sequence_number" binding:"required"`
	ClientID       *uint32            `json:"client_id"`
	CustomerName   string             `json:"customer_name"`
	StoreID        *int32             `json:"store_id"`
	Status         string             `json:"status"`
	Note           string             `json:"note"`
	Items          []OrderItemRequest `json:"items"`
}

// OrderListRequest is POST /api/v2/order/all body.
type OrderListRequest struct {
	PaginationRequest
}
