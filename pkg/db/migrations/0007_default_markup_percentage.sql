-- 0007_default_markup_percentage.sql
-- Owner-configured default markup % applied to cost price when a
-- non-admin/manager user creates a brand-new store product from a
-- purchase-bill line (see resolveNewProductSellingPrice). Idempotent.

INSERT IGNORE INTO settings (setting_key, value, description) VALUES
  ('default_markup_percentage', '20', 'نسبة الهامش الافتراضية على سعر التكلفة لتحديد سعر البيع');
