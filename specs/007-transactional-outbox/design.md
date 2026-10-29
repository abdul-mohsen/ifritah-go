# T007 Design

## Boundary

`business transaction -> outbox row -> post-commit drain -> NATS/JetStream`

Only backend persistence, handlers, publisher lifecycle, tests, and backend
documentation change. The downstream daemon consumes the existing event
contract and is out of scope.

## Data Model

### `zatca_event_outbox`

The additive table stores:

- `id`: monotonic primary key.
- `event_id`: durable event identity.
- `tenant_id`: existing event-contract tenant/database identity.
- `merchant_id`: authenticated database ownership key, nullable only where the
  existing onboarding domain has no separate merchant row.
- `document_type`: `bill`, `credit`, `debit`, or `onboard`.
- `document_id` and `branch_id`.
- `idempotency_key`: stable JetStream message identity.
- `payload`: versioned event JSON.
- `status`: `pending`, `processing`, `published`, or `failed`.
- `attempt_count`, `available_at`, `claimed_at`, `published_at`,
  `last_error`, `last_error_at`, `created_at`, and `updated_at`.

The unique idempotency key prevents duplicate enqueue for the same durable
request. Indexes cover due status/time and tenant/document lookup.

## Transactional Enqueue

1. Handler begins its existing transaction.
2. Handler performs the business mutation and all existing stock/domain work.
3. Handler constructs the existing `Message` with tenant, branch, document
   identity, event ID, idempotency key, and OTP where applicable.
4. Handler marshals the message and calls the generated outbox insert through
   `qtx`.
5. Handler commits. If any step fails, the transaction rolls back and no
   outbox row exists.

Bill and credit handlers use the authenticated session ID for `merchant_id`.
Onboarding uses its existing admin-authenticated session ID and branch ID.
No new ownership or authorization rule is introduced.

## Drain and Retry

`ZatcaOutboxDrainer` owns a database handle, generated query object, and
`ZatcaPublisher`:

1. Claim a bounded batch of due `pending`/`failed` rows in a short transaction.
2. Publish each stored payload outside the transaction using the existing
   subject and JetStream message identity.
3. On acknowledgement, mark the row `published`.
4. On error, increment `attempt_count`, mark `failed`, persist a bounded
   sanitized error, and calculate the next `available_at` with capped
   exponential backoff.
5. Continue processing other rows; one failure does not discard the batch.

Claiming is conditional on status and availability. A stale `processing` row
is eligible after a lease timeout, allowing recovery after a process crash.
The exact payload/event identity is reused on every retry.

The main process starts a bounded ticker goroutine after handler/publisher
construction and stops it on shutdown. Handlers may request a non-blocking
drain wake-up after commit, but correctness does not depend on the wake-up.

## Query Interfaces

`pkg/db/queries/zatca_outbox.sql` defines generated methods for:

- inserting a row in the caller's transaction;
- claiming due rows;
- marking a row published;
- marking a row failed with retry metadata;
- releasing stale claims.

The SQL uses explicit state predicates and does not bypass tenant/domain
arguments. Generated sqlc code remains ignored pipeline output.

## Failure Handling

- A database failure while enqueueing returns an HTTP failure and rolls back
  the business transaction.
- A NATS failure never makes the original request pretend that publication
  succeeded; it is recorded in the outbox and retried.
- A failure while updating publication state remains observable through logs
  and the next due/stale claim; no event is deleted on uncertain outcome.
- JetStream idempotency uses the stored key to tolerate acknowledgement/update
  races.

## Security and Domain Boundaries

- Existing JWT middleware and admin requirement for onboarding remain intact.
- Existing store/branch access checks remain unchanged.
- Payload tenant/document IDs are derived from authenticated context and
  handler-created IDs, not client-supplied arbitrary queue identity.
- OTP is persisted only as part of the existing event payload and is not
  written to logs or error text.

## Performance and Rollout

- Enqueue adds one indexed insert to each relevant business transaction.
- Drain uses small batches and short claim transactions; NATS calls occur
  outside database locks.
- Deploy migration before application rollout. Existing historical business
  rows remain unchanged and are not backfilled.
- Rollback requires stopping the new application before removing the table;
  no destructive rollback is supplied because rows may contain pending work.

## Test Strategy

- Handler sqlmock tests verify `ExpectBegin`, business SQL, outbox insert,
  `ExpectCommit`, and absence of publisher calls before commit.
- Publisher/drainer tests use a fake publish function and sqlmock to assert
  success and durable failure transitions.
- The red baseline runs before implementation and documents the current
  pre-commit/ignored-error behavior.
- Run source generation, formatting, build, vet, targeted tests, and the full
  Go suite.

## Requirement Traceability

| Requirement | Design element |
|---|---|
| FR-1 / atomic enqueue | Handler transaction plus generated outbox insert |
| FR-2 / no pre-commit publish | Post-commit drainer and wake-up |
| FR-3 / durable lifecycle | Outbox status, attempts, errors, timestamps |
| FR-4 / retries | Conditional claim, lease, capped backoff |
| FR-5 / identity | Stored versioned payload and idempotency key |
| FR-6 / boundaries | Authenticated merchant/tenant fields and existing checks |
| FR-7 / compatibility | Additive migration; no historical backfill |
