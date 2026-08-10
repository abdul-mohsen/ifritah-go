package handlers

import (
	"testing"

	"github.com/shopspring/decimal"
)

// TestValidateCashVoucherRequest_Amount verifies that the positive-amount
// validation rejects zero and negative values and accepts positive ones.
func TestValidateCashVoucherRequest_Amount(t *testing.T) {
	baseReq := func(amount decimal.Decimal) *cashVoucherCreateRequest {
		return &cashVoucherCreateRequest{
			VoucherType:   "disbursement",
			RecipientType: "supplier",
			RecipientName: "Test Supplier",
			PaymentMethod: "cash",
			Amount:        amount,
			StoreID:       1,
			BranchID:      1,
		}
	}

	cases := []struct {
		name    string
		amount  decimal.Decimal
		wantErr bool
	}{
		{"zero amount rejected", decimal.Zero, true},
		{"negative amount rejected", decimal.NewFromFloat(-1.00), true},
		{"very small negative rejected", decimal.NewFromFloat(-0.01), true},
		{"positive amount accepted", decimal.NewFromFloat(100.00), false},
		{"minimum positive accepted", decimal.NewFromFloat(0.01), false},
		{"large positive accepted", decimal.NewFromFloat(9999999.99), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCashVoucherRequest(baseReq(tc.amount))
			if tc.wantErr && err == nil {
				t.Errorf("amount=%s: expected validation error, got nil", tc.amount.String())
			}
			if !tc.wantErr && err != nil {
				t.Errorf("amount=%s: unexpected error: %v", tc.amount.String(), err)
			}
		})
	}
}

// TestValidateCashVoucherRequest_VoucherType verifies voucher_type validation.
func TestValidateCashVoucherRequest_VoucherType(t *testing.T) {
	baseReq := func(voucherType string) *cashVoucherCreateRequest {
		return &cashVoucherCreateRequest{
			VoucherType:   voucherType,
			RecipientType: "supplier",
			RecipientName: "Test",
			PaymentMethod: "cash",
			Amount:        decimal.NewFromFloat(100),
			StoreID:       1,
			BranchID:      1,
		}
	}

	cases := []struct {
		name        string
		voucherType string
		wantErr     bool
	}{
		{"disbursement valid", "disbursement", false},
		{"receipt valid", "receipt", false},
		{"cash_box valid", "cash_box", false},
		{"invalid type rejected", "unknown", true},
		{"empty type rejected", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCashVoucherRequest(baseReq(tc.voucherType))
			if tc.wantErr && err == nil {
				t.Errorf("voucher_type=%q: expected error, got nil", tc.voucherType)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("voucher_type=%q: unexpected error: %v", tc.voucherType, err)
			}
		})
	}
}

// TestValidateCashVoucherRequest_BankTransfer verifies bank fields are required
// when payment_method is bank_transfer.
func TestValidateCashVoucherRequest_BankTransfer(t *testing.T) {
	bankName := "Test Bank"
	bankAccount := "SA1234567890"

	cases := []struct {
		name        string
		bankName    *string
		bankAccount *string
		wantErr     bool
	}{
		{"both fields present", &bankName, &bankAccount, false},
		{"bank name missing", nil, &bankAccount, true},
		{"bank account missing", &bankName, nil, true},
		{"both fields missing", nil, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &cashVoucherCreateRequest{
				VoucherType:   "disbursement",
				RecipientType: "supplier",
				RecipientName: "Test",
				PaymentMethod: "bank_transfer",
				Amount:        decimal.NewFromFloat(100),
				StoreID:       1,
				BranchID:      1,
				BankName:      tc.bankName,
				BankAccount:   tc.bankAccount,
			}
			err := validateCashVoucherRequest(req)
			if tc.wantErr && err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
			}
		})
	}
}
