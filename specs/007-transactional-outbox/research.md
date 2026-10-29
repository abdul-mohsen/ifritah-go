# T007 Research

## Repository Evidence

1. `pkg/handlers/bill.go` opens a transaction for `AddBill` and
   `SubmitDraftBill`, but calls `h.pub.SubmitBill` before `tx.Commit`; the
   error is logged and the request continues.
2. `pkg/handlers/credit.go` opens a transaction for credit-note and stock
   work, but calls `h.pub.SubmitCredit` before commit and ignores publication
   failure.
3. `pkg/handlers/branch.go` calls `h.pub.OnboadBranch` directly after validating
   OTP syntax. It has no durable business/outbox transaction.
4. `pkg/handlers/pub.go` publishes directly to JetStream and already exposes
   `SubmitBill`, `SubmitCredit`, `SubmitDebit`, and the misspelled
   `OnboadBranch` compatibility method. It currently has no database handle or
   retry state.
5. `pkg/main.go` creates the database, sqlc queries, and publisher once, then
   constructs the handler. This is the available lifecycle boundary for a
   backend drain loop.
6. `pkg/db/schema/schema.sql` and `pkg/db/migrations/0008_zatca_submission_state.sql`
   are the repository's additive schema/migration conventions. Generated
   `pkg/db/gen` output is ignored and must be reproduced with `sqlc generate`.
7. Existing handler tests use `go-sqlmock`, explicit `ExpectBegin`/`ExpectCommit`,
   and Gin contexts containing `model.Claims`. This supports deterministic
   ordering regressions without live infrastructure.
8. The T003 schema keeps the legacy `zatca_submission` record and explicit
   outcome state. T007 should not conflate queue delivery state with provider
   outcome state; they have different owners and lifecycles.

## Decisions

### D1: Add a dedicated outbox table

Use an additive `zatca_event_outbox` table rather than embedding queue state in
`zatca_submission`. Bill, credit, debit, and onboarding events do not all map
to the existing submission row, and queue delivery state must remain distinct
from provider outcome state.

### D2: Store the durable event payload

Store the versioned event JSON plus normalized identity columns. The payload
allows retries after the original request and preserves onboarding OTP/event
contract details. The normalized columns support due-row lookup, tenant
auditing, and uniqueness.

### D3: Claim before publish, transition after outcome

The drain claims a due row in a short database transaction, publishes outside
that transaction, then marks it `published` or `failed` in a separate
transaction. This prevents a slow NATS call from holding database locks while
ensuring only claimed rows are processed.

### D4: Keep request paths transactional and synchronous only for DB work

Handlers enqueue through `qtx` after all business writes and before commit.
They do not wait for NATS. A post-commit wake-up may drain promptly, but the
durable row is the source of truth if publication is unavailable.

### D5: Reuse the existing event identity/idempotency contract

The outbox creates the event payload before commit and retries that exact
payload. The existing NATS subject and message ID behavior remain unchanged.
`SubmitDebit` remains available as a durable enqueue API even without a new
debit route.

## Alternatives Considered

1. **Publish after `tx.Commit` in each handler**: rejected because a process
   crash between commit and publish still loses the event and no retry state
   exists.
2. **Publish inside the transaction and roll back on error**: rejected because
   external NATS acknowledgement cannot participate in the database
   transaction and can leave duplicate/phantom messages.
3. **Use an in-memory retry queue**: rejected because restart loses pending
   events and publication failures are not durable.
4. **Reuse `zatca_submission` for queue status**: rejected because onboarding
   and debit events have different domain rows, and delivery state is not
   provider outcome state.
5. **Add a new daemon**: rejected by scope; the backend may drain the database
   outbox, while the existing downstream daemon remains the event consumer.

## Migration Concerns

- The outbox migration must run after the T003 schema migration and be safe on
  fresh and existing databases.
- Existing rows are not backfilled because no reliable event payload exists
  for historical business changes; only new mutations require an outbox row.
- The migration must preserve all existing `bill`, `credit_note`, `debit_note`,
  branch, and monetary columns.
- Deployment systems outside this repository must apply the new migration
  before enabling the updated handlers. Live MySQL execution is not assumed.

## Validation Evidence Planned

- Red: a handler sqlmock expects commit before any publication side effect and
  expects the outbox insert; base code fails due to pre-commit publisher call.
- Red: a publication failure test expects an outbox failure update; base code
  has no durable update and only logs.
- Green: focused handler/outbox tests, `sqlc generate`, formatting, build,
  vet, and `go test ./...`.
