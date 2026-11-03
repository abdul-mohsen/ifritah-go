package model

import "github.com/shopspring/decimal"

// CreditNoteRequest credits part of a final bill. Amount is the total that the note credits, with VAT, and it must equal the total
// of Lines. Each note has its own bill_request_id: a retry repeats it, and a new note sends a new one.
type CreditNoteRequest struct {
	BillID        uint64           `json:"bill_id" binding:"required"`
	BranchID      uint32           `json:"branch_id" binding:"required"`
	PosID         string           `json:"pos_id"`
	BillRequestID string           `json:"bill_request_id"`
	Reason        string           `json:"reason"`
	Amount        decimal.Decimal  `json:"amount"`
	Lines         []CreditNoteLine `json:"lines"`
}

// CreditNoteLine credits one line of the bill. Its price and VAT rate must be the bill line's, and its quantity must not be more
// than the bill line has left after the earlier notes.
type CreditNoteLine struct {
	BillProductID uint64          `json:"bill_product_id"`
	Quantity      decimal.Decimal `json:"quantity"`
	Price         decimal.Decimal `json:"price"`
	VAT           decimal.Decimal `json:"vat"`
}

// DebitNoteRequest adds an extra charge to a final bill. Amount is the total of the charge, with VAT, and it must equal the total
// of Lines.
type DebitNoteRequest struct {
	BillID        uint64          `json:"bill_id" binding:"required"`
	BranchID      uint32          `json:"branch_id" binding:"required"`
	PosID         string          `json:"pos_id"`
	BillRequestID string          `json:"bill_request_id"`
	Reason        string          `json:"reason"`
	Amount        decimal.Decimal `json:"amount"`
	Lines         []DebitNoteLine `json:"lines"`
}

// DebitNoteLine is one extra charge of a debit note.
type DebitNoteLine struct {
	Name     string          `json:"name"`
	Quantity decimal.Decimal `json:"quantity"`
	Price    decimal.Decimal `json:"price"`
	VAT      decimal.Decimal `json:"vat"`
}
