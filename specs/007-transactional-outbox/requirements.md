# T007: Transactional and Durable ZATCA Queue Publication

## Problem

Bill, credit-note, debit-note, and ZATCA onboarding handlers currently call
NATS before their database transaction commits. A publish error is logged and
ignored, so a request can return success with no durable indication that the
downstream work was lost. A process crash between publication and commit can
also leave a queue message referring to a rolled-back business row.

## Goals

- Persist each queue event in the same database transaction as the business
  change that created it.
- Publish only committed outbox rows.
- Retry transient publication failures from durable state after a process
  restart.
- Preserve a durable failure/error state when a publication attempt fails.
- Keep tenant, authentication, and document-domain boundaries unchanged.

## In Scope

- An additive outbox table and replay-safe migration.
- sqlc source queries and generated contracts for enqueue, claim, publish
  success, and publish failure transitions.
- Transactional enqueue for bill creation/submission, credit-note creation,
  debit-note publication entry points, and onboarding.
- A backend publisher drain loop that runs after commit and retries pending or
  failed rows with bounded backoff.
- Focused sqlmock and handler regressions for pre-commit publication and
  ignored publication errors.
- Backend README documentation for the durable queue behavior.

## Out of Scope

- Frontend, daemon/worker, deployment, or provider-client changes.
- Changing authentication, authorization, store access, or tenant ownership
  rules.
- Replacing the existing ZATCA event contract or NATS subject scheme.
- A live migration run against production data.
- Creating or pushing a pull request.

## User Scenarios & Testing

### Scenario 1: Commit business data and queue event together

Given a valid bill, credit note, debit note, or onboarding request, when the
handler succeeds, then the business row and exactly one durable outbox event
are committed together. If either database write fails, neither is committed
and no publication is attempted.

### Scenario 2: Publish only after commit

Given a committed outbox event, when the post-commit publisher drains it, then
the event is published with the existing tenant/document identity and the
outbox row is marked published. A rolled-back business transaction must never
be published.

### Scenario 3: Recover from publication failure

Given a committed outbox event and an unavailable NATS/JetStream publisher,
when a drain attempt fails, then the row records the failure, attempt count,
and next retry time. A later drain retries it without requiring the original
HTTP request.

### Scenario 4: Preserve boundaries

Given two authenticated tenants or branches, when either handler enqueues an
event, then the outbox payload and database predicates retain the caller's
tenant/document/branch boundary. Onboarding remains admin-gated and document
events retain their existing IDs.

### Scenario 5: Regression on the base

Given the focused transaction and failure tests, when they run at commit
`fc084a4`, then they fail because publication occurs before commit or a
publication error is ignored. After T007, the same tests pass.

## Functional Requirements

### FR-1: Atomic enqueue

The system MUST insert the outbox event using the same SQL transaction as the
business row mutation for every in-scope publication path.

### FR-2: No pre-commit publication

The request handlers MUST NOT call NATS/JetStream while the business
transaction is still open. Publication MUST happen only from committed
outbox rows.

### FR-3: Durable lifecycle

Each outbox row MUST retain a lifecycle state, attempt count, last error, and
next-attempt timestamp. A successful publication MUST be durable and
idempotent; a failed attempt MUST update durable state rather than only
logging.

### FR-4: Drain and retry

The backend MUST drain due pending/failed rows after commit and on a recurring
interval. A transient failure MUST remain eligible for a later retry with a
bounded backoff and no loss of the original event identity.

### FR-5: Event identity and payload

The outbox MUST preserve the existing bill, credit, debit, and onboarding
document type, document ID, branch ID, tenant identity, and onboarding OTP
semantics. Retries MUST reuse the durable event payload/idempotency identity.

### FR-6: Tenant and auth boundaries

Outbox writes MUST derive tenant/merchant scope from the authenticated
request and existing domain ownership checks. No new route or authorization
path may broaden access.

### FR-7: Migration compatibility

The migration MUST be additive and replay-safe, preserve existing business
rows and invoice totals, and leave historical data valid when no outbox event
exists.

## Non-Functional Requirements

- The existing sqlc source-query workflow remains authoritative; generated
  output is never hand-edited.
- Claiming and state transitions MUST be safe when more than one drain attempt
  runs.
- Error text persisted to the database MUST be bounded and sanitized for
  operational use.
- Existing event subjects and compatibility fields remain unchanged.
- Tests MUST run deterministically without live MySQL, NATS, or the daemon.

## Acceptance Criteria

1. A replay-safe migration defines an outbox table with event identity,
   document/tenant fields, payload, lifecycle state, retry scheduling, and
   error timestamps.
2. Bill, credit, debit, and onboarding business mutations enqueue inside the
   same transaction; no handler publishes before commit.
3. A publisher drain marks a row published only after NATS acknowledges it and
   records durable failure state when publication fails.
4. A process restart or later drain can retry a due failed event with the
   original payload and idempotency key.
5. Focused sqlmock/handler tests fail at `fc084a4` and pass after T007,
   demonstrating both transaction ordering and durable failure handling.
6. `gofmt`, `sqlc generate`, `go build ./...`, `go vet ./...`, targeted tests,
   and `go test ./...` pass, or exact blockers are recorded.
7. The diff contains backend and feature artifacts only; no frontend, daemon,
   deployment, or PR changes are included.

## Assumptions, Constraints, Risks, and Unknowns

### Assumptions

- `DBNAME` remains the tenant identity required by the existing ZATCA event
  contract; authenticated `merchant_id` remains the database ownership key.
- The existing `ZatcaPublisher` remains the NATS adapter, while outbox state
  and scheduling are database-owned.
- An outbox payload is application-managed JSON and may contain the onboarding
  OTP because the existing event contract already requires it.
- `SubmitDebit` is the existing debit publication entry point even though the
  current baseline has no debit HTTP handler; T007 must make the entry point
  durable without inventing a new route.

### Constraints

- Work only in the target backend worktree and branch.
- Do not edit frontend, daemon, or deployment files.
- Do not alter the generated sqlc output by hand.
- Do not use a live external provider or require a running NATS/MySQL service
  for regression tests.

### Risks

- Existing deployments require the new migration before handlers can enqueue.
- Multiple backend processes may drain the same rows; row claiming must avoid
  duplicate in-flight work.
- A process can fail after NATS acknowledgement but before the success update;
  JetStream idempotency and the durable event key must make the retry safe.

### Resolved Unknowns

- The repository has no existing outbox table or worker; T007 will add the
  smallest backend-owned drain loop rather than changing the daemon.
- Main initializes the publisher and handler, so the drain lifecycle belongs
  beside that initialization and must not publish in request transactions.
- The existing migration directory and sqlc schema/query directories are the
  authoritative migration and generation locations.
