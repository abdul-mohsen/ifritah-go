# T022 Research

## Baseline

- Worktree: `fix/zatca/h8-monitor-api`.
- Base commit: `fc084a4`, the T003 durable submission-state implementation.
- Existing monitor routes:
  - `GET /api/v2/zatca/monitor/stats`
  - `GET /api/v2/zatca/monitor/branches`
  - `GET /api/v2/zatca/monitor/submissions`
- Routes are mounted behind `JWTVerifyMiddleware` and `RequireAdmin`.
- `getMerchantID` reads the authenticated JWT claim and is already passed to
  the three T003 monitor queries.

## Evidence

### Durable schema

`pkg/db/schema/schema.sql` and migration
`pkg/db/migrations/0008_zatca_submission_state.sql` define:

- `zatca_submission` as the current durable row, linked to `bill`, with
  `merchant_id`, `document_id`, `outcome_state`, provider fields, retry count,
  retry timestamps, outcome timestamp, and artifact paths.
- `zatca_submission_outcome` as append-only provider observations ordered by
  `observed_at` and `id`.
- `zatca_submission_attempt` as durable attempt/retry history with
  `attempt_number`, `attempt_state`, provider/error fields, retry count, and
  timestamps.

The six checked states are `pending`, `accepted`,
`accepted_with_warning`, `rejected`, `ambiguous`, and `terminal_failure`.

### Current query behavior

`pkg/db/queries/zatca_monitor.sql` already applies both
`bill.merchant_id = sqlc.arg('merchant_id')` and
`zatca_submission.merchant_id = sqlc.arg('merchant_id')` to the monitor
queries. However:

1. State expressions use a literal `'pending'` when both durable state
   expressions are empty.
2. Provider detail expressions fall back field-by-field from the latest outcome
   to the submission row even when a latest outcome exists with a null optional
   field.
3. Branch rows are selected when a tenant has any bill for the branch, even if
   the branch has no durable ZATCA submission.
4. Branch output only contains daily/weekly summary values and does not expose
   all six state counts or pending age.
5. Submission output does not expose latest durable attempt state, attempt
   number, pending age, or durable retry error details.

### Current handler/model behavior

`pkg/handlers/zatca_monitor.go` preserves legacy response fields and maps an
empty explicit state through the legacy numeric status. This mapping is useful
for API compatibility, but it MUST NOT be used to invent provider fields or
turn a missing durable row into a pending row. `pkg/model/zatca_monitor.go`
already has additive provider/retry/artifact fields that can be extended.

### Existing tests and pipeline

- `pkg/db/zatca_monitor_regression_test.go` uses `go-sqlmock` against the
  generated sqlc query object.
- Generated files under `pkg/db/gen` are ignored and are produced by
  `sqlc generate`.
- `sqlc.yaml` parses `pkg/db/schema`, `pkg/db/migrations`, and
  `pkg/db/queries`; source SQL is authoritative.
- The repository README already documents the generation command.

## Decisions

| Requirement | Evidence | Decision |
|---|---|---|
| Tenant isolation | Existing auth helper and T003 predicates | Keep both bill and durable submission tenant predicates on every monitor query. |
| State source | Current submission plus latest outcome tables | Prefer latest outcome state when an outcome row exists; otherwise use the durable submission state; never use a literal state default. |
| Provider source | Latest outcome has nullable provider fields | When a latest outcome exists, return its nullable fields as-is; only use current submission provider fields when no outcome exists. |
| Branch inclusion | Current branch query uses bill-only existence | Join durable submissions so branches without submissions do not appear as fabricated monitor rows. |
| Retry source | Attempt table contains attempt state/history | Project the latest attempt state/number/error fields; preserve current submission retry count/timestamps for existing API fields. |
| Pending age | Durable submission and creation timestamps | Project the oldest pending timestamp and age; return null age when no pending row exists. |
| Compatibility | Existing model fields/routes and T003 docs | Add fields only, retain `warnings`, legacy numeric status, filters, route paths, and middleware. |
| Test strategy | Existing sqlmock/generated query pattern | Add seeded query tests and handler tests using `db.New(sqlmockDB)`; do not introduce a new query abstraction. |

## Alternatives Rejected

1. **Keep the literal pending fallback**: rejected because it makes missing
   state appear operationally pending and violates the requirement to report
   real durable rows only.
2. **Return branch configuration rows with zero aggregates**: rejected because
   a configured branch without a durable submission is not a submission monitor
   row.
3. **Copy provider fields independently from two rows**: rejected because it
   can combine a latest outcome's null field with stale current-row evidence.
4. **Add a new route or frontend contract**: rejected because API compatibility
   and frontend/out-of-scope constraints require the existing routes.

## Remaining Uncertainty

No unresolved requirement-level uncertainty remains. Exact nullable Go types
will be confirmed by sqlc generation and handled in the backend handler.
