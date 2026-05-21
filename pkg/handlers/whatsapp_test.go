package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	wamodels "github.com/abdul-mohsen/go-whatsapp/pkg/models"
	"github.com/gin-gonic/gin"
)

var (
	listSettingsPattern  = regexp.QuoteMeta("SELECT setting_key, COALESCE(value, '') AS value\nFROM settings\nORDER BY setting_key")
	getSettingPattern    = regexp.QuoteMeta("SELECT COALESCE(value, '') AS value\nFROM settings\nWHERE setting_key = ?\nLIMIT 1")
	upsertSettingPattern = regexp.QuoteMeta("INSERT INTO settings (setting_key, value, updated_by)\n" +
		"VALUES (?, ?, ?)\n" +
		"ON DUPLICATE KEY UPDATE value = VALUES(value), updated_by = VALUES(updated_by)")
	getBillPDFPattern      = regexp.QuoteMeta("SELECT b.id, b.effective_date, b.payment_due_date, b.state, b.discount, b.store_id, b.sequence_number, b.merchant_id, b.maintenance_cost, b.note, b.username, b.client_id, b.user_phone_number, b.qr_code, b.invoice_uuid, b.invoice_hash, b.branch_id, b.payment_method, b.deliver_date, b.invoice_xml_path, b.total_before_vat, b.total_vat, b.total, b.discount_amount, b.amount_paid, b.sequence_number_str,")
	getBillProductsPattern = regexp.QuoteMeta("select id, product_id, bill_id, vat, price, quantity, name, part_name, type, discount, total_before_discount, total_before_vat, vat_total, total_including_vat from bill_product where bill_id = ?")
)

type responseDetail struct {
	Detail    string `json:"detail"`
	MessageID string `json:"message_id"`
}

type fakeWhatsAppClient struct {
	uploadData     []byte
	uploadFilename string
	uploadMimeType string
	sendTo         string
	sendDocument   *wamodels.DocumentContent
}

func (f *fakeWhatsAppClient) UploadMediaBytes(_ context.Context, data []byte, filename, mimeType string) (*wamodels.MediaUploadResponse, error) {
	f.uploadData = data
	f.uploadFilename = filename
	f.uploadMimeType = mimeType
	return &wamodels.MediaUploadResponse{ID: "media-123"}, nil
}

func (f *fakeWhatsAppClient) SendDocument(_ context.Context, to string, doc *wamodels.DocumentContent) (*wamodels.MessageResponse, error) {
	f.sendTo = to
	f.sendDocument = doc
	return &wamodels.MessageResponse{Messages: []wamodels.MessageInfo{{ID: "wamid.123"}}}, nil
}

func newMockHandler(t *testing.T) (*handler, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	h := New(sqlDB, db.New(sqlDB), nil)
	return &h, mock, func() { _ = sqlDB.Close() }
}

func runHandlerRequest(t *testing.T, handlerFunc gin.HandlerFunc, method, path, body string, params gin.Params, withSession bool) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	if withSession {
		c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "admin"})
	}
	handlerFunc(c)
	return w
}

func decodeDetail(t *testing.T, w *httptest.ResponseRecorder) responseDetail {
	t.Helper()
	var response responseDetail
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	return response
}

func expectListSettings(mock sqlmock.Sqlmock, values map[string]string) {
	rows := sqlmock.NewRows([]string{"setting_key", "value"})
	for _, key := range []string{
		whatsappEnabledKey,
		whatsappBusinessAccountIDKey,
		whatsappPhoneNumberIDKey,
		whatsappAccessTokenKey,
		whatsappAPIVersionKey,
		whatsappInvoiceMessageKey,
	} {
		if value, ok := values[key]; ok {
			rows.AddRow(key, value)
		}
	}
	mock.ExpectQuery(listSettingsPattern).WillReturnRows(rows)
}

func enabledWhatsAppSettings(message string) map[string]string {
	return map[string]string{
		whatsappEnabledKey:           "true",
		whatsappBusinessAccountIDKey: "business-1",
		whatsappPhoneNumberIDKey:     "phone-1",
		whatsappAccessTokenKey:       "token-1",
		whatsappAPIVersionKey:        "v18.0",
		whatsappInvoiceMessageKey:    message,
	}
}

func expectUpsertSetting(mock sqlmock.Sqlmock, key, value string) {
	mock.ExpectExec(upsertSettingPattern).
		WithArgs(key, value, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectBillDetail(mock sqlmock.Sqlmock, id uint64, userPhone *string, userName *string, sequence uint64, total string) {
	columns := []string{
		"id", "effective_date", "payment_due_date", "state", "discount", "store_id", "sequence_number",
		"merchant_id", "maintenance_cost", "note", "username", "client_id", "user_phone_number", "qr_code",
		"invoice_uuid", "invoice_hash", "branch_id", "payment_method", "deliver_date", "invoice_xml_path",
		"total_before_vat", "total_vat", "total", "discount_amount", "amount_paid", "sequence_number_str",
		"company_name", "vat_registration_number", "commercial_registration_number", "address_name", "store_name",
		"credit_state", "credit_note", "credit_id",
	}
	sequenceString := "42"
	rows := sqlmock.NewRows(columns).AddRow(
		id,
		time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
		nil,
		int32(1),
		"0.00",
		int32(1),
		sequence,
		int32(7),
		"0.00",
		nil,
		userName,
		nil,
		userPhone,
		nil,
		sql.NullString{},
		nil,
		nil,
		int32(0),
		nil,
		nil,
		"100.00",
		"23.45",
		total,
		"0.00",
		"0.00",
		&sequenceString,
		"Ifritah",
		nil,
		nil,
		nil,
		"Main Store",
		nil,
		nil,
		nil,
	)
	mock.ExpectQuery(getBillPDFPattern).WithArgs(id).WillReturnRows(rows)
	mock.ExpectQuery(getBillProductsPattern).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{
		"id", "product_id", "bill_id", "vat", "price", "quantity", "name", "part_name", "type", "discount",
		"total_before_discount", "total_before_vat", "vat_total", "total_including_vat",
	}))
}

func assertExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetSettingsIncludesIntegrationsAndMasksToken(t *testing.T) {
	h, mock, cleanup := newMockHandler(t)
	defer cleanup()
	expectListSettings(mock, map[string]string{
		whatsappEnabledKey:           "false",
		whatsappBusinessAccountIDKey: "business-1",
		whatsappPhoneNumberIDKey:     "phone-1",
		whatsappAccessTokenKey:       "secret-token",
		whatsappAPIVersionKey:        "v18.0",
		whatsappInvoiceMessageKey:    "Invoice PDF is attached.",
	})

	w := runHandlerRequest(t, h.GetSettings, http.MethodGet, "/settings", "", nil, false)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var response struct {
		Data map[string]map[string]string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := response.Data["integrations"]; !ok {
		t.Fatalf("expected integrations category, got %#v", response.Data)
	}
	if got := response.Data["integrations"][whatsappAccessTokenKey]; got != maskedWhatsAppAccessToken {
		t.Fatalf("token got %q, want masked", got)
	}
	assertExpectations(t, mock)
}

func TestUpdateSettingsTokenPreserveAndReplace(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		expectGetToken bool
		expectToken    string
	}{
		{
			name:        "omitted token leaves it untouched",
			body:        `{"category":"integrations","settings":{"whatsapp_api_version":"v19.0"}}`,
			expectToken: "",
		},
		{
			name:           "blank token preserves old token",
			body:           `{"category":"integrations","settings":{"whatsapp_access_token":""}}`,
			expectGetToken: true,
			expectToken:    "old-token",
		},
		{
			name:           "masked token preserves old token",
			body:           `{"category":"integrations","settings":{"whatsapp_access_token":"********"}}`,
			expectGetToken: true,
			expectToken:    "old-token",
		},
		{
			name:        "new token replaces old token",
			body:        `{"category":"integrations","settings":{"whatsapp_access_token":"new-token"}}`,
			expectToken: "new-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, mock, cleanup := newMockHandler(t)
			defer cleanup()
			if tt.expectGetToken {
				mock.ExpectQuery(getSettingPattern).
					WithArgs(whatsappAccessTokenKey).
					WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("old-token"))
			}
			if strings.Contains(tt.body, whatsappAPIVersionKey) {
				expectUpsertSetting(mock, whatsappAPIVersionKey, "v19.0")
			} else {
				expectUpsertSetting(mock, whatsappAccessTokenKey, tt.expectToken)
			}

			w := runHandlerRequest(t, h.UpdateSettings, http.MethodPut, "/settings", tt.body, nil, true)
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			assertExpectations(t, mock)
		})
	}
}

func TestSendBillWhatsAppValidationBehavior(t *testing.T) {
	t.Run("invalid invoice id", func(t *testing.T) {
		h, mock, cleanup := newMockHandler(t)
		defer cleanup()
		w := runHandlerRequest(t, h.SendBillWhatsApp, http.MethodPost, "/bill/nope/whatsapp", "", gin.Params{{Key: "id", Value: "nope"}}, false)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		if got := decodeDetail(t, w).Detail; got != ErrInvalidInvoiceID {
			t.Fatalf("detail=%q", got)
		}
		assertExpectations(t, mock)
	})

	t.Run("disabled integration", func(t *testing.T) {
		h, mock, cleanup := newMockHandler(t)
		defer cleanup()
		expectListSettings(mock, map[string]string{whatsappEnabledKey: "false"})
		w := runHandlerRequest(t, h.SendBillWhatsApp, http.MethodPost, "/bill/10/whatsapp", "", gin.Params{{Key: "id", Value: "10"}}, false)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		if got := decodeDetail(t, w).Detail; got != ErrWhatsAppDisabled {
			t.Fatalf("detail=%q", got)
		}
		assertExpectations(t, mock)
	})

	t.Run("missing credentials", func(t *testing.T) {
		h, mock, cleanup := newMockHandler(t)
		defer cleanup()
		expectListSettings(mock, map[string]string{whatsappEnabledKey: "true"})
		w := runHandlerRequest(t, h.SendBillWhatsApp, http.MethodPost, "/bill/10/whatsapp", "", gin.Params{{Key: "id", Value: "10"}}, false)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		if got := decodeDetail(t, w).Detail; got != ErrWhatsAppConfig {
			t.Fatalf("detail=%q", got)
		}
		assertExpectations(t, mock)
	})

	t.Run("missing phone", func(t *testing.T) {
		h, mock, cleanup := newMockHandler(t)
		defer cleanup()
		expectListSettings(mock, enabledWhatsAppSettings(defaultWhatsAppInvoiceMessage))
		expectBillDetail(mock, 10, nil, nil, 42, "123.45")
		w := runHandlerRequest(t, h.SendBillWhatsApp, http.MethodPost, "/bill/10/whatsapp", "", gin.Params{{Key: "id", Value: "10"}}, false)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		if got := decodeDetail(t, w).Detail; got != ErrWhatsAppNoPhone {
			t.Fatalf("detail=%q", got)
		}
		assertExpectations(t, mock)
	})

	t.Run("invalid phone", func(t *testing.T) {
		h, mock, cleanup := newMockHandler(t)
		defer cleanup()
		phone := "05abc"
		name := "Walk In"
		expectListSettings(mock, enabledWhatsAppSettings(defaultWhatsAppInvoiceMessage))
		expectBillDetail(mock, 10, &phone, &name, 42, "123.45")
		w := runHandlerRequest(t, h.SendBillWhatsApp, http.MethodPost, "/bill/10/whatsapp", "", gin.Params{{Key: "id", Value: "10"}}, false)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		if got := decodeDetail(t, w).Detail; got != ErrWhatsAppBadPhone {
			t.Fatalf("detail=%q", got)
		}
		assertExpectations(t, mock)
	})
}

func TestSendBillWhatsAppHappyPathBehavior(t *testing.T) {
	h, mock, cleanup := newMockHandler(t)
	defer cleanup()

	oldClientFactory := newWhatsAppClient
	defer func() {
		newWhatsAppClient = oldClientFactory
	}()

	pdfBytes := []byte("%PDF fake invoice")
	h.renderBillPDF = func(bill model.Bill, products []model.BillProductResponse) ([]byte, error) {
		if bill.Id != 10 {
			t.Fatalf("render bill id=%d", bill.Id)
		}
		return pdfBytes, nil
	}

	fakeClient := &fakeWhatsAppClient{}
	newWhatsAppClient = func(settings whatsappSettings) (whatsappClient, error) {
		if settings.AccessToken != "token-1" {
			t.Fatalf("unexpected token %q", settings.AccessToken)
		}
		return fakeClient, nil
	}

	phone := "0551234567"
	name := "Walk In Customer"
	expectListSettings(mock, enabledWhatsAppSettings("Invoice {invoice_id}/{sequence_number} for {customer_name}: {total}"))
	expectBillDetail(mock, 10, &phone, &name, 42, "123.45")

	w := runHandlerRequest(t, h.SendBillWhatsApp, http.MethodPost, "/bill/10/whatsapp", "", gin.Params{{Key: "id", Value: "10"}}, false)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	response := decodeDetail(t, w)
	if response.Detail != "sent" || response.MessageID != "wamid.123" {
		t.Fatalf("response=%#v", response)
	}
	if !bytes.Equal(fakeClient.uploadData, pdfBytes) {
		t.Fatalf("uploaded pdf bytes mismatch")
	}
	if fakeClient.uploadMimeType != "application/pdf" {
		t.Fatalf("mime=%q", fakeClient.uploadMimeType)
	}
	if fakeClient.uploadFilename != "invoice-10.pdf" {
		t.Fatalf("filename=%q", fakeClient.uploadFilename)
	}
	if fakeClient.sendTo != "966551234567" {
		t.Fatalf("sendTo=%q", fakeClient.sendTo)
	}
	if fakeClient.sendDocument == nil || fakeClient.sendDocument.ID != "media-123" {
		t.Fatalf("send document=%#v", fakeClient.sendDocument)
	}
	if got := fakeClient.sendDocument.Caption; got != "Invoice 10/42 for Walk In Customer: 123.45" {
		t.Fatalf("caption=%q", got)
	}
	assertExpectations(t, mock)
}

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
