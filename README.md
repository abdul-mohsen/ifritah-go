# ifritah-go

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