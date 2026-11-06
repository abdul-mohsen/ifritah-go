-- +goose Up
-- Restore the defaults that were previously owned by the legacy settings
-- migrations. INSERT IGNORE keeps this safe for existing tenants and replay.
INSERT IGNORE INTO settings (setting_key, value, description) VALUES
  ('whatsapp_enabled',             'false',                    'Enable WhatsApp Business invoice PDF sending'),
  ('whatsapp_business_account_id', '',                         'WhatsApp Business account ID'),
  ('whatsapp_phone_number_id',     '',                         'WhatsApp phone number ID'),
  ('whatsapp_access_token',        '',                         'WhatsApp Business API access token'),
  ('whatsapp_api_version',         'v18.0',                    'WhatsApp Graph API version'),
  ('whatsapp_invoice_message',     'Invoice PDF is attached.',  'Default WhatsApp invoice PDF caption'),
  ('default_markup_percentage',    '20',                       'نسبة الهامش الافتراضية على سعر التكلفة لتحديد سعر البيع');

-- +goose Down
-- Settings are tenant data. The migration cannot distinguish a row it inserted
-- from a pre-existing row with the same key, so rollback must not delete them.
SELECT 1;
