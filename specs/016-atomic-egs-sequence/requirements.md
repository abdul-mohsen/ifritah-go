# T016: Atomic Per-EGS Invoice Sequence and Hash Ownership

## Problem

Posted invoice creation currently derives the next invoice number with a
tenant-wide `MAX(bill.sequence_number)`. That read-then-write pattern is
incorrect for branches and can return duplicate or cross-branch numbers under
concurrency. The existing ZATCA branch configuration also stores the EGS
invoice counter value and previous invoice hash without a backend contract
that makes ownership and commit semantics explicit for the daemon.

## Goals

- Allocate posted invoice numbers from a branch-scoped, transactional counter.
- Define one tenant-local, branch/EGS-scoped contract for the shared ZATCA ICV
  and previous-invoice-hash chain state used by invoices, credit notes, and
  debit notes.
- Make reservation and completion ownership explicit so two daemon workers
  cannot consume the same branch/EGS chain state concurrently.
- Keep tenant isolation, the T014 event envelope, transactional outbox
  behavior, and sqlc-generated workflow intact.
- Add focused sqlmock and concurrency regressions that demonstrate the
  incorrect global-max behavior and verify branch/EGS scoping after the fix.

## In Scope

- SQL migration/schema for branch/EGS chain ownership and recovery metadata.
- sqlc queries for branch-scoped invoice allocation and daemon-consumable
  reserve/commit/release operations.
- Backend invoice creation wiring for the existing posted-bill paths.
- Additive T014 envelope fields needed to carry a durable chain reservation
  when the backend enqueues a document event.
- Focused handler/database regression tests and related documentation.
- T016 specification and validation artifacts.

## Out of Scope

- Daemon XML generation, signing, submission, retry, or hash-chain
  orchestration.
- Adding a debit-note domain/table or debit HTTP handler.
- Replacing or changing the existing outbox/event envelope identity,
  subject, idempotency, or publication behavior.
- Global sequence allocation for purchase bills, orders, or unrelated
  documents.
- Live MySQL/NATS/ZATCA integration in the focused test suite.

## User Scenarios

### Scenario 1: Branch-local invoice numbers

Given two branches in one tenant have existing invoice numbers, when each
branch creates posted invoices concurrently, then each branch receives the
next number in its own sequence and no branch observes the other branch's
maximum.

### Scenario 2: Shared EGS chain ownership

Given one tenant/branch/EGS has a current ICV and previous hash, when a daemon
worker reserves chain state for an invoice, credit note, or debit note, then
the reservation returns one ICV and prior hash and prevents another worker
from reserving the same active EGS chain until the owner commits or releases
it.

### Scenario 3: Commit and recovery

Given a valid reservation, when the owner commits the newly produced invoice
hash, then the state advances exactly once and the reservation is cleared.
When the owner releases the reservation or its lease expires, a later worker
can safely retry without accepting a stale owner token.

### Scenario 4: Outbox compatibility

Given a posted invoice is created, when the business transaction enqueues its
ZATCA event, then the existing T014 legacy aliases, stable identity, subject,
and outbox behavior remain unchanged; chain metadata is additive and durable.

## Functional Requirements

### FR-1: Branch-scoped invoice allocation

The backend MUST allocate a posted invoice sequence using a row keyed by
`branch_id` and a fixed invoice scope. Allocation MUST be atomic under
concurrent transactions and MUST NOT read a tenant-wide `MAX` as the source
of truth.

### FR-2: Tenant-safe ownership

Every chain reservation, completion, and release MUST be scoped by the
tenant's database connection and by `branch_id` plus the exact EGS identity.
No caller-supplied branch or owner token may authorize a different tenant,
branch, EGS, or reservation.

### FR-3: Shared ICV semantics

Invoices, credit notes, and debit notes MUST share one strictly increasing ICV
counter for the same tenant/branch/EGS. The backend MUST NOT allocate separate
independent ICV streams for those document kinds.

### FR-4: Prior-hash ownership

A successful reservation MUST return the prior hash associated with the
reserved ICV and an opaque owner token. Only that token, with the same
tenant/branch/EGS/document reservation identity, MAY commit the resulting
hash or release the reservation.

### FR-5: Transaction and lock semantics

Reservation state changes MUST use InnoDB row-level locking and transactions.
A transaction rollback MUST leave the counter, prior hash, and ownership state
unchanged. SQL MUST use fixed parameterized statements and no interpolated
tenant or branch identifiers.

### FR-6: Idempotent durable reservation

The same stable document key MUST be able to recover its existing reservation
without incrementing the ICV twice. A different document key MUST NOT claim an
active reservation owned by another document.

### FR-7: Event compatibility

Existing T014 envelope fields and outbox/event identity behavior MUST remain
wire-compatible. Any chain fields added to the envelope MUST be additive,
deterministic, and sourced from the transaction's durable reservation.

### FR-8: No daemon/domain expansion

This task MUST NOT implement daemon XML/hash-chain processing or create a
debit-note domain. It provides only the backend persistence/query contract
that a later daemon task can consume.

### FR-9: Regression coverage

Focused tests MUST fail against the current global-max implementation and
MUST pass after the branch allocator and chain ownership contract are wired.
Tests MUST cover branch isolation, concurrent allocator calls, reservation
ownership, idempotent retry, rollback/error paths, and T014 compatibility.

## Non-Functional Requirements

- The allocator MUST use database serialization rather than process-local
  mutexes.
- Queries MUST remain compatible with sqlc generation and MySQL 8/InnoDB.
- Existing handlers and outbox tests MUST retain their behavior except for the
  expected allocator query replacement.
- No generated sqlc output may be hand-edited or committed.
- The change MUST remain isolated to the T016 worktree and one local commit.
- The backend MUST document that focused tests do not prove live MySQL lock
  behavior; a live MySQL validation remains an operational limitation.

## Acceptance Criteria

1. Posted invoice allocation no longer calls `GetMaxSequenceNumber` and uses
   an atomic branch-scoped counter.
2. Concurrent sqlmock regression coverage proves branch A and branch B can
   allocate the same local number without a global-max collision, while two
   allocations for one branch advance monotonically.
3. A daemon-consumable SQL contract exists for reserve, commit, release, and
   idempotent lookup of tenant/branch/EGS chain state.
4. The shared ICV and prior hash are owned by one active document reservation
   and are updated only by the matching owner.
5. T014 envelope compatibility and transactional outbox behavior remain
   unchanged, with only additive chain metadata where required.
6. `sqlc generate`, `gofmt`, targeted red/green tests, `go build ./...`,
   `go vet ./...`, and `go test ./...` are run and recorded.
7. No daemon XML chain or debit domain code appears in the diff.
8. One isolated local commit is created; nothing is pushed and no PR is
   created.

## Assumptions

- Each tenant is served by its own database connection, so the database name
  is the tenant boundary; all T016 SQL runs on that tenant connection.
- `branch_zatca_config.csr_computer_number` is the established EGS UUID when
  present. A deterministic branch fallback is used only for pre-onboarding
  rows with no UUID.
- Existing `branch_sequence` is the migration-owned invoice counter table and
  remains the source of truth for the branch-local human invoice number.
- Existing `branch_zatca_config` fields remain available for daemon
  onboarding/config reads; T016 adds a dedicated ownership table rather than
  overloading the branch configuration row with transient reservation state.

## Risks and Unknowns

- Live MySQL behavior for `SELECT ... FOR UPDATE`, conditional lease recovery,
  and deadlock/retry timing cannot be proven by sqlmock alone.
- The deployed daemon may initially read the existing branch configuration
  fields directly; the contract document must state the new table/query
  shapes without changing daemon code in this task.
- Credit-note creation currently has no independent sequence column and debit
  storage is intentionally absent. T016 therefore defines shared EGS ICV
  ownership without adding either domain model.

## Observable Success Measures

- No production handler path uses a global `MAX(sequence_number)` to assign a
  posted invoice number.
- For one branch, N successful allocator calls produce N distinct consecutive
  values in commit order; for two branches, both may independently start at
  one.
- An owner mismatch cannot commit or release another document's reservation.
- Existing event envelope compatibility tests and outbox tests remain green.
