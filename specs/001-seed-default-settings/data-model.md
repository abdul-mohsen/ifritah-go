# Data Model: Seed Fresh Tenant Defaults

## Tenant Setting

The existing tenant `settings` table stores one value per unique setting key.

| Field | Constraint | Use |
| --- | --- | --- |
| `id` | Unsigned integer primary key | Row identity |
| `setting_key` | Required, unique, up to 100 characters | Stable setting name |
| `value` | Nullable text | String setting value |
| `description` | Nullable text up to 255 characters | Human-readable setting description |
| `updated_by` | Nullable user foreign key | Last application updater |
| `updated_at` | Timestamp maintained by MySQL | Last update time |

## Seeded Values

The migration adds these keys only when absent:

- WhatsApp integration: `whatsapp_enabled`,
  `whatsapp_business_account_id`, `whatsapp_phone_number_id`,
  `whatsapp_access_token`, `whatsapp_api_version`, and
  `whatsapp_invoice_message`.
- Inventory: `default_markup_percentage`.

No new entity, column, relationship, or runtime query is introduced.
