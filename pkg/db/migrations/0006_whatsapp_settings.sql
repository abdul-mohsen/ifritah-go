INSERT IGNORE INTO settings (setting_key, value, description) VALUES
  ('whatsapp_enabled', 'false', 'Enable sending invoice PDFs through WhatsApp Business'),
  ('whatsapp_business_account_id', '', 'WhatsApp Business Account ID'),
  ('whatsapp_phone_number_id', '', 'WhatsApp phone number ID'),
  ('whatsapp_access_token', '', 'WhatsApp Business API access token'),
  ('whatsapp_api_version', 'v18.0', 'WhatsApp Graph API version'),
  ('whatsapp_invoice_message', 'Invoice PDF is attached.', 'Default WhatsApp invoice PDF caption');