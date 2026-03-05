package model

type Product struct {
	Id          int32  `json:"id" binding:"required"`
	Price       string `json:"price" binding:"required"`
	PartName    string `json:"name" binding:"required"`
	CostPrice   string `json:"cost_price"`
	ShelfNumber string `json:"shelf_number"`
	Quantity    string `json:"quantity"`
}
