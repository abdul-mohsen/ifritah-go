package model

// PaginationRequest is the shared list-endpoint request body. Keyset
// (Limit/Cursor/Sort) and legacy offset keys (Page/PageSize) coexist;
// handlers pick based on whether Cursor is set.
type PaginationRequest struct {
	Query  *string `json:"query"`
	Limit  int32   `json:"limit"`
	Cursor string  `json:"cursor"`
	Sort   string  `json:"sort"`
	Dir    string  `json:"dir"`
	// Stock filter — product list only. "in" / "out" / "low" / "".
	Stock string `json:"stock"`

	// Typed-field filters (prefix match). Each is independent and
	// AND-ed with Query; handlers ignore the ones they don't support.
	Phone                  *string `json:"phone"`
	VatNumber              *string `json:"vat_number"`
	CommercialRegistration *string `json:"commercial_registration"`
	SequenceNumber         *string `json:"sequence_number"`
	Email                  *string `json:"email"`
	PartNumber             *string `json:"part_number"`
	Barcode                *string `json:"barcode"`

	// Legacy offset paging — accepted but ignored once Cursor is set.
	Page       int32 `json:"page"`
	PageSize   int32 `json:"page_size"`
	PageNumber int32 `json:"page_number"`
}

type PaginationResponse struct {
	Page  int32 `json:"page"`
	Total int32 `json:"total"`
	Data  any   `json:"data"`
}
