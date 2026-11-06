# Design: Seed Fresh Tenant Defaults

## Architecture

The tenant database's Goose migration stream remains the single owner of
database defaults. Add one data migration after `00010`:

1. Goose applies `00011_seed_default_settings.sql`.
2. The migration inserts seven key/value rows only when their unique keys are
   absent.
3. Runtime handlers continue reading the existing `settings` table unchanged.

No generated SQLC model changes are needed because the schema and queries do
not change.

## Data Flow

```text
fresh or existing tenant database
        |
        v
Goose 00011 seed migration
        |
        +--> missing setting keys inserted with defaults
        +--> existing setting keys left unchanged
        |
        v
settings queries used by handlers
```

## Persistence and Idempotency

The migration uses the existing unique `settings.setting_key` constraint and an
insert-if-absent statement. The seven keys and exact values are:

| Key | Value | Consumer |
| --- | --- | --- |
| `whatsapp_enabled` | `false` | WhatsApp handler |
| `whatsapp_business_account_id` | empty | WhatsApp handler |
| `whatsapp_phone_number_id` | empty | WhatsApp handler |
| `whatsapp_access_token` | empty | WhatsApp handler |
| `whatsapp_api_version` | `v18.0` | WhatsApp handler |
| `whatsapp_invoice_message` | `Invoice PDF is attached.` | WhatsApp handler |
| `default_markup_percentage` | `20` | Purchase-bill inventory pricing |

The Down section is intentionally non-destructive. It contains a harmless SQL
statement and leaves settings rows in place, because deleting by key could
remove values that existed before this migration or were configured afterward.

## Failure Handling

- Any SQL error causes Goose to fail the migration, preserving the existing
  migration behavior.
- Duplicate keys are ignored by design.
- No application startup or request path is changed.

## Security and Performance

- The migration writes only fixed non-secret defaults; the access-token value
  is empty.
- It uses one multi-row statement and relies on an existing unique index, so
  the operation is bounded to seven rows.
- It does not expose or log credentials.

## Test Strategy

`pkg/db/goose/settings_test.go` will:

1. Create a fresh database through migration 10 and insert custom values for a
   subset of the keys.
2. Apply migration 11 and assert all seven values, including preservation of
   custom values.
3. Run a down/up replay of migration 11 and assert one row per key and the same
   values.

The test uses the existing `GOOSE_PARITY_DSN` integration-test convention and
skips when no MySQL server is configured.

## Rollout and Rollback

- Rollout is automatic wherever the normal Goose `up` command runs.
- Rollback marks the migration down without deleting tenant settings.
- Reapplying the migration is safe and restores any missing default keys.

## Requirement Traceability

| Requirement | Design element |
| --- | --- |
| FR1 / AC1 | Seven-row seed in `00011` and fresh database assertions |
| FR2 / AC3 | Unique-key insert-if-absent and down/up replay assertions |
| FR3 / AC2 | Existing custom values inserted before migration and asserted unchanged |
| FR4 | Five-digit Goose file with Up/Down sections |
| FR5 / AC4 | MySQL-backed migration regression plus repository validation |
