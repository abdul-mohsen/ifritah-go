# ifritah-go

## Database contracts

The backend owns the ZATCA submission persistence contract. The durable
submission, provider outcome, attempt, and artifact schema is defined in
`pkg/db/schema/schema.sql` and the replay-safe migration
`pkg/db/migrations/0008_zatca_submission_state.sql`. Monitor SQL lives in
`pkg/db/queries/zatca_monitor.sql`.

Regenerate the ignored sqlc output before building or testing:

```powershell
rtk sqlc generate
```

Generated files under `pkg/db/gen` are pipeline output and must not be edited
by hand.

## ZATCA onboarding validation

`POST /api/v2/branch/:id/zatca/onboard` is an authenticated administrator
operation. Before the existing ZATCA queue publication, the backend trims the
OTP and requires exactly six ASCII digits. It then validates the persisted
branch ZATCA configuration: all CSR, VAT, seller, and address fields in the
existing 12-field contract must be present, with the established VAT, CRN, and
CSR location formats.

Invalid requests return HTTP 400 with a `detail` message naming the field and
reason; no onboarding event is published. The configuration save endpoint may
still save drafts, but onboarding is blocked until the persisted configuration
is complete.
