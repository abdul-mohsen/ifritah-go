# T003 Research

## Repository Evidence

### Existing ZATCA persistence

- `pkg/db/schema/schema.sql` contains `zatca_submission` with one row per
  `bill_id`, numeric `status` values `0..4`, rejection code/message, submitted
  and cleared timestamps, inline response XML, retry count, and created/updated
  timestamps.
- The legacy table has no stable document key, copied tenant key, provider outcome history,
  attempt table, artifact table, provider warning fields, or next retry
  timestamp.
- `bill` contains `merchant_id`, `branch_id`, and `invoice_uuid`. Bill creation
  currently stores the authenticated user id in `merchant_id`; therefore
  monitor reads must scope through `bill.merchant_id` to preserve the existing
  tenant boundary.

### Existing monitor contract

- `pkg/db/queries/zatca_monitor.sql` has three sqlc queries:
  `GetZatcaMonitorStats`, `ListZatcaMonitorBranches`, and
  `ListZatcaMonitorSubmissions`.
- All three queries currently read legacy numeric `status` values and omit a
  tenant predicate.
- `pkg/handlers/zatca_monitor.go` maps the numeric states to the old labels
  `pending`, `accepted`, `rejected`, and `warning`.
- No ZATCA monitor unit test existed at the base. The new focused regression
  test was added before the production change and fails at base because the
  generated query scans 8 legacy columns while the expected provider-outcome
  contract has 20 columns:

  ```
  sql: expected 20 destination arguments in Scan, not 8
  ```

### Migration and generation conventions

- Authoritative deployable schema changes live in `pkg/db/migrations`.
- The checked-in `schema.sql` is a MySQL dump used as the baseline schema.
- Existing migrations use additive/replay-safe MySQL patterns based on
  `information_schema`, prepared statements, and `CREATE TABLE IF NOT EXISTS`
  where appropriate.
- `sqlc.yaml` reads `pkg/db/schema` and `pkg/db/migrations`, emits Go to
  `pkg/db/gen`, and maps nullable database values to pointers.
- `pkg/db/gen` is ignored by git. CI runs `sqlc generate` before build/test;
  local validation must do the same. Installed local version is sqlc 1.30.0;
  the workflow requests 1.27.0, so generated code must remain compatible with
  the repository's existing pipeline rather than being hand-edited.
- Existing unit tests use `go-sqlmock` and construct handlers/queries without a
  live database. The T003 regression follows that deterministic pattern.

## Decisions

### D1: Extend the existing submission row and normalize history

Keep `zatca_submission` as the compatibility anchor and add explicit current
state/provider/retry/artifact-summary columns. Add
`zatca_submission_outcome`, `zatca_submission_attempt`, and
`zatca_submission_artifact` child tables for durable history and evidence.

**Reason**: Existing monitor/dashboard SQL and the foreign key from `bill`
already establish the document boundary. Additive columns preserve old rows,
while child tables prevent retries and artifacts from overwriting one another.

### D2: Use textual checked states

Represent operational state in a `VARCHAR(32)` column with a `CHECK` constraint
rather than reusing the legacy numeric `status` or introducing an opaque
numeric enum. The permitted values are exactly:

`pending`, `accepted`, `accepted_with_warning`, `rejected`, `ambiguous`,
`terminal_failure`.

**Reason**: sqlc emits a stable string contract, monitor responses remain
self-describing, and the legacy numeric status can remain for compatibility.

### D3: Copy and verify the bill tenant key for monitor scope

The migration copies `bill.merchant_id` to the submission record and every
document-level monitor query joins `bill` and applies both
`bill.merchant_id = sqlc.arg('merchant_id')` and
`zatca_submission.merchant_id = sqlc.arg('merchant_id')`. Branch monitor rows
additionally require an in-scope bill for the branch.

**Reason**: Current writes use `merchant_id` as the authenticated user/tenant
key, whereas `company_id` is a separate organization relation. Copying and
checking the key makes stable document uniqueness explicit without changing
authentication or out-of-scope handlers.

### D4: Keep current outcome on the submission and audit outcomes separately

`zatca_submission.outcome_state` is the current monitor state. Each provider
response is also append-only in `zatca_submission_outcome`, linked to the
attempt when available. Monitor detail reads the latest outcome row and falls
back to the current submission state for legacy rows.

**Reason**: Fast current-state counts do not require a history aggregate, while
outcome history remains available for reconciliation and retry diagnosis.

### D5: Store references, not business amounts

Request/cleared XML columns store application-managed paths, and artifact
metadata stores kind/path/content type/checksum/JSON metadata. No monetary
columns are changed and no decimal values are converted.

**Reason**: T003 owns evidence persistence, not invoice calculations or XML
generation. Paths/metadata avoid duplicating potentially sensitive XML bodies.

## Alternatives Considered

1. **Replace the numeric `status` column with text**: rejected because legacy
   dashboards/consumers and existing rows rely on numeric values.
2. **Only add columns to `zatca_submission`**: rejected because retry and
   artifact history would be overwritten and could not support multiple
   attempts.
3. **Scope by `company_id` only**: rejected because current bill writes use
   `merchant_id` as the tenant key; using company id would not consistently
   isolate the existing data model.
4. **Store full XML in every outcome row**: rejected because this expands
   durable sensitive payload storage and duplicates the existing response XML;
   paths/artifact metadata satisfy the T003 evidence requirement.

## Remaining Uncertainty

- The repository has no production migration runner in Go; Docker mounts only
  the earliest migration files. T003 will add the migration to the established
  migration directory and the local compose seed list, but deployment systems
  outside this repository must apply the new file through their existing
  migration process.
- Generated sqlc files remain ignored by design. The committed source of truth
  is schema/query input plus the documented generation command.
