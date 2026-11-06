# Research: Seed Fresh Tenant Defaults

## Evidence

1. `pkg/db/goose/00001_baseline.sql` defines `settings` with a unique
   `setting_key`, nullable `value`, and nullable `updated_by`, but contains no
   setting seed rows. This establishes schema ownership and explains why a
   fresh Goose database is missing defaults.
2. Historical commit `0d7754d` added
   `pkg/db/migrations/0006_whatsapp_settings.sql` with the six WhatsApp keys
   and values required by the current WhatsApp handler:
   `false`, empty identifiers/token, `v18.0`, and
   `Invoice PDF is attached.`.
3. Historical commit `8c1fc8e` added
   `pkg/db/migrations/0007_default_markup_percentage.sql` with
   `default_markup_percentage = '20'`.
4. `pkg/handlers/whatsapp.go` reads the six WhatsApp keys and has the same
   fallback values for API version and invoice message. This confirms the
   migration should persist those values rather than changing runtime fallback
   behavior.
5. `pkg/handlers/setting.go` maps the WhatsApp keys to `integrations` and
   `default_markup_percentage` to `inventory`. No handler change is needed.
6. `pkg/db/goose/README.md` requires new tenant changes in the Goose folder,
   contiguous five-digit versions, and a Down section for every migration after
   the baseline.
7. Existing migration tests use `openFreshGooseDB` and
   `GOOSE_PARITY_DSN`, so a new integration regression belongs in
   `pkg/db/goose`.

## Decisions

### Decision 1: Add `00011_seed_default_settings.sql`

**Rationale:** `00010` is the current highest migration. A new numbered
migration preserves the one-way Goose ownership model and reaches both fresh
databases and existing databases that are migrated forward.

**Alternatives considered:**

- Put seed rows in the baseline: rejected because the baseline is already
  adopted at version 1 and changing it would not reach existing databases.
- Seed from application startup: rejected because provisioning correctness
  belongs to tenant migrations and startup behavior would be harder to replay
  deterministically.
- Reintroduce legacy migration folders: rejected because the repository
  explicitly documents Goose as the single migration path.

### Decision 2: Use an insert-if-absent operation

**Rationale:** `settings.setting_key` is unique, and the historical migrations
use `INSERT IGNORE`. This adds missing keys while preserving custom values and
is safe to replay.

**Alternatives considered:**

- `ON DUPLICATE KEY UPDATE`: rejected because it could overwrite tenant
  configuration unless every column update were carefully guarded.
- Application-level read-then-insert: rejected because it adds a race and
  duplicates migration ownership in Go code.

### Decision 3: Make Down non-destructive

**Rationale:** The migration cannot distinguish a row it inserted from a row
that already existed with the same key and value. Deleting keys on rollback
could remove tenant configuration. The Down section will contain a harmless
statement so Goose can round-trip the version without deleting tenant data;
the idempotent Up operation will remain safe when reapplied.

**Alternatives considered:**

- Delete all seven keys: rejected as destructive for existing tenants.
- Delete only rows with `updated_by IS NULL`: rejected because pre-existing or
  manually configured rows may also have a null updater.

## Remaining Uncertainty

No unresolved requirement or design question remains. A MySQL-backed test
environment may be unavailable locally; in that case the regression test will
skip locally and remain active in the repository's migration CI.
