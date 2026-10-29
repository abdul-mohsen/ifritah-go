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