package handlers

import (
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"testing"
)

func TestNormalizeWhatsAppPhone(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "international with separators", input: "+966 55-123-4567", want: "966551234567"},
		{name: "local leading zero", input: "0551234567", want: "966551234567"},
		{name: "local without zero", input: "551234567", want: "966551234567"},
		{name: "invalid characters", input: "05abc", wantErr: true},
		{name: "too short", input: "123", wantErr: true},
		{name: "missing", input: " ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeWhatsAppPhone(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectBillWhatsAppPhonePrefersClientPhone(t *testing.T) {
	clientPhone := "0551111111"
	walkInPhone := "0552222222"
	bill := model.Bill{
		Client:          &db.Client{Phone: &clientPhone},
		UserPhoneNumber: &walkInPhone,
	}

	if got := selectBillWhatsAppPhone(bill); got != clientPhone {
		t.Fatalf("got %q, want client phone %q", got, clientPhone)
	}
}

func TestSelectBillWhatsAppPhoneFallsBackToWalkInPhone(t *testing.T) {
	walkInPhone := "0552222222"
	bill := model.Bill{
		Client:          &db.Client{},
		UserPhoneNumber: &walkInPhone,
	}

	if got := selectBillWhatsAppPhone(bill); got != walkInPhone {
		t.Fatalf("got %q, want walk-in phone %q", got, walkInPhone)
	}
}

func TestFormatWhatsAppInvoiceMessage(t *testing.T) {
	sequenceNumber := uint64(42)
	companyName := "ACME Parts"
	bill := model.Bill{
		Id:             99,
		SequenceNumber: &sequenceNumber,
		Total:          "123.45",
		Client: &db.Client{
			Name:        "Client Name",
			CompanyName: &companyName,
		},
	}

	got := formatWhatsAppInvoiceMessage("Invoice {invoice_id}/{sequence_number} for {customer_name}: {total}", bill)
	want := "Invoice 99/42 for ACME Parts: 123.45"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSettingTokenMaskAndPreserve(t *testing.T) {
	if got := maskSettingValue("whatsapp_access_token", "secret-token"); got != maskedWhatsAppAccessToken {
		t.Fatalf("got %q, want masked token", got)
	}
	if got := maskSettingValue("whatsapp_access_token", " "); got != " " {
		t.Fatalf("blank token should stay blank, got %q", got)
	}
	if !shouldPreserveSettingValue("whatsapp_access_token", "") {
		t.Fatalf("blank token should preserve existing value")
	}
	if !shouldPreserveSettingValue("whatsapp_access_token", maskedWhatsAppAccessToken) {
		t.Fatalf("masked token should preserve existing value")
	}
	if shouldPreserveSettingValue("whatsapp_access_token", "new-token") {
		t.Fatalf("new token should be updated")
	}
}
