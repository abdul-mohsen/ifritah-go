# T010 Research

## Evidence inspected

| Area | Evidence | Finding |
|---|---|---|
| Current onboarding handler | `pkg/handlers/branch.go` | T026 validates OTP/config, then calls `OnboadBranch`; it performs no durable queued transition and returns an empty 200 response. |
| Existing config query | `pkg/db/queries/branch.sql` | `GetZatcaBranchConfig` returns the 12 editable fields plus status/certificate lengths and branch name, but no onboarding progress metadata. |
| Existing schema | `pkg/db/schema/schema.sql` | `branch_zatca_config` already has `onboard_state`, `last_error`, `last_attempt_at`, and `zatca_onboarded_at`, but no separate step, attempt number, retry counter, error code, or queued/start/completion timestamps. |
| Existing migration set | `pkg/db/migrations` and `docker-compose.yml` | The checked-in migration sequence ends at `0008_zatca_submission_state.sql`; Docker seeds that migration explicitly. T010 needs a new migration and seed entry for existing/fresh databases. |
| Later daemon contract | `zatca-fix-t008-zatca/processor/onboard.go`, `scripts/migration_zatca_schema.sql` | The daemon already understands CSR, compliance invoice, and production phases and uses the branch config row as its persistence boundary, but it does not yet update the backend state in this lane. |
| Client polling contract | `afrita-go-vin-to-parts-smart-search/handlers/zatca.go` | The existing frontend-side consumer already looks for `onboard_state` and `last_error` from `GET /branch/:id/zatca`; backend exposure is currently missing. |
| Validation contract | `.specify/specs/t026-backend-validation/*`, `pkg/handlers/zatca_validation.go` | OTP must be six ASCII digits after trimming, and all 12 persisted config fields must be valid before publication. |
| Transaction patterns | `pkg/handlers/bill.go`, `cash_vouchar.go` | The backend uses `Begin`, `queries.WithTx`, deferred rollback, and commit for multi-write operations, providing the established atomic database pattern. |
| Generated-code policy | `sqlc.yaml`, `README.md`, `.gitignore` | sqlc output is generated locally and ignored; source schema/query files are authoritative and generated output must not be hand-edited or committed. |

## Resolved decisions

1. **Extend the existing branch row and add attempt history.** The branch row
   is already the daemon's authoritative configuration boundary and is what the
   API reads. A child attempt table preserves retry/error history instead of
   overwriting it.
2. **Use `queued` as the new initial state.** Existing `csr`/`invoices` rows
   remain readable for compatibility, while the backend's accepted request
   explicitly records `queued`.
3. **Keep state and step separate.** State describes the lifecycle visible to
   clients (`queued`, `compliance`, `production`, `done`, `failed`); step
   describes the worker's finer-grained progress (`csr`, `compliance`,
   `invoices`, or `production`).
4. **Use a database transaction for the local atomic transition.** The update
   of the branch row and insertion of the attempt row commit together. The
   publisher is called after commit; a publisher error is written back as a
   failed attempt because a true database/queue transaction requires the
   outbox explicitly excluded by the request.
5. **Expose only non-secret metadata.** Config credentials, private keys,
   compliance secrets, and OTP values remain outside the API response.
6. **Keep T003/T026 boundaries intact.** Submission state tables and strict
   onboarding validation are reused, not redesigned.

## Alternatives considered

### Only add columns to `branch_zatca_config`

Rejected because retries and failures would overwrite the previous attempt and
would not satisfy durable attempt history.

### Add an outbox or change the NATS message

Rejected because the user explicitly excludes outbox and daemon changes from
T010. The local transaction plus post-commit failure persistence is the
smallest safe backend contract under that constraint.

### Add a new frontend route

Rejected because the existing GET branch-ZATCA endpoint is already the polling
boundary used by the dependent client.

## Remaining uncertainty

- Live migration execution depends on a MySQL 8 instance and deployment
  privileges. sqlc parsing, generated contracts, sqlmock regressions, and
  static migration checks can be run without one; live DDL evidence will be
  reported separately.
