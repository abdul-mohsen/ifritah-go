package model

type PartsProvider struct {
  Id int32 `json:"-"`
  Copany_id int32 `json:"-"`
  Name string
  Address string
  PhoneNumber string
  Number string
  VatNumber string
  IsDeleted bool
}
