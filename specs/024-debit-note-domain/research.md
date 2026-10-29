# T024 Research

## Repository evidence

1. `pkg/handlers/zatca_outbox.go` already accepts `debit` as a document type
   and exposes `EnqueueDebit`, but the baseline has no `debit_note` table or
   authenticated debit handler. `SubmitDebit` publishes directly and therefore
   cannot establish a durable source transaction.
2. `pkg/db/migrations/0009_zatca_event_outbox.sql` owns stable event identity
   (`event_id`, `idempotency_key`) and stores the serialized envelope. The
   drainer publishes only after a committed outbox row and uses the stored
   key for JetStream identity.
3. `pkg/handlers/zatca_envelope.go` already carries tenant, branch, EGS,
   source, outbox, stable-key, and attempt fields. It has no sequence field;
   adding an optional `sequence_number` is wire-compatible with the existing
   daemon decoder.
4. `pkg/db/migrations/0010_zatca_egs_chain.sql` and the T016 contract define
   `branch_zatca_config.csr_computer_number` as the onboarded EGS identity and
   `<tenant>:branch:<branch>` as the existing deterministic fallback. The
   shared EGS ICV belongs to the daemon chain contract, not a new debit
   sequence.
5. `pkg/db/migrations/0008_zatca_submission_state.sql` and
   `pkg/db/queries/zatca_monitor.sql` currently key submissions by `bill_id`.
   The daemon outcome branch documents that credit/debit persistence requires
   a future document-kind/source-key extension. A debit submission therefore
   needs `document_type` and `source_id` while retaining `bill_id` as the
   parent compatibility reference.
6. `pkg/handlers/credit.go` is the established authenticated correction-note
   flow: bind `{bill_id,note}`, begin a transaction, insert the correction,
   resolve the parent branch, enqueue the outbox event with `Queries.WithTx`,
   commit, then wake the drainer. T024 mirrors this boundary without adding
   stock/import behavior.
7. `pkg/handlers/zatca_monitor.go` and `pkg/model/zatca_monitor.go` expose
   existing tenant-scoped monitor routes. Additive document/source/sequence/EGS
   fields can be returned without changing route names or existing JSON keys.
8. The daemon integration repository contains a `debit_note` table migration
   mirroring `credit_note`, but its current debit query is intentionally a
   placeholder. This confirms the backend table shape and reinforces that
   daemon XML/processing must remain out of T024.
9. The repository's sqlc generated directory is ignored and recreated by
   `sqlc generate`; query source and schema/migration files are authoritative.

## Decisions

### D1 — Parent bill remains the correction context

Debit creation reads the parent bill under the authenticated merchant
predicate. The parent supplies `branch_id` and `sequence_number`; no new
human invoice-number counter is allocated for a debit note.

### D2 — Persist source context before publication

The debit row stores merchant, branch, parent sequence, EGS identity, and
stable document fields. The handler finalizes the source-ID-derived identity
inside the same transaction before inserting the pending submission and
outbox event.

### D3 — Extend, do not replace, submission identity

`zatca_submission.bill_id` remains the parent compatibility link. Add
`document_type`, `source_id`, `sequence_number`, and nullable `egs_id`; backfill
historical rows as `bill`/`bill_id`. Replace the bill-only unique key with a
tenant/document-kind/source unique key so a parent can own distinct bill,
credit, and debit submissions.

### D4 — Reuse the versioned envelope and add sequence

The existing `egs_id`, event ID, stable key, and attempt fields are retained.
An optional `sequence_number` field is populated for debit events from the
persisted source context. Existing bill, credit, onboarding, and old daemon
decoders remain valid because the field is additive.

### D5 — Pending status is registered in the request transaction

The debit API creates a `zatca_submission` row with explicit `pending` state
and generic source identity. The later daemon may create attempts/outcomes and
update the same row. This makes a committed but not-yet-processed debit
visible to the monitor without implementing daemon processing.

### D6 — Preserve tenant/auth at every boundary

The source-context query requires `bill.merchant_id = authenticated merchant`.
The publisher query requires both debit and parent merchant predicates and the
expected branch. Monitor stats/list/attempt/artifact queries retain merchant
filters and use the parent bill only for compatibility joins.

## Alternatives considered

1. **Keep direct `SubmitDebit` only** — rejected because a NATS call before a
   database commit loses the source/outbox transaction and cannot support
   durable retry identity.
2. **Create a debit-specific monitor table** — rejected because it duplicates
   T003 attempt/outcome/artifact state and makes the existing monitor routes
   inconsistent. Generic source identity is the smaller additive contract.
3. **Use a second debit sequence counter** — rejected because T016 defines the
   parent bill number as the human reference and one shared EGS ICV chain for
   bill/credit/debit.
4. **Trust branch/sequence/EGS values from the HTTP request** — rejected
   because it permits cross-branch or cross-tenant identity injection.
5. **Update daemon code in this task** — rejected by the requested boundary;
   daemon debit XML/processing and import persistence remain later work.

## Remaining operational limitation

Sqlmock can prove tenant predicates, query arguments, transaction ordering,
source identity, and retry payload stability. It cannot prove MySQL online
DDL, unique-key migration behavior, InnoDB lock/isolation/deadlock behavior,
or provider/daemon outcome writes. A representative live MySQL migration and
daemon integration rehearsal remains required before production rollout.
