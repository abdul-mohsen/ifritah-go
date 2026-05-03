package model

// PaginationRequest is the shared list-endpoint request body. Both
// keyset (Limit/Cursor/Sort) and the legacy offset keys (Page/PageSize)
// are accepted; the handler picks one based on whether Cursor is set.
//
// Renaming or removing JSON keys here breaks the wire contract — keep
// all keys here in lockstep with FE helpers/cursor.go and SEARCH_API.md.
type PaginationRequest struct {
	Query *string `json:"query"`
	// Keyset pagination (preferred).
	Limit  int32  `json:"limit"`
	Cursor string `json:"cursor"`
	Sort   string `json:"sort"`
	// Legacy offset paging — accepted but ignored once Cursor is set.
	Page     int32 `json:"page"`
	PageSize int32 `json:"page_size"`
	// Legacy alias for Page used by some FE callers (page_number).
	PageNumber int32 `json:"page_number"`
}

type PaginationResponse struct {
	Page  int32 `json:"page"`
	Total int32 `json:"total"`
	Data  any   `json:"data"`
}
