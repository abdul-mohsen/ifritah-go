-- 0006_whatsapp_settings.sql
-- WhatsApp Business invoice PDF integration settings. Idempotent.

INSERT IGNORE INTO settings (setting_key, value, description) VALUES
  ('whatsapp_enabled',             'false', 'Enable WhatsApp Business invoice PDF sending'),
  ('whatsapp_business_account_id', '',      'WhatsApp Business account ID'),
  ('whatsapp_phone_number_id',     '',      'WhatsApp phone number ID'),
  ('whatsapp_access_token',        '',      'WhatsApp Business API access token'),
  ('whatsapp_api_version',         'v18.0', 'WhatsApp Graph API version'),
  ('whatsapp_invoice_message',     'Invoice PDF is attached.', 'Default WhatsApp invoice PDF caption');
