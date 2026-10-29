# T010 Backend ZATCA Onboarding State Requirements

## Problem

The backend accepts a valid onboarding request and publishes it to the
asynchronous ZATCA worker, but the branch row does not provide a durable,
observable contract for the queued attempt, worker step, retry history,
failure details, or completion time. The existing `onboard_state`,
`last_error`, and `last_attempt_at` fields are incomplete and are not exposed
by the backend configuration query. Operators and clients therefore cannot
distinguish a newly queued request from compliance, production, completed, or
failed onboarding, nor can they inspect retry/error evidence without reading
database internals.

## Goals

- Persist one durable onboarding attempt record and the branch's current
  onboarding state.
- Make the initial queued transition atomic within the backend database,
  including the attempt number and cleared retry/error state.
- Expose current onboarding state, step, attempt, retry, error, and completion
  metadata through the existing branch ZATCA API/query contract.
- Preserve the strict OTP/config validation added by T026 before any durable
  transition or publisher call.
- Provide a backend contract that the later daemon can update without changing
  the frontend or queue message in this lane.

## In scope

- `branch_zatca_config` onboarding bookkeeping columns and a durable
  `zatca_onboarding_attempt` history table.
- A replay-safe migration, checked-in schema dump, sqlc source queries, and
  regenerated local sqlc contracts.
- The existing `GET /api/v2/branch/:id/zatca` response and the
  `POST /api/v2/branch/:id/zatca/onboard` queued response.
- Atomic database transition to `queued`, publisher-failure persistence, and
  daemon-facing progress/error/completion update queries.
- Focused regression tests for queued, compliance, production, done, and
  failed visibility, plus retry/error persistence.
- Related backend README and Spec Kit artifacts.

## Out of scope

- Daemon/worker implementation or provider API behavior.
- Frontend/UI changes.
- NATS/outbox redesign or a transactional outbox.
- Authentication, tenant, role, invoice, submission, or monetary data
  changes.
- Editing generated sqlc output by hand.

## User stories

1. As an administrator, after submitting a valid OTP, I can see that the
   branch is queued and identify the current attempt.
2. As an operator, I can distinguish compliance and production progress from
   done and failed onboarding through the existing branch endpoint.
3. As an operator, when an attempt retries or fails, I can retrieve its retry
   count, error code/message, and timestamps after a process restart.
4. As the later daemon, I can update a durable attempt by branch and attempt
   number without losing the previous attempt history.
5. As a security boundary, invalid OTP/config requests never create or change
   an onboarding attempt and never publish an onboarding message.

## Functional requirements

### FR-1: Strict pre-queue validation

- The handler MUST retain T026's exact-six-ASCII-digit OTP validation and
  trimming behavior.
- The handler MUST retain T026's persisted 12-field configuration validation.
- Invalid input MUST return HTTP 400 before any transaction, state update, or
  publisher call.

### FR-2: Durable attempt contract

- Each accepted queue request MUST have a monotonically increasing attempt
  number for the branch.
- Attempt history MUST retain state, worker step, retry count, error code,
  error message, queued/started/completed timestamps, and row timestamps.
- Attempt rows MUST be linked to an existing branch and MUST be tenant-safe
  through the branch's existing database boundary.
- The OTP MUST NOT be persisted in the attempt or branch bookkeeping fields.

### FR-3: State and step contract

- The current branch state MUST expose these operational states:
  `not_started`, `queued`, `compliance`, `production`, `done`, and `failed`.
- The current step MUST be separately observable and support at least
  `queued`, `csr`, `compliance`, `invoices`, `production`, and `done`.
- Existing historical `csr` and `invoices` state values MUST remain readable
  during compatibility migration, but new queue requests MUST use
  `onboard_state = queued`.
- A completed state MUST have a non-null completion timestamp; a failed state
  MUST retain its latest error evidence.

### FR-4: Atomic queued transition

- After validation, the backend MUST update the branch bookkeeping and create
  the corresponding attempt row in one database transaction.
- The transition MUST increment the attempt number, reset retry/error fields,
  set state/step to `queued`, set the queued/attempt timestamp, and clear a
  stale completion timestamp.
- A request already in `queued`, `compliance`, `production`, or `done` MUST
  not create a duplicate active attempt.
- The existing queue publisher MUST be called only after the database
  transaction commits.
- If publication fails, the backend MUST persist a failed state/error for the
  queued attempt and return the existing HTTP 500 behavior.

### FR-5: API/query exposure

- `GET /api/v2/branch/:id/zatca` MUST expose current state, step, attempt,
  retry count, error code/message, queued/started/completed/last-attempt
  timestamps, and the existing completion timestamp when present.
- The response MUST continue omitting credentials/private keys.
- A successful onboarding request MUST return a JSON body identifying the
  queued state and attempt without claiming production/active completion.
- sqlc source queries MUST expose the contract and provide updates for future
  daemon progress, retry/error, and completion writes.

### FR-6: Migration and compatibility

- Fresh schema and existing deployments MUST receive the same contract.
- The migration MUST be replay-safe for columns, indexes, constraints, and the
  attempt table.
- Existing configuration values and completion credentials MUST be preserved.
- Existing T003 submission tables and T026 validation behavior MUST remain
  unchanged.

## Non-functional requirements

- State transitions MUST be deterministic and guarded against duplicate active
  attempts.
- Error responses MUST not include OTPs, credentials, or raw database errors.
- Queries MUST use indexed branch/state/attempt lookups suitable for polling
  and daemon updates.
- Generated sqlc files MUST be reproducible with the repository's `sqlc
  generate` command and MUST NOT be committed if ignored by the repository.

## Acceptance criteria

1. A focused regression test fails before production changes because the
   existing API does not expose the requested onboarding fields and the
   onboarding request does not durably transition to `queued`.
2. After implementation, focused tests prove visibility of `queued`,
   `compliance`, `production`, `done`, and `failed`.
3. Focused tests prove retry count and error code/message survive persistence
   and are returned by the API/query contract.
4. A duplicate active queue request does not create a second active attempt.
5. Strict OTP/config tests from T026 remain green.
6. `gofmt`, `sqlc generate`, `go build ./...`, `go vet ./...`, targeted tests,
   and `go test ./...` are run; any exact repository/tooling blocker is
   recorded.
7. No daemon, frontend, outbox, or unrelated files are included in the T010
   commit.

## Assumptions, constraints, risks, and unknowns

### Assumptions

- `branch_zatca_config` is one row per branch and is the existing authoritative
  backend configuration boundary.
- The later daemon will update state using branch ID plus the durable attempt
  number.
- The existing `zatca_onboarded_at` field is the compatibility completion
  timestamp and can be mirrored by a new explicit completion field.

### Constraints

- The branch is based on T003 commit `fc084a4` and T026 commit `15ffd3c`;
  both must remain ancestors and their behavior must be preserved.
- No outbox or daemon changes are allowed.
- The repository's generated sqlc output is ignored and must be generated
  locally for compilation/validation.

### Risks

- A database commit followed by a publisher failure cannot provide true
  cross-system atomicity without an outbox; this lane records the failure
  durably and explicitly leaves the outbox out of scope.
- Older deployments may contain `csr`, `invoices`, or `not_started` state
  values; compatibility reads must not make those rows invisible.
- Existing schema/migration tooling may reject some MySQL conditional DDL;
  the exact command result must be recorded rather than hidden.

### Unknowns / open questions

- The daemon's final step granularity may evolve after T010. The schema must
  allow the known CSR/compliance/invoices/production steps without forcing a
  daemon implementation into this lane.
- A live MySQL instance may not be available in the execution environment, so
  migration execution may remain a documented blocker even when sqlc parsing
  and unit tests pass.

## Observable success measures

- A caller can submit a valid OTP and immediately read `onboard_state=queued`
  with a non-zero attempt.
- The same response/query shape can represent compliance, production, done,
  and failed rows without exposing secrets.
- Retry/error/completion fields remain available after reopening the database.
