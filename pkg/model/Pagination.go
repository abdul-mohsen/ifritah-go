package model

type PaginationRequest struct {
	Query    *string `json:"query"`
	Page     int32   `json:"page"`
	PageSize int32   `json:"page_size"`
}

type PaginationResponse struct {
	Page  int32 `json:"page"`
	Total int32 `json:"total"`
	Data  any   `json:"data"`
}
