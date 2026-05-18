package model

type UpdateSettingsRequest struct {
	Category string            `json:"category" binding:"required"`
	Settings map[string]string `json:"settings" binding:"required"`
}

type WhatsAppSendResponse struct {
	Detail    string `json:"detail"`
	MessageID string `json:"message_id,omitempty"`
}
