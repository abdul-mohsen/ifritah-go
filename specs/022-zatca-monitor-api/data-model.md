# T022 Data Model

## Durable Submission

`zatca_submission` is the current durable row linked to one bill and one
tenant. Monitor-relevant fields include:

- `merchant_id`, `bill_id`, `document_id`
- `outcome_state`, legacy `status`
- provider code/warning/reference fields
- `submitted_at`, `created_at`, `outcome_at`
- `retry_count`, `last_attempt_at`, `next_retry_at`
- request and cleared artifact paths

## Durable Outcome

`zatca_submission_outcome` stores provider observations for a submission.
The monitor selects the latest row by `observed_at DESC, id DESC`. Its state
and nullable provider fields are authoritative when present.

## Durable Attempt

`zatca_submission_attempt` stores retry evidence. The monitor selects the
latest attempt by `attempt_number DESC, started_at DESC, id DESC` and exposes
its attempt number, state, and error details.

## Monitor Aggregates

Tenant statistics group all in-scope durable submissions by current state.
Branch aggregates group the same rows by `bill.branch_id`. Pending age is
measured from `submitted_at`, falling back only to that row's `created_at`, and
is null when no pending row exists.

## Isolation Rules

Every monitor query requires the authenticated tenant key and matches both
`bill.merchant_id` and `zatca_submission.merchant_id`. A branch without a
durable submission row is not a monitor aggregate row.
