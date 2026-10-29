# T010 Design

## Boundary flow

```text
authenticated admin request
        |
        v
trim + strict OTP/config validation (T026)
        |
        v
BEGIN
  lock/update branch row -> next attempt, queued state/step
  insert immutable onboarding attempt row
COMMIT
        |
        v
publish existing onboarding message
        |
        +-- success: return queued state/attempt
        +-- failure: persist failed state/error for the same attempt, return 500
```

The daemon-facing update queries operate on `(branch_id, attempt_number)`, so a
late worker update cannot overwrite a newer attempt.

## Persistence model

### Current branch contract

`branch_zatca_config` retains the existing compatibility fields and gains:

- `onboard_step`
- `onboard_attempt`
- `onboard_retry_count`
- `onboard_error_code`
- `onboard_error`
- `onboard_queued_at`
- `onboard_started_at`
- `onboard_completed_at`

`last_error`, `last_attempt_at`, and `zatca_onboarded_at` remain available for
older consumers and are kept synchronized by backend update queries.

### Attempt history

`zatca_onboarding_attempt` is append-once per branch attempt number:

- branch and attempt number;
- state and step;
- retry count;
- error code/message;
- queued, started, and completed timestamps;
- created/updated timestamps.

`UNIQUE(branch_id, attempt_number)` makes a retry/replay deterministic. State,
step, and retry indexes support polling and daemon updates. The foreign key
deletes attempts with their branch.

## SQL/query contract

`pkg/db/queries/branch.sql` adds:

- a richer `GetZatcaBranchConfig` projection for API exposure;
- `QueueZatcaOnboarding` for the guarded current-row transition;
- `CreateZatcaOnboardingAttempt` for the transaction's history insert;
- `MarkZatcaOnboardingFailed` for publisher failure persistence;
- `UpdateZatcaOnboardingProgress` for daemon state/step/retry/error writes;
- `CompleteZatcaOnboarding` for the done/completion transition;
- `GetZatcaOnboardingAttempts` for durable attempt visibility and diagnostics.

All updates are guarded by branch and attempt number. The queue transition
allows `not_started` and `failed`, rejects active/done duplicate requests, and
increments the attempt number atomically while holding the branch row lock.

## Handler contract

`GetBranchZatcaConfig` returns the generated row directly, which now includes
only non-secret onboarding metadata. `OnboardBranchZatca`:

1. executes T026 validation;
2. starts a transaction;
3. executes the guarded queue update and detects duplicate active/done state;
4. inserts the matching attempt row;
5. commits before calling `OnboadBranch`;
6. records a failed state if publication errors;
7. returns a JSON queued acknowledgement with attempt/state/step.

A duplicate active request returns the current durable status without
publishing another message. A completed branch returns conflict semantics
without changing its completion evidence.

## Error and failure handling

- Validation errors remain HTTP 400 with `detail`.
- Missing/invalid database state transition is logged and returns a generic
  HTTP 500/409 response without secrets.
- Publisher failure stores a bounded error code/message and retry metadata but
  never stores the OTP.
- The later daemon owns progress/error/completion updates; this lane only
  supplies their source queries and API projection.

## Migration behavior

`0009_zatca_onboarding_state.sql` uses the repository's information-schema
conditional DDL pattern for new columns, creates the attempt table if absent,
adds state/attempt indexes, and backfills step/current metadata from existing
rows without altering credentials or completion values. Docker compose seeds
the new migration after `0008`.

The checked-in schema dump contains the final columns/table so fresh database
initialization and sqlc see the same contract.

## Test strategy

- Add a focused sqlmock handler test before production edits. It uses the richer
  query projection and expects a transaction/update/attempt insert; it must
  fail against the current handler.
- Table-drive GET visibility for `queued`, `compliance`, `production`, `done`,
  and `failed`, asserting state/step/attempt/retry/error/completion fields.
- Exercise publisher failure and daemon-style progress persistence, asserting
  retry/error data is retained and returned.
- Keep the T026 strict OTP/no-publication tests unchanged and green.
- Run `gofmt`, `sqlc generate`, targeted tests, build, vet, and `go test ./...`.

## Security and compatibility

- Existing JWT/admin route registration is unchanged.
- No credentials, private key, compliance secret, or OTP is selected by
  `GetZatcaBranchConfig`.
- Existing legacy state values remain readable.
- Existing T003 submission tables and generated query contracts are not
  changed except for regeneration caused by the new branch query/schema.

## Requirement traceability

| Requirement | Design element |
|---|---|
| FR-1 | T026 validators remain first and side-effect free |
| FR-2 | `zatca_onboarding_attempt` plus current-row metadata |
| FR-3 | Separate current state and worker step with compatibility reads |
| FR-4 | Guarded transaction, unique attempt number, post-commit publisher |
| FR-5 | Rich branch query/model and queued acknowledgement |
| FR-6 | Replay-safe `0009` migration and schema dump |
| NFR safety | no OTP/secret projection, bounded errors, branch+attempt guards |
