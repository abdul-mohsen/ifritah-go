package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	waclient "github.com/abdul-mohsen/go-whatsapp/pkg/client"
	waconfig "github.com/abdul-mohsen/go-whatsapp/pkg/config"
	wamodels "github.com/abdul-mohsen/go-whatsapp/pkg/models"
)

const (
	whatsappEnabledKey           = "whatsapp_enabled"
	whatsappBusinessAccountIDKey = "whatsapp_business_account_id"
	whatsappPhoneNumberIDKey     = "whatsapp_phone_number_id"
	whatsappAccessTokenKey       = "whatsapp_access_token"
	whatsappAPIVersionKey        = "whatsapp_api_version"
	whatsappInvoiceMessageKey    = "whatsapp_invoice_message"

	defaultWhatsAppAPIVersion     = "v18.0"
	defaultWhatsAppInvoiceMessage = "Invoice PDF is attached."
)

type whatsappSettings struct {
	Enabled           bool
	BusinessAccountID string
	PhoneNumberID     string
	AccessToken       string
	APIVersion        string
	InvoiceMessage    string
}

type whatsappClient interface {
	UploadMediaBytes(ctx context.Context, data []byte, filename, mimeType string) (*wamodels.MediaUploadResponse, error)
	SendDocument(ctx context.Context, to string, doc *wamodels.DocumentContent) (*wamodels.MessageResponse, error)
}

var newWhatsAppClient = func(settings whatsappSettings) (whatsappClient, error) {
	cfg := waconfig.DefaultConfig()
	cfg.BusinessAccountID = settings.BusinessAccountID
	cfg.PhoneNumberID = settings.PhoneNumberID
	cfg.AccessToken = settings.AccessToken
	cfg.APIVersion = settings.APIVersion
	return waclient.New(cfg)
}

func defaultWhatsAppSettings() whatsappSettings {
	return whatsappSettings{
		APIVersion:     defaultWhatsAppAPIVersion,
		InvoiceMessage: defaultWhatsAppInvoiceMessage,
	}
}

func (h *handler) SendBillWhatsApp(c *gin.Context) {
	billID, err := parseBillID(c.Param("id"))
	if err != nil {
		log.Printf("SendBillWhatsApp: %v", err)
		c.JSON(http.StatusBadRequest, model.WhatsAppSendResponse{Detail: ErrInvalidInvoiceID})
		return
	}

	settings, err := h.loadWhatsAppSettings(c.Request.Context())
	if err != nil {
		log.Printf("SendBillWhatsApp: %v", err)
		c.JSON(http.StatusInternalServerError, model.WhatsAppSendResponse{Detail: ErrDatabase})
		return
	}
	if !settings.Enabled {
		c.JSON(http.StatusBadRequest, model.WhatsAppSendResponse{Detail: ErrWhatsAppDisabled})
		return
	}
	if !settings.hasCredentials() {
		c.JSON(http.StatusBadRequest, model.WhatsAppSendResponse{Detail: ErrWhatsAppConfig})
		return
	}

	bill, products, err := h.getBillDetailByID(c.Request.Context(), billID)
	if err != nil {
		log.Printf("SendBillWhatsApp: %v", err)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, model.WhatsAppSendResponse{Detail: ErrInvoiceNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, model.WhatsAppSendResponse{Detail: ErrDatabase})
		return
	}

	recipient, err := normalizeWhatsAppPhone(selectBillWhatsAppPhone(bill))
	if err != nil {
		log.Printf("SendBillWhatsApp: %v", err)
		if errors.Is(err, errMissingWhatsAppPhone) {
			c.JSON(http.StatusBadRequest, model.WhatsAppSendResponse{Detail: ErrWhatsAppNoPhone})
			return
		}
		c.JSON(http.StatusBadRequest, model.WhatsAppSendResponse{Detail: ErrWhatsAppBadPhone})
		return
	}

	pdfBytes, err := h.renderBillPDFBytes(bill, products)
	if err != nil {
		log.Printf("SendBillWhatsApp: %v", err)
		c.JSON(http.StatusInternalServerError, model.WhatsAppSendResponse{Detail: ErrGenerateInvoicePDF})
		return
	}

	waClient, err := newWhatsAppClient(settings)
	if err != nil {
		log.Printf("SendBillWhatsApp: %v", err)
		c.JSON(http.StatusBadRequest, model.WhatsAppSendResponse{Detail: ErrWhatsAppConfig})
		return
	}

	filename := fmt.Sprintf("invoice-%d.pdf", bill.Id)
	mediaResp, err := waClient.UploadMediaBytes(c.Request.Context(), pdfBytes, filename, "application/pdf")
	if err != nil {
		log.Printf("SendBillWhatsApp: %v", err)
		c.JSON(http.StatusBadGateway, model.WhatsAppSendResponse{Detail: ErrSendWhatsAppMessage})
		return
	}

	messageResp, err := waClient.SendDocument(c.Request.Context(), recipient, &wamodels.DocumentContent{
		ID:       mediaResp.ID,
		Caption:  formatWhatsAppInvoiceMessage(settings.InvoiceMessage, bill),
		Filename: filename,
	})
	if err != nil {
		log.Printf("SendBillWhatsApp: %v", err)
		c.JSON(http.StatusBadGateway, model.WhatsAppSendResponse{Detail: ErrSendWhatsAppMessage})
		return
	}

	c.JSON(http.StatusOK, model.WhatsAppSendResponse{
		Detail:    "sent",
		MessageID: firstWhatsAppMessageID(messageResp),
	})
}

func (h *handler) loadWhatsAppSettings(ctx context.Context) (whatsappSettings, error) {
	settings := defaultWhatsAppSettings()
	rows, err := h.queries.ListSettings(ctx)
	if err != nil {
		return settings, err
	}

	for _, row := range rows {
		value := strings.TrimSpace(row.Value)
		switch row.SettingKey {
		case whatsappEnabledKey:
			settings.Enabled = strings.EqualFold(value, "true") || value == "1"
		case whatsappBusinessAccountIDKey:
			settings.BusinessAccountID = value
		case whatsappPhoneNumberIDKey:
			settings.PhoneNumberID = value
		case whatsappAccessTokenKey:
			settings.AccessToken = value
		case whatsappAPIVersionKey:
			if value != "" {
				settings.APIVersion = value
			}
		case whatsappInvoiceMessageKey:
			if value != "" {
				settings.InvoiceMessage = value
			}
		}
	}

	return settings, nil
}

func (s whatsappSettings) hasCredentials() bool {
	return strings.TrimSpace(s.BusinessAccountID) != "" &&
		strings.TrimSpace(s.PhoneNumberID) != "" &&
		strings.TrimSpace(s.AccessToken) != "" &&
		strings.TrimSpace(s.APIVersion) != ""
}

func selectBillWhatsAppPhone(bill model.Bill) string {
	if bill.Client != nil && bill.Client.Phone != nil && strings.TrimSpace(*bill.Client.Phone) != "" {
		return *bill.Client.Phone
	}
	if bill.UserPhoneNumber != nil {
		return *bill.UserPhoneNumber
	}
	return ""
}

var (
	errMissingWhatsAppPhone = errors.New("missing whatsapp phone")
	errInvalidWhatsAppPhone = errors.New("invalid whatsapp phone")
)

func normalizeWhatsAppPhone(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return "", errMissingWhatsAppPhone
	}

	var digits strings.Builder
	for i, r := range phone {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case i == 0 && r == '+':
			continue
		case r == ' ' || r == '-' || r == '(' || r == ')':
			continue
		default:
			return "", errInvalidWhatsAppPhone
		}
	}

	normalized := digits.String()
	if strings.HasPrefix(normalized, "05") {
		normalized = "966" + normalized[1:]
	} else if strings.HasPrefix(normalized, "5") {
		normalized = "966" + normalized
	}
	if len(normalized) < 8 {
		return "", errInvalidWhatsAppPhone
	}
	return normalized, nil
}

func formatWhatsAppInvoiceMessage(template string, bill model.Bill) string {
	if strings.TrimSpace(template) == "" {
		template = defaultWhatsAppInvoiceMessage
	}

	sequenceNumber := ""
	if bill.SequenceNumber != nil {
		sequenceNumber = fmt.Sprint(*bill.SequenceNumber)
	}

	customerName := ""
	if bill.Client != nil {
		customerName = bill.Client.Name
		if bill.Client.CompanyName != nil && strings.TrimSpace(*bill.Client.CompanyName) != "" {
			customerName = *bill.Client.CompanyName
		}
	} else if bill.UserName != nil {
		customerName = *bill.UserName
	}

	return strings.NewReplacer(
		"{invoice_id}", fmt.Sprint(bill.Id),
		"{sequence_number}", sequenceNumber,
		"{customer_name}", customerName,
		"{total}", bill.Total,
	).Replace(template)
}

func firstWhatsAppMessageID(resp *wamodels.MessageResponse) string {
	if resp == nil || len(resp.Messages) == 0 {
		return ""
	}
	return resp.Messages[0].ID
}
