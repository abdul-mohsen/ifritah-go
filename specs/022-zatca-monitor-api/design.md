# T022 Design

## Boundary

This change remains within:

`durable schema rows -> sqlc source monitor queries -> generated query
contracts -> monitor model/handler -> seeded regression tests`.

It does not change submission writers, provider clients, workers, outbox,
frontend, deployment, or route middleware.

## Data Flow

1. The admin route receives an authenticated request.
2. The handler obtains the tenant key through `getMerchantID(c)`.
3. The generated sqlc query receives that tenant key and any existing filters.
4. SQL joins bills and durable submissions, then selects the latest durable
   outcome and latest durable attempt per submission.
5. SQL returns aggregate counts, pending timing, retry evidence, and provider
   evidence.
6. The handler serializes nullable database values without manufacturing
   defaults and retains existing JSON fields.

## Query Design

### Shared current-state rule

For each durable submission:

- If a latest outcome row exists, its `outcome_state` and nullable provider
  fields are authoritative.
- Otherwise, use the corresponding fields on `zatca_submission`.
- Do not append a literal `'pending'` or a fabricated provider/retry value.
- If an empty legacy state reaches the handler, retain the existing numeric
  compatibility mapping for the `status`/`state` labels only; provider fields
  remain nullable.

### Tenant statistics

`GetZatcaMonitorStats(merchant_id)` will count only joined durable rows for the
tenant and return:

- total submissions;
- all six explicit state counts;
- oldest pending durable timestamp;
- pending age in seconds when a pending row exists.

### Branch aggregates

`ListZatcaMonitorBranches(merchant_id)` will join branch configuration, tenant
bills, and durable submissions, grouping by branch. It will retain existing
status/certificate/today/week/last-submission projections and add:

- total and six-state counts;
- oldest pending timestamp and age;
- pending retry count and next retry timestamp where applicable.

The inner durable-submission join means a configured branch with no submission
rows is absent rather than represented by fabricated zero activity.

### Submission details

`ListZatcaMonitorSubmissions` will retain all current selections and filters,
then add:

- latest durable attempt number and attempt state;
- latest durable retry error code/message;
- pending-since timestamp and pending age;
- provider fields selected as a complete latest-outcome/current-submission
  source, not field-by-field stale fallback.

The latest outcome is selected by `observed_at DESC, id DESC`; the latest
attempt is selected by `attempt_number DESC`, start time, and id.

## Response Contract

Existing fields remain unchanged. Additive fields:

### Stats

- `oldest_pending_at`
- `pending_age_seconds`

### Branch row

- `total_submitted`
- `accepted`
- `accepted_with_warning`
- `warnings`
- `rejected`
- `pending`
- `ambiguous`
- `terminal_failure`
- `oldest_pending_at`
- `pending_age_seconds`
- `pending_retry_count`
- `pending_next_retry_at`

### Submission row

- `attempt_number`
- `retry_state`
- `retry_error_code`
- `retry_error_message`
- `pending_since`
- `pending_age_seconds`

Existing provider, retry timing, artifact, status, state, and warning fields
remain in place.

## Security and Isolation

- Every aggregate/detail query receives the tenant key from the authenticated
  JWT context.
- Both `bill.merchant_id` and `zatca_submission.merchant_id` predicates remain
  required.
- Branch rows are included only through tenant-scoped bills with durable
  submissions.
- No new route or permission path is introduced.

## Failure Handling

- Existing query errors continue to return the same endpoint-specific 500
  response and are logged.
- Invalid status/branch filters retain the existing 400 responses.
- Missing nullable evidence is encoded as null; it is not replaced with a
  synthetic state, provider detail, or retry record.

## Performance

- Latest outcome/attempt selection remains correlated to each durable
  submission and uses existing indexes.
- Tenant and state predicates are applied in source SQL.
- Branch aggregation is performed in one generated query rather than one query
  per branch.
- Existing submission limit and filters remain unchanged.

## Test Strategy

1. Add seeded sqlmock rows for tenant statistics and assert all six counts plus
   pending age.
2. Add seeded branch rows and assert branch aggregates and null/no-row
   behavior.
3. Add a handler test with authenticated tenant context and a seeded durable
   submission row containing provider and attempt evidence; assert JSON values
   and tenant argument.
4. Keep the existing provider-outcome regression test green.
5. Run sqlc, formatting, targeted tests, build, vet, and the full test suite.

## Requirement Traceability

| Requirement | Design element |
|---|---|
| FR-1/FR-2 | Tenant stats and branch aggregate queries count durable rows by six states. |
| FR-3 | Branch query joins durable submissions and adds state counts while retaining legacy fields. |
| FR-4 | Oldest pending timestamp/age is nullable and sourced from durable timestamps. |
| FR-5 | Latest durable attempt projections plus existing submission retry fields. |
| FR-6 | Complete provider source selection from latest outcome or current row. |
| FR-7 | Existing routes, filters, middleware, and fields remain unchanged. |
| FR-8 | Source SQL is regenerated through sqlc. |
| FR-9 | Seeded SQL and handler regressions cover real-row behavior. |
