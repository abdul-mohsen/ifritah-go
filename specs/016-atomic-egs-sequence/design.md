# T016 Design

## Boundary

```text
posted bill transaction
  ├─ branch_sequence atomic increment
  ├─ bill insert/update
  ├─ additive chain reservation metadata
  └─ existing T014 outbox insert

daemon transaction
  ├─ reserve/get tenant-local branch+EGS chain row
  ├─ generate/sign XML outside the transaction
  ├─ commit resulting hash with owner token
  └─ release on a failed/non-committed attempt
```

## Data model

Add a migration-owned `zatca_egs_chain` table:

- `branch_id` — FK to `branches`, part of the primary key;
- `egs_id` — canonical EGS UUID/identity, part of the primary key;
- `last_icv` — last committed shared ICV;
- `previous_invoice_hash` — last committed PIH, nullable for the first
  document;
- `reservation_key` — stable T014 document key, nullable while unreserved;
- `reservation_token` — opaque owner token, nullable while unreserved;
- `reserved_icv` and `reserved_previous_hash` — values owned by the active
  reservation;
- `reserved_at`, `lease_expires_at`, `updated_at` — lifecycle timestamps.

The unique key on `(branch_id, egs_id, reservation_key)` is represented by
the primary row plus a lookup index; tenant isolation is the database
connection boundary and is documented in the contract.

## SQL contract

All statements are fixed, parameterized sqlc queries:

1. `ReserveZatcaEGSChain` (`:one`) starts from an existing row or creates a
   zero-state row, locks it with `FOR UPDATE`, and returns:
   tenant-local branch/EGS identity, the reserved ICV, the prior hash,
   reservation key, token, and lease.
2. `GetZatcaEGSChainForUpdate` (`:one`) reads the row under `FOR UPDATE` for a
   daemon transaction that already owns the row.
3. `CommitZatcaEGSChain` (`:execresult`) verifies branch, EGS, reservation
   key, owner token, and expected ICV before setting `last_icv` and
   `previous_invoice_hash`, then clears reservation fields.
4. `ReleaseZatcaEGSChain` (`:execresult`) verifies the same owner predicates
   and clears only reservation fields. It does not decrement the committed
   ICV.

Because sqlc cannot express a multi-statement reserve/lock/upsert procedure,
the generated query contract exposes primitive statements and the caller must
execute them in one transaction. The contract document gives the exact
ordering and retry rules. The Go backend helper uses `Queries.WithTx` for
business transactions; a daemon can consume the same SQL contract directly.

## Branch invoice allocation

Replace `getNextSquenceNumber` with a helper that calls the existing
`AllocateBranchSequence` query with:

```text
branch_id = request.BranchID
scope     = "bill"
```

The helper returns `LastInsertId()` and propagates errors. It is called only
inside the existing `AddBill`/`SubmitDraftBill` transaction, before the bill
insert. No global max query remains in posted-bill paths.

## Event envelope and outbox

T014's top-level compatibility fields, stable key, subject, event ID, and
attempt semantics remain unchanged. Chain reservation metadata is additive:
the backend may carry the reservation key/ICV/prior hash in the payload for a
future daemon, but publication still uses the existing outbox row and
`stable_document_key`/NATS message identity. No event is published before
the business transaction commits.

## Failure handling

- Allocation or reservation failure aborts the caller transaction and returns
  the existing handler error response.
- A bill insert failure rolls back the allocator increment in the same
  transaction.
- Commit/release affect zero rows on owner mismatch; the daemon must treat
  this as a lost reservation and not mutate chain state.
- Lease expiry is only a recovery eligibility check; it does not advance the
  chain or authorize a different tenant.
- MySQL deadlocks/timeouts remain retryable at the caller boundary; no
  process-local retry loop is introduced in T016.

## Security and tenant isolation

- No tenant/database/table identifier is interpolated into SQL.
- The tenant is the existing database connection selected for the tenant;
  branch and EGS are mandatory query arguments.
- Owner tokens and stable document keys are compared inside the SQL predicate,
  not trusted from a prior read.
- EGS identity is normalized before persistence; a missing onboarded UUID uses
  the documented deterministic branch fallback only.

## Test strategy

- Baseline red: run the existing bill outbox regression with the current
  global-max expectation after adding the new focused test; it must fail when
  the allocator query is absent.
- Branch isolation: sqlmock runs two transactions with different branch IDs
  and asserts `AllocateBranchSequence` receives each branch key and returns
  the same local starting value.
- Same-branch concurrency: parallel helper calls use separate sqlmock DB
  connections and assert unique allocator arguments/transaction outcomes.
- Chain ownership: sqlmock tests cover reserve, same-key idempotent recovery,
  owner mismatch on commit/release, and rollback.
- Compatibility: existing T014 envelope/outbox tests remain unchanged except
  for explicitly additive assertions.

## Rollout and rollback

Deploy migration before code paths that call the new contract. The invoice
allocator migration already exists in the repository; the T016 chain table
migration is additive. Rolling back application code leaves the tables
unused, but rolling back the migration is not safe after reservations exist;
use forward-compatible cleanup instead.

## Requirement traceability

| Requirement | Design element |
|---|---|
| FR-1 | Existing `branch_sequence` upsert and handler wiring |
| FR-2 | Tenant connection plus branch/EGS/owner predicates |
| FR-3 | One `zatca_egs_chain.last_icv` per branch/EGS |
| FR-4 | Reservation key/token and expected-ICV commit |
| FR-5 | InnoDB row lock and caller-owned transaction ordering |
| FR-6 | Stable document key idempotent reserve/get behavior |
| FR-7 | Additive T014 fields and unchanged outbox identity |
| FR-8 | Backend SQL contract only; no daemon/debit implementation |
| FR-9 | Focused sqlmock/concurrency regression suite |
