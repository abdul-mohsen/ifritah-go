# ifritah-go

## Database contracts

The backend owns the ZATCA submission persistence contract. The durable
submission, provider outcome, attempt, and artifact schema is defined in
`pkg/db/schema/schema.sql` and the replay-safe migration
`pkg/db/migrations/0008_zatca_submission_state.sql`. Monitor SQL lives in
`pkg/db/queries/zatca_monitor.sql`.

Regenerate the ignored sqlc output before building or testing:

```powershell
rtk sqlc generate
```

Generated files under `pkg/db/gen` are pipeline output and must not be edited
by hand.

## ZATCA event delivery

Bill, credit, debit, and onboarding handlers write a durable
`zatca_event_outbox` row in the same database transaction as the business
change. The backend publisher drains committed rows after the transaction,
reuses the stored event payload/idempotency key, and records publish failures
with a retry time and bounded error message. The additive migration is
`pkg/db/migrations/0009_zatca_event_outbox.sql`; historical business rows are
not backfilled.

The outbox drainer is started by `main.go`. It requires the migration before
new handlers are deployed, but tests can exercise enqueue and retry behavior
with sqlmock without a live MySQL, NATS, or ZATCA daemon.

Published outbox events use the additive versioned envelope documented in
`specs/014-zatca-event-idempotency-envelope/contracts/zatca-event-envelope.md`.
It carries tenant, branch, EGS, source-document, outbox, stable-key, and
attempt metadata while retaining the legacy top-level `doc_type`, `id`,
`branch_id`, `db_name`, and onboarding `otp` fields for deployed daemon
decoders. The stable document key is supplied as JetStream `Nats-Msg-Id`, so
retries of one outbox event keep the same NATS identity and subject.

## ZATCA sequence and EGS chain ownership

Posted invoice numbers are allocated by the tenant-local
`branch_sequence(branch_id, scope)` counter inside the existing business
transaction. The backend no longer derives a posted invoice number from a
tenant-wide `MAX(bill.sequence_number)`, so branches can allocate their own
monotonic sequence concurrently.

The additive `0010_zatca_egs_chain.sql` migration defines the daemon-facing
`zatca_egs_chain` contract. A row is keyed by `(branch_id, egs_id)` and owns
the shared invoice/credit/debit ICV and previous-invoice-hash chain. Daemon
reserve, commit, and release statements must use the documented owner token
and row-lock ordering in
`specs/016-atomic-egs-sequence/contracts/zatca-chain.md`.

Focused sqlmock tests cover query ordering, branch isolation, owner predicates,
and transaction behavior. They cannot prove live InnoDB lock, isolation,
deadlock, or lease-expiry behavior; validate those against live MySQL before
production rollout.