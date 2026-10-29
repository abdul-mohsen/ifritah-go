# T026 Research

## Evidence inspected

| Area | Evidence | Finding |
|---|---|---|
| Onboarding boundary | `pkg/handlers/branch.go`, `OnboardBranchZatca` | The handler trims only for length, accepts non-digits, and publishes `req.OTP` rather than the trimmed value. It does not inspect branch configuration before publishing. |
| Publisher contract | `pkg/handlers/pub.go` | `OnboadBranch(branchID, otp)` is the existing compatibility method; changing queue/outbox/message shape is unnecessary. |
| Existing config query | `pkg/db/queries/branch.sql`, generated `GetZatcaBranchConfig` | The existing sqlc query joins `branch_zatca_config` to `branches` and returns the 12 writable contract fields plus status metadata. A missing row is `sql.ErrNoRows`. |
| Config persistence contract | `UpdateZatcaBranchConfigParams` and frontend `zatcaFieldNames` | The writable contract is `csr_org_identifier`, `csr_org_unit`, `csr_org_name`, `csr_country`, `csr_location`, `business_category`, `seller_vat`, `seller_crn`, `street`, `building`, `district`, and `postal_code`. Config saves intentionally support drafts. |
| Worker consumer | `zatca-fix-t027-zatca/processor/onboard.go` and `config/taxpayer.go` | The worker treats branch config values as authoritative where present, requires the listed CSR/VAT/address inputs for taxpayer construction, and has no safe fallback for `csr_location`. Company/store fields such as Arabic name and city are outside this backend config row and are handled by the worker/frontend contract in later lanes. |
| User-facing validation | `zatca-fix-t028-frontend/templates/settings.html` | The existing UI marks all 12 config fields required before connect, validates VAT as `3` + 13 digits + `3`, CRN as `7` + 9 digits, and location as four uppercase ASCII letters plus four digits. |
| Route boundaries | `main.go` | ZATCA config writes and onboarding remain behind `JWTVerifyMiddleware` and `RequireAdmin`; no route or middleware change is needed. |
| Error convention | `pkg/handlers/error_messages.go` and existing handlers | 400 responses use JSON with a `detail` string. The frontend reads `detail`, so field names and reasons should be included in that string rather than introducing a new envelope. |

## Resolved decisions

1. **Validate at onboarding, not config save.** The frontend and existing comments allow partial config drafts. The authoritative gate is therefore immediately before onboarding publication.
2. **Use the existing generated query.** `GetZatcaBranchConfig` already supplies the exact writable contract and branch join. No SQL query or schema change is necessary.
3. **Require all 12 persisted config fields for onboarding.** This matches the existing connect UI and prevents worker fallback/default behavior from making a queued request appear valid when the authoritative branch config is incomplete.
4. **Validate VAT, CRN, and CSR location formats.** These are existing UI contract rules and avoid queueing values that the ZATCA worker/provider cannot use. The checks remain limited to the established field formats.
5. **Leave city and company Arabic name out of this backend config validator.** They are not fields in `branch_zatca_config` or the existing backend config response; the worker/frontend owns those inputs in the dependent lanes. This avoids inventing a new persistence/query contract in T026.
6. **Introduce only a publisher interface seam.** The concrete `*ZatcaPublisher` API and misspelled compatibility method remain unchanged; the interface permits a focused no-publication regression test.

## Risks and mitigations

- Existing deployments with old/incomplete config rows will receive a clear 400 instead of an asynchronous provider failure; the response names every missing field.
- Strict format checks could reject legacy malformed values, which is intentional for the pre-queue boundary and matches the current connect UI.
- `GetZatcaBranchConfig` errors other than `sql.ErrNoRows` remain 500 and are logged without exposing database details.

## Remaining uncertainty

None blocking. The dependent worker's company/store requirements are intentionally outside T026 because they are not represented in the existing backend ZATCA config contract.
