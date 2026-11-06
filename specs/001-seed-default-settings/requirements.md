# Requirements: Seed Fresh Tenant Defaults

## Problem

The Goose tenant baseline creates the `settings` table but does not restore the
legacy data migrations that seeded the WhatsApp integration settings and the
inventory markup setting. A newly created tenant therefore has missing settings
that the existing handlers and purchase-bill workflow expect.

## Goals

- Seed the required WhatsApp settings when a fresh tenant database is migrated.
- Seed `default_markup_percentage` with `20`.
- Make the seed safe to apply repeatedly.
- Preserve any setting values that already exist for an existing tenant.
- Cover the behavior with migration regression tests.

## In Scope

- Tenant Goose migration data for the seven required setting keys.
- Migration-level tests for fresh-database values, preservation, and replay.
- Documentation of the migration behavior.

## Out of Scope

- Changes to settings HTTP handlers, WhatsApp request handling, or markup
  calculation.
- Adding defaults for other settings not required by issue #124.
- Changes to tenant provisioning, deployment scripts, or unrelated migrations.

## User Stories

### US1 — Fresh tenants have usable defaults

As a newly provisioned tenant, I need the WhatsApp integration defaults and
standard markup percentage to exist after migrations so the settings UI and
purchase-bill workflow have deterministic initial values.

### US2 — Existing tenant configuration is preserved

As an existing tenant, I need migration replay to add only missing defaults and
never replace settings I have already configured.

## Functional Requirements

- **FR1:** A migration after the current Goose baseline must create these rows
  when absent:
  - `whatsapp_enabled = 'false'`
  - `whatsapp_business_account_id = ''`
  - `whatsapp_phone_number_id = ''`
  - `whatsapp_access_token = ''`
  - `whatsapp_api_version = 'v18.0'`
  - `whatsapp_invoice_message = 'Invoice PDF is attached.'`
  - `default_markup_percentage = '20'`
- **FR2:** The migration must be idempotent: applying the seed operation more
  than once must not create duplicate keys or change values.
- **FR3:** Existing values for any of the seven keys must remain unchanged.
- **FR4:** The migration must follow the repository's Goose naming, embedding,
  and up/down validation rules.
- **FR5:** Regression tests must verify fresh values, preservation of existing
  values, and replay behavior.

## Non-Functional Requirements

- The migration must work on the MySQL versions supported by the repository's
  migration workflow.
- The change must not alter runtime handler behavior or unrelated tenant data.
- The migration must be safe for a tenant account with privileges limited to
  its own database.

## Acceptance Criteria

- **AC1:** A fresh database migrated through the latest Goose version contains
  all seven keys with the exact values above.
- **AC2:** A pre-existing custom value is preserved while missing keys are added.
- **AC3:** Replaying the migration seed leaves one row per key and preserves all
  values.
- **AC4:** Focused migration tests, package tests, build, vet, and formatting
  checks pass.
- **AC5:** The change is committed locally and no remote/GitHub write occurs.

## Assumptions, Constraints, Risks, and Unknowns

### Assumptions

- `settings.setting_key` is uniquely constrained, as defined by the baseline.
- Goose migrations are applied in numeric order and the next version after
  `00010` is `00011`.
- The legacy migrations are the authoritative source for the required default
  values.

### Constraints

- Work only in `backend-b13-check-2`.
- Do not change unrelated tenant behavior or hand-edit generated `pkg/db/gen`
  files.

### Risks

- A destructive down migration could remove a tenant's configured setting.
  Rollback behavior must therefore preserve tenant data.
- A migration that seeds only fresh databases but ignores missing keys on an
  existing database would leave partially configured tenants inconsistent.

### Resolved Unknowns

- The settings owner is the tenant `settings` table; the baseline defines the
  table but no default rows.
- The authoritative WhatsApp values are the historical `0006_whatsapp_settings`
  migration, and the markup value is the historical
  `0007_default_markup_percentage` migration.
- Existing Goose migration tests use a real MySQL server through
  `GOOSE_PARITY_DSN`; the regression will follow that pattern.
