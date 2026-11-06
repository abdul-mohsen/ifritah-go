# Feature Specification: Seed Fresh Tenant Defaults

## User Scenarios & Testing

### User Story 1 — Fresh tenant defaults (Priority: P1)

When a new tenant is created and its database is migrated, the settings screen
and purchase-bill workflow can immediately read the required WhatsApp defaults
and a 20 percent default markup.

**Acceptance scenarios**

1. Given a fresh tenant database, when all migrations complete, then all seven
   required setting keys exist with their specified values.
2. Given a fresh tenant database, when the migration seed is replayed, then
   each key still has exactly one row and the values are unchanged.

### User Story 2 — Preserve existing configuration (Priority: P1)

When a tenant already has one or more of these settings, migrating forward
adds only missing defaults and keeps the configured values.

**Acceptance scenarios**

1. Given an existing custom WhatsApp API version and markup percentage, when the
   default-seeding migration runs, then both custom values remain unchanged.
2. Given a missing default key, when the migration runs, then that key is added
   without changing other settings.

## Functional Requirements

- **FR1:** Seed `whatsapp_enabled=false`.
- **FR2:** Seed empty values for the WhatsApp business account ID, phone number
  ID, and access token.
- **FR3:** Seed `whatsapp_api_version=v18.0`.
- **FR4:** Seed `whatsapp_invoice_message=Invoice PDF is attached.`.
- **FR5:** Seed `default_markup_percentage=20`.
- **FR6:** Never replace an existing value for any required key.
- **FR7:** Applying the seed repeatedly must not create duplicates or change
  values.

## Edge Cases

- A tenant may have only some of the seven keys; missing keys are added while
  present keys remain unchanged.
- A tenant may have a custom value equal to the default; it remains a single
  row.
- Rolling back the data migration must not delete tenant settings.

## Key Entities

- **Tenant setting:** A unique `setting_key` and its string `value`, optionally
  associated with the user who last updated it.

## Assumptions

- Tenant settings are stored in the `settings` table and the key is unique.
- Migration execution is ordered and the current highest migration is 00010.
- The exact defaults are those previously defined by the legacy WhatsApp and
  markup migrations.

## Success Criteria

- 100% of fresh-database migration runs expose all seven required keys with the
  exact expected values.
- 100% of replay tests preserve configured values and keep one row per key.
- No runtime handler source changes are required to consume the defaults.
