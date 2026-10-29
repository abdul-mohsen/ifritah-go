# T024 Design: Durable Debit-Note Backend Contract

## Boundary

```text
authenticated POST /bill/debit
  └─ BEGIN
      ├─ lock/read parent bill under merchant predicate
      ├─ insert debit_note source + parent sequence/EGS context
      ├─ finalize source/document/stable identity
      ├─ register pending generic zatca_submission
      ├─ insert zatca_event_outbox payload
      └─ COMMIT
  └─ wake existing post-commit outbox drainer

outbox drainer
  ├─ claim committed debit row
  ├─ publish stable envelope/key outside DB transaction
  └─ record outbox publish outcome

later daemon (not T024)
  └─ consume source/EGS/sequence context and update generic submission outcome
```

## Data model

### `debit_note`

Add migration `0011_zatca_debit_note.sql` and the corresponding final schema
source table with:

- `id`, `bill_id`, `merchant_id`, and `branch_id` ownership identifiers;
- nullable parent `sequence_number` for the billing reference context;
- non-null `egs_id` selected from branch configuration or deterministic
  tenant/branch fallback;
- nullable `document_id` and `stable_document_key` finalized from the source
  ID in the same transaction;
- `state`, `note`, `created_at`;
- `invoice_uuid`, `invoice_hash`, `invoice_qr`, and `invoice_xml_path` for the
  later daemon contract;
- tenant/source, status, and branch/EGS indexes; one debit correction per
  merchant/parent bill; parent bill and branch foreign keys.

The source row is durable before event publication. Output fields remain
nullable until the later daemon processes the row.

### Generic submission source identity

Add migration `0012_zatca_submission_sources.sql` and final schema columns:

- `document_type` (`bill`, `credit`, or `debit`), defaulting historical rows
  to `bill`;
- `source_id`, backfilled to `bill_id`;
- nullable `sequence_number` and `egs_id`.

Retain `bill_id` as the parent bill reference used by compatibility joins.
Replace the bill-only unique key with
`(merchant_id, document_type, source_id)`. Existing document ID and tenant
uniqueness remain intact. Migration statements are replay-safe using the
repository's existing information-schema guards.

## SQL/query contracts

`pkg/db/queries/debit.sql` provides:

1. `GetDebitNoteSourceContext` — parent bill/branch/sequence/EGS lookup with
   `merchant_id` and tenant fallback arguments, executed in the caller's
   transaction with `FOR UPDATE`.
2. `InsertDebitNote` — inserts the authenticated source row.
3. `SetDebitNoteIdentity` — writes the source-ID-derived document and stable
   keys with merchant/source predicates.
4. `GetDebitNotePublicationContext` — reads only an authorized, fully
   identified source row for publisher enqueue.
5. `RegisterDebitZatcaSubmission` — idempotently creates a pending generic
   submission keyed by merchant/document type/source ID and retains parent,
   sequence, EGS, and stable document ID.

All statements use `?`/`sqlc.arg` parameters and no interpolated tenant or
table identifiers.

`zatca_monitor.sql` adds the generic source/sequence/EGS fields to stats
compatibility joins and detail rows while retaining existing merchant
predicates and route parameters.

## API contract

Mount `POST /api/v2/bill/debit` inside the existing JWT-authorized group.
Request:

```json
{"bill_id": 99, "note": "Additional taxable charge"}
```

The handler uses `GetSessionInfo(c).id`; no merchant, branch, EGS, sequence,
state, or provider fields are accepted from the request. It returns `201` and
the new debit source ID. Duplicate parent corrections return the existing
database conflict behavior. A failed source, submission, or outbox step rolls
back the complete transaction.

## Publisher/envelope contract

`EnqueueDebit` remains the public transactional publisher method, but it first
loads `GetDebitNotePublicationContext` from the passed `Queries` handle. It
builds the existing outbox payload with:

- `document_type=debit`;
- `source_id`/legacy `id` equal to `debit_note.id`;
- stable key `tenant:branch:debit:source_id`;
- persisted `egs_id`;
- optional `sequence_number` from the parent bill;
- existing event ID, tenant, branch, outbox, and attempt fields.

The direct `SubmitDebit` method remains for legacy callers but is not used by
the authenticated API. `publishOutboxRow` continues to source identity from
the durable outbox row and preserves additive envelope context from its
payload.

## Monitor/status contract

`ZatcaSubmissionRow` gains additive `document_type`, `source_id`,
`sequence_number`, `egs_id`, and `stable_document_key` fields. Existing
`invoice_id`, status/state, provider, retry, artifact, and timestamp fields
remain unchanged. The monitor list selects the generic submission source
fields; the pending row registered by the API is therefore visible before
daemon processing. Stats and branch aggregates count debit submissions through
the same explicit outcome state logic.

## Security and tenant isolation

- JWT middleware remains the only API authentication boundary.
- Parent source lookup and publisher context lookup require the authenticated
  merchant ID and expected branch.
- Query source IDs are never accepted as authorization for another merchant.
- Monitor queries retain both parent-bill and submission merchant predicates.
- Stable keys include tenant and branch; OTP and secrets are not added to the
  debit payload.

## Failure handling

- Missing/unauthorized parent, missing branch/EGS context, duplicate source,
  identity finalization, pending submission, or outbox errors roll back the
  transaction and return the existing handler error class.
- Outbox publication failure remains durable in the existing outbox status
  columns; no debit-specific pre-commit publish path is added.
- A later daemon can update pending submission state/attempt/outcome rows
  without changing the source identity.

## Test strategy

- Add a source/API regression that expects the parent lookup, debit insert,
  identity update, pending submission insert, outbox insert, commit, and no
  pre-commit NATS call. It fails at `b602c05` because the route/query/schema
  do not exist.
- Add tenant rejection and duplicate-source assertions.
- Add publisher regression that verifies the caller transaction, query context,
  stable key, sequence, and EGS fields in the outbox payload.
- Add monitor sqlmock regression that scans a debit submission row and asserts
  additive document/source/context fields while preserving tenant args.
- Keep all existing envelope/outbox/monitor tests green after `sqlc generate`.

## Rollout/rollback

Apply migrations before enabling the endpoint or daemon consumers. `0011` is
additive. `0012` backfills historical submission source identity and removes
only the obsolete bill-only uniqueness constraint after backfill. Application
rollback leaves unused additive columns/tables and is safe; migration rollback
must be forward cleanup because debit/submission rows may exist.

## Requirement traceability

| Requirement | Design element |
|---|---|
| FR-1 | `debit_note` migration/schema and insert/context queries |
| FR-2 | JWT route plus merchant predicates in source/publisher queries |
| FR-3 | source-ID-derived document/stable keys and outbox identity |
| FR-4 | parent sequence plus persisted/onboarded/fallback EGS ID |
| FR-5 | one handler transaction and existing post-commit drainer |
| FR-6 | generic `document_type`/`source_id` submission identity |
| FR-7 | additive monitor fields and unchanged routes/legacy fields |
| FR-8 | schema/migrations/sqlc query source, generated output regenerated |
| FR-9 | no daemon/import/frontend changes and focused regression scope |
