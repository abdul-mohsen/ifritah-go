package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ifritah/web-service-gin/pkg/model"

	waclient "github.com/abdul-mohsen/go-whatsapp/pkg/client"
	waconfig "github.com/abdul-mohsen/go-whatsapp/pkg/config"
	wamodels "github.com/abdul-mohsen/go-whatsapp/pkg/models"
	"github.com/gin-gonic/gin"
)

const (
	whatsAppEnabledKey           = "whatsapp_enabled"
	whatsAppBusinessAccountIDKey = "whatsapp_business_account_id"
	whatsAppPhoneNumberIDKey     = "whatsapp_phone_number_id"
	whatsAppAccessTokenKey       = "whatsapp_access_token"
	whatsAppAPIVersionKey        = "whatsapp_api_version"
	whatsAppInvoiceMessageKey    = "whatsapp_invoice_message"
)

var (
	whatsAppBaseURL           = "https://graph.facebook.com"
	buildWhatsAppBillPDFBytes = buildBillPDFBytes
)

type whatsAppSettings struct {
	Enabled           bool
	BusinessAccountID string
	PhoneNumberID     string
	AccessToken       string
	APIVersion        string
	InvoiceMessage    string
}

func (h *handler) SendBillPDFWhatsApp(c *gin.Context) {
	rawID := c.Param("id")
	if _, err := strconv.ParseUint(rawID, 10, 64); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid bill id"})
		return
	}

	settings, err := h.loadWhatsAppSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to load WhatsApp settings"})
		return
	}
	if !settings.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "WhatsApp invoice sending is disabled"})
		return
	}
	if strings.TrimSpace(settings.BusinessAccountID) == "" || strings.TrimSpace(settings.PhoneNumberID) == "" || strings.TrimSpace(settings.AccessToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "WhatsApp Business credentials are incomplete"})
		return
	}

	bill, products := h.getBillDetail(c)
	if bill.Id == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "bill not found"})
		return
	}

	recipient := whatsAppRecipientPhone(bill)
	if recipient == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "customer phone number is missing"})
		return
	}

	pdfBytes, err := buildWhatsAppBillPDFBytes(bill, products)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to build invoice PDF"})
		return
	}

	client, err := newWhatsAppClient(settings)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	filename := fmt.Sprintf("invoice-%s.pdf", rawID)
	media, err := client.UploadMediaBytes(ctx, pdfBytes, filename, "application/pdf")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": "failed to upload invoice PDF to WhatsApp"})
		return
	}

	caption := renderWhatsAppInvoiceMessage(settings.InvoiceMessage, bill)
	message, err := client.SendDocument(ctx, recipient, &wamodels.DocumentContent{
		ID:       media.ID,
		Filename: filename,
		Caption:  caption,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": "failed to send invoice PDF through WhatsApp"})
		return
	}

	messageID := ""
	if len(message.Messages) > 0 {
		messageID = message.Messages[0].ID
	}
	c.JSON(http.StatusOK, gin.H{"detail": "sent", "message_id": messageID})
}

func (h *handler) loadWhatsAppSettings(ctx context.Context) (whatsAppSettings, error) {
	settings := whatsAppSettings{APIVersion: "v18.0"}
	rows, err := h.DB.QueryContext(ctx, `
		SELECT setting_key, COALESCE(value, '')
		FROM settings
		WHERE setting_key IN (?, ?, ?, ?, ?, ?)`,
		whatsAppEnabledKey,
		whatsAppBusinessAccountIDKey,
		whatsAppPhoneNumberIDKey,
		whatsAppAccessTokenKey,
		whatsAppAPIVersionKey,
		whatsAppInvoiceMessageKey,
	)
	if err != nil {
		return settings, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return settings, err
		}
		switch key {
		case whatsAppEnabledKey:
			settings.Enabled = strings.EqualFold(strings.TrimSpace(value), "true")
		case whatsAppBusinessAccountIDKey:
			settings.BusinessAccountID = strings.TrimSpace(value)
		case whatsAppPhoneNumberIDKey:
			settings.PhoneNumberID = strings.TrimSpace(value)
		case whatsAppAccessTokenKey:
			settings.AccessToken = strings.TrimSpace(value)
		case whatsAppAPIVersionKey:
			if strings.TrimSpace(value) != "" {
				settings.APIVersion = strings.TrimSpace(value)
			}
		case whatsAppInvoiceMessageKey:
			settings.InvoiceMessage = value
		}
	}
	return settings, rows.Err()
}

func newWhatsAppClient(settings whatsAppSettings) (*waclient.Client, error) {
	return waclient.New(&waconfig.Config{
		BusinessAccountID: settings.BusinessAccountID,
		PhoneNumberID:     settings.PhoneNumberID,
		AccessToken:       settings.AccessToken,
		APIVersion:        settings.APIVersion,
		BaseURL:           whatsAppBaseURL,
	})
}

func whatsAppRecipientPhone(bill model.Bill) string {
	if bill.Client != nil && bill.Client.Phone != nil {
		if phone := normalizeWhatsAppPhone(*bill.Client.Phone); phone != "" {
			return phone
		}
	}
	if bill.UserPhoneNumber != nil {
		return normalizeWhatsAppPhone(*bill.UserPhoneNumber)
	}
	return ""
}

func normalizeWhatsAppPhone(phone string) string {
	digits := strings.Builder{}
	for _, r := range phone {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}
	cleaned := digits.String()
	cleaned = strings.TrimPrefix(cleaned, "00")
	if strings.HasPrefix(cleaned, "0") && len(cleaned) == 10 {
		return "966" + cleaned[1:]
	}
	return cleaned
}

func renderWhatsAppInvoiceMessage(template string, bill model.Bill) string {
	message := strings.TrimSpace(template)
	if message == "" {
		message = "Invoice PDF is attached."
	}

	sequenceNumber := fmt.Sprint(bill.Id)
	if bill.SequenceNumber != nil {
		sequenceNumber = fmt.Sprint(*bill.SequenceNumber)
	}

	replacements := map[string]string{
		"{invoice_id}":      fmt.Sprint(bill.Id),
		"{sequence_number}": sequenceNumber,
		"{total}":           bill.Total,
	}
	for old, newValue := range replacements {
		message = strings.ReplaceAll(message, old, newValue)
	}
	return message
}
