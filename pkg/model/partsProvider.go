package model

type Supplier struct {
	Id          int32   `json:"id"`
	Copany_id   int32   `json:"-"`
	Name        string  `json:"name"`
	Address     string  `json:"address"`
	PhoneNumber string  `json:"phone_number"`
	Number      string  `json:"number"`
	VatNumber   string  `json:"vat_number"`
	IsDeleted   bool    `json:"-"`
	BankAccount *string `json:"bank_account"`
}
