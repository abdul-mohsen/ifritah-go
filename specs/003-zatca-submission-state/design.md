# T003 Design

## Boundary

This lane changes only backend persistence and read contracts:

`MySQL schema/migration -> sqlc source queries -> generated database contract ->
monitor handler/model`.

It does not add a worker, daemon, outbox event, provider client, or frontend
route.

## Data Model

### `zatca_submission` current record

Retain the existing `id`, `bill_id`, legacy numeric `status`, rejection fields,
submitted/cleared/response fields, retry count, and timestamps. Add:

- `document_id`: stable non-null application/document identity, backfilled from
  `bill.invoice_uuid` or a deterministic `bill:<id>` fallback.
- `merchant_id`: tenant key copied from `bill.merchant_id`; the pair
  `(merchant_id, document_id)` is unique.
- `outcome_state`: checked string state with the six approved values, defaulting
  to `pending`.
- `provider_code`, `provider_warning_code`, `provider_warning_message`, and
  `provider_reference`.
- `outcome_at`, `last_attempt_at`, and `next_retry_at`.
- `request_xml_path` and `cleared_xml_path`.

The existing `bill_id` uniqueness remains the current one-submission-per-bill
invariant. A composite `(bill_id, document_id)` lookup/index supports stable
document reads without changing invoice monetary data.

### `zatca_submission_outcome`

Append-only provider observations:

- submission and optional attempt references;
- checked explicit state;
- provider code/reference and warning/rejection detail;
- observed and created timestamps.

The latest observation is selected by `observed_at DESC, id DESC`.

### `zatca_submission_attempt`

One row per provider request try:

- submission and attempt number;
- attempt state/error/provider identifiers;
- started/completed timestamps;
- retry count and next retry timestamp;
- request/cleared XML paths;
- created/updated timestamps.

`(submission_id, attempt_number)` is unique, preserving history while making
retries deterministic.

### `zatca_submission_artifact`

Evidence metadata associated with a submission and optional attempt:

- artifact kind (`request_xml`, `cleared_xml`, `response_xml`, `other`);
- path/reference, content type, SHA-256, JSON metadata;
- created timestamp.

The foreign keys flow through `zatca_submission` to `bill`; the submission also
stores the tenant key so monitor queries can enforce both the source-bill and
durable-record boundary without duplicating tenant IDs in every child table.

## Query Design

### Statistics

`GetZatcaMonitorStats(merchant_id)` joins `bill` and counts the current
`outcome_state` per tenant. It returns counts for all six states plus a total.
Legacy rows with no explicit state are represented as `pending` until an
outcome is recorded; no provider outcome is fabricated.

### Branch overview

`ListZatcaMonitorBranches(merchant_id)` limits branch rows to branches with an
in-scope bill and computes today/week/latest metrics from the current explicit
state. The certificate column remains an existing configuration value.

### Submission detail

`ListZatcaMonitorSubmissions` accepts the tenant id, optional pending/state
filters, optional branch filter, and limit. It returns stable document identity,
explicit current/latest outcome fields, retry timestamps/count, and XML paths.
The query uses a derived latest-outcome selection so old rows without outcome
history remain visible with their current state.

## Handler and Model Contract

- Resolve the authenticated tenant using the existing `getMerchantID` helper.
- Pass the tenant id to all three generated monitor queries.
- Prefer the explicit `outcome_state`; only map the legacy numeric status for
  legacy rows where the state is empty.
- Expose explicit state labels and provider/retry/artifact fields in
  `pkg/model/zatca_monitor.go`.
- Preserve existing endpoint paths and response fields while adding compatible
  fields; no frontend route changes are required.

## Migration and Compatibility

1. Add the new columns to the checked-in schema dump.
2. Add replay-safe `0008_zatca_submission_state.sql`.
3. Add child tables and indexes.
4. Backfill `document_id` from the bill identity and set legacy rows to
   `pending` when no explicit outcome exists.
5. Preserve legacy `status`, `response_xml`, and monetary columns.

The migration must be safe for existing rows and must not drop or rewrite
invoice totals. Fresh Docker initialization receives the final schema; the new
migration is also listed in the seed files for existing-schema compatibility.

## Regression Test

The focused `pkg/db/zatca_monitor_regression_test.go` uses sqlmock to return a
provider outcome row with 20 expected monitor columns. At base, the generated
query scans only 8 legacy columns and fails with a scan mismatch. After
schema/query/sqlc regeneration, the same test reads `accepted_with_warning`,
provider code, and warning code from the generated row.

## Requirement Traceability

| Requirement | Design element |
|---|---|
| FR-001 / stable identity | `zatca_submission.document_id`, bill link, unique/index |
| FR-002 / six states | checked `outcome_state` on submission/outcome/attempt |
| FR-003 / provider details | current and append-only outcome fields |
| FR-004 / retry evidence | attempt table plus current retry timestamps/count |
| FR-005 / artifacts | current XML paths plus artifact metadata table |
| FR-006 / monitor reads | tenant-scoped stats/branches/submission queries |
| FR-007 / compatibility | additive migration, retained legacy fields/totals |
| NFR / sqlc | source schema/query updates and pipeline generation |
