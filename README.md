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

## ZATCA onboarding state

After validation, `POST /api/v2/branch/:id/zatca/onboard` records a durable
branch attempt in `queued` state before publishing the existing onboarding
message. The response includes `onboard_state`, `onboard_step`, and the
monotonic `onboard_attempt`. Duplicate active or completed requests do not
create another attempt.

`GET /api/v2/branch/:id/zatca` exposes non-secret progress metadata:
`onboard_state` (`not_started`, `queued`, `compliance`, `production`, `done`,
or `failed`), `onboard_step`, retry count, error code/message, and queued,
started, and completed timestamps. The durable attempt history is stored in
`zatca_onboarding_attempt`; daemon-facing sqlc queries update it by branch and
attempt number. OTPs and ZATCA credentials are never included in this
metadata.

The database transition and attempt insert are atomic locally. The existing
publisher is called after commit; if publication fails, the same attempt is
marked `failed` with bounded error evidence. Cross-system atomicity requires an
outbox and remains outside this backend lane.

For existing databases, apply
`pkg/db/migrations/0009_zatca_onboarding_state.sql` after the T003 submission
state migration. Fresh Docker initialization includes it automatically.
