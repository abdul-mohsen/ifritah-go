package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func TestLoadWhatsAppSettingsDefaultsDisabled(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer database.Close()

	mock.ExpectQuery("SELECT setting_key").
		WillReturnRows(sqlmock.NewRows([]string{"setting_key", "value"}))

	h := handler{DB: database}
	settings, err := h.loadWhatsAppSettings(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if settings.Enabled {
		t.Fatal("WhatsApp should be disabled by default")
	}
	if settings.APIVersion != "v18.0" {
		t.Fatalf("APIVersion = %q, want v18.0", settings.APIVersion)
	}
}

func TestLoadWhatsAppSettings(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer database.Close()

	rows := sqlmock.NewRows([]string{"setting_key", "value"}).
		AddRow(whatsAppEnabledKey, "true").
		AddRow(whatsAppBusinessAccountIDKey, "business-1").
		AddRow(whatsAppPhoneNumberIDKey, "phone-1").
		AddRow(whatsAppAccessTokenKey, "token-1").
		AddRow(whatsAppAPIVersionKey, "v19.0").
		AddRow(whatsAppInvoiceMessageKey, "Invoice {sequence_number}: {total}")

	mock.ExpectQuery("SELECT setting_key").WillReturnRows(rows)

	h := handler{DB: database}
	settings, err := h.loadWhatsAppSettings(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if !settings.Enabled || settings.BusinessAccountID != "business-1" || settings.PhoneNumberID != "phone-1" || settings.AccessToken != "token-1" || settings.APIVersion != "v19.0" {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestWhatsAppRecipientPhonePrefersClientPhone(t *testing.T) {
	clientPhone := "05 1234 5678"
	walkInPhone := "0599999999"

	got := whatsAppRecipientPhone(model.Bill{
		Client:          &db.Client{Phone: &clientPhone},
		UserPhoneNumber: &walkInPhone,
	})

	if got != "966512345678" {
		t.Fatalf("recipient phone = %q", got)
	}
}

func TestRenderWhatsAppInvoiceMessage(t *testing.T) {
	sequenceNumber := uint64(42)
	message := renderWhatsAppInvoiceMessage("Invoice {sequence_number} total {total}", model.Bill{
		Id:             7,
		SequenceNumber: &sequenceNumber,
		Total:          "150.00",
	})

	if message != "Invoice 42 total 150.00" {
		t.Fatalf("message = %q", message)
	}
}

func TestSendBillPDFWhatsAppUploadsAndSendsDocument(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var mu sync.Mutex
	uploaded := false
	sent := false

	graph := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fail := func(format string, args ...any) {
			t.Helper()
			t.Errorf(format, args...)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}

		if r.Header.Get("Authorization") != "Bearer token-1" {
			fail("Authorization = %q", r.Header.Get("Authorization"))
			return
		}

		switch r.URL.Path {
		case "/v18.0/phone-1/media":
			if err := r.ParseMultipartForm(4 << 20); err != nil {
				fail("parse multipart: %v", err)
				return
			}
			if got := r.FormValue("messaging_product"); got != "whatsapp" {
				fail("messaging_product = %q", got)
				return
			}
			if got := r.FormValue("type"); got != "application/pdf" {
				fail("type = %q", got)
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				fail("file: %v", err)
				return
			}
			defer file.Close()
			body, err := io.ReadAll(file)
			if err != nil {
				fail("read file: %v", err)
				return
			}
			if header.Filename != "invoice-123.pdf" || string(body) != "%PDF-1.7 test" {
				fail("uploaded file = %s %q", header.Filename, string(body))
				return
			}
			mu.Lock()
			uploaded = true
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"media-1"}`))
		case "/v18.0/phone-1/messages":
			var payload struct {
				To       string `json:"to"`
				Type     string `json:"type"`
				Document struct {
					ID       string `json:"id"`
					Filename string `json:"filename"`
					Caption  string `json:"caption"`
				} `json:"document"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				fail("decode message: %v", err)
				return
			}
			if payload.To != "966512345678" || payload.Type != "document" || payload.Document.ID != "media-1" {
				fail("message payload = %+v", payload)
				return
			}
			if payload.Document.Filename != "invoice-123.pdf" || payload.Document.Caption != "Invoice 123 total 115" {
				fail("document payload = %+v", payload.Document)
				return
			}
			mu.Lock()
			sent = true
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"messaging_product":"whatsapp","messages":[{"id":"wamid.1"}]}`))
		default:
			fail("unexpected Graph path: %s", r.URL.Path)
			return
		}
	}))
	defer graph.Close()

	oldBaseURL := whatsAppBaseURL
	whatsAppBaseURL = graph.URL
	defer func() { whatsAppBaseURL = oldBaseURL }()

	oldBuildPDF := buildWhatsAppBillPDFBytes
	buildWhatsAppBillPDFBytes = func(model.Bill, []model.BillProductResponse) ([]byte, error) {
		return []byte("%PDF-1.7 test"), nil
	}
	defer func() { buildWhatsAppBillPDFBytes = oldBuildPDF }()

	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer database.Close()

	settingsRows := sqlmock.NewRows([]string{"setting_key", "value"}).
		AddRow(whatsAppEnabledKey, "true").
		AddRow(whatsAppBusinessAccountIDKey, "business-1").
		AddRow(whatsAppPhoneNumberIDKey, "phone-1").
		AddRow(whatsAppAccessTokenKey, "token-1").
		AddRow(whatsAppAPIVersionKey, "v18.0").
		AddRow(whatsAppInvoiceMessageKey, "Invoice {invoice_id} total {total}")
	mock.ExpectQuery("SELECT setting_key").WillReturnRows(settingsRows)

	billRows := sqlmock.NewRows([]string{
		"id", "effective_date", "payment_due_date", "state", "discount", "store_id", "sequence_number", "merchant_id", "maintenance_cost", "note", "username", "client_id", "user_phone_number", "qr_code", "invoice_uuid", "invoice_hash", "branch_id", "payment_method", "deliver_date", "invoice_xml_path", "total_before_vat", "total_vat", "total", "discount_amount", "amount_paid", "sequence_number_str", "company_name", "vat_registration_number", "commercial_registration_number", "address_name", "store_name", "credit_state", "credit_note", "credit_id",
	}).AddRow(
		uint64(123), time.Now(), nil, int32(1), "0", int32(1), nil, int32(1), "0", nil, nil, nil, "05 1234 5678", nil, nil, nil, nil, int32(1), nil, nil, "100.00", "15.00", "115.00", "0", "0", nil, "ACME", nil, nil, nil, "Main Store", nil, nil, nil,
	)
	mock.ExpectQuery("SELECT b.id").WithArgs(uint64(123)).WillReturnRows(billRows)

	productRows := sqlmock.NewRows([]string{"id", "product_id", "bill_id", "vat", "price", "quantity", "name", "part_name", "type", "discount", "total_before_discount", "total_before_vat", "vat_total", "total_including_vat"})
	mock.ExpectQuery("select id, product_id").WithArgs(uint64(123)).WillReturnRows(productRows)

	h := handler{DB: database, queries: db.New(database)}
	router := gin.New()
	router.POST("/bill/:id/whatsapp", h.SendBillPDFWhatsApp)

	request := httptest.NewRequest(http.MethodPost, "/bill/123/whatsapp", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !uploaded || !sent {
		t.Fatalf("uploaded=%v sent=%v", uploaded, sent)
	}
}
