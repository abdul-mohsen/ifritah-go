# T024: Durable Debit-Note Backend Contract

## Problem

The backend advertises a `SubmitDebit` publisher method and accepts debit as an
outbox document type, but it has no debit-note source table or authenticated
API path that creates a durable source row. A debit event therefore cannot
reliably carry its source identity, parent invoice sequence, branch/EGS
context, or a status row that the existing ZATCA monitor can display.

## Goals

- Persist one authenticated, tenant-scoped debit-note source row linked to its
  original bill.
- Give the row and its outbox event a stable identity that survives retries.
- Persist the parent invoice sequence and branch EGS identity needed by a
  later daemon without implementing daemon XML or submission processing.
- Create a monitor-compatible pending submission record and expose debit source
  identity/context and provider outcome fields through the existing monitor
  contract.
- Preserve the existing JWT/RBAC boundary, tenant predicates, event envelope,
  outbox behavior, sqlc generation, and generated SQL workflow.

## In Scope

- Additive debit-note schema/migrations and the checked-in schema source.
- sqlc queries for authenticated source lookup, debit-note creation,
  stable-identity finalization, publication context, and pending submission
  registration.
- An authenticated `POST /api/v2/bill/debit` endpoint matching the existing
  credit-note request shape (`bill_id`, `note`).
- Transactional debit outbox enqueue using the persisted source row rather than
  a publisher-only method.
- Additive event-envelope sequence context and existing EGS context.
- Generic ZATCA submission source identity fields and monitor query/response
  fields so debit outcomes are visible without weakening tenant filters.
- Focused red/green sqlmock and handler regressions, documentation, and Spec
  Kit artifacts.

## Out of Scope

- Daemon debit XML generation, signing, provider submission, retries, or
  processing.
- Debit-note line-item/import persistence, stock movements, accounting, or
  payment settlement.
- Frontend changes, new monitor routes, NATS/JetStream infrastructure changes,
  or changes to existing bill/credit business behavior.
- Live MySQL, NATS, or ZATCA integration in the focused suite.

## User Scenarios & Testing

### Scenario 1: Create an authenticated debit note

Given a logged-in tenant user and a bill owned by that tenant, when the user
submits a debit-note reason, the API atomically creates a debit source row,
assigns the parent bill sequence and branch EGS context, registers a pending
monitor submission, and enqueues one debit outbox event before committing.
No event is published before commit.

### Scenario 2: Reject cross-tenant access

Given a bill owned by another tenant, when the authenticated user submits a
debit note for that bill, the source lookup returns no authorized bill and the
transaction creates neither a debit row nor an outbox/submission row.

### Scenario 3: Stable retries

Given a committed debit source row, when its outbox event is drained or
retried, the event retains the same source ID, stable key, tenant, branch, EGS,
and sequence context. A duplicate source request for the same parent bill is
rejected rather than creating a second source row.

### Scenario 4: Monitor-visible outcome

Given a debit source row with a pending or provider-updated submission record,
when an authenticated monitor query runs for the owning tenant, the result
includes the debit document kind/source ID, sequence/EGS context, explicit
outcome state, and provider fields when present. Another tenant cannot see it.

### Regression baseline

Focused tests are first run against commit `b602c05` and must fail because
debit is only an advertised publisher method: there is no debit source schema,
authenticated debit API flow, generic submission source identity, or persisted
sequence/EGS context. The same tests pass after T024.

## Functional Requirements

### FR-1: Durable debit source

The system MUST provide a debit-note table keyed by an auto-increment source
ID, linked to a parent bill, with tenant, branch, parent sequence, EGS
identity, reason, lifecycle metadata, and daemon output metadata fields.

### FR-2: Tenant-safe API

The debit-note API MUST require the existing JWT middleware and MUST select the
parent bill using the authenticated merchant/tenant predicate. It MUST NOT
trust a caller-supplied merchant, branch, EGS, or sequence value.

### FR-3: Stable identity

Each committed debit source MUST have a non-empty stable document identity and
stable outbox idempotency key derived from the tenant, branch, debit document
kind, and source ID. Retries MUST reuse that identity.

### FR-4: Sequence and EGS context

The debit source and published envelope MUST carry the parent bill sequence
when available and the branch EGS identity. The EGS value MUST use the
onboarded branch identity when present and the existing deterministic
tenant/branch fallback otherwise.

### FR-5: Transactional publication

Debit source creation, stable identity finalization, pending submission
registration, and outbox insertion MUST occur in one caller-owned database
transaction. The NATS publisher MUST not be called by the request path before
commit.

### FR-6: Generic submission identity

The durable submission contract MUST distinguish bill, credit, and debit
documents using document kind and source ID while retaining the parent bill
reference for compatibility. It MUST allow one parent bill to have distinct
document-kind/source submissions.

### FR-7: Monitor contract

Existing monitor routes MUST retain their current response fields and add
document kind, source ID, sequence, EGS identity, and stable document ID as
additive fields. Stats, branch aggregates, detail rows, attempts, and
artifacts MUST continue to apply the authenticated tenant predicate.

### FR-8: Generated SQL

All database access introduced by T024 MUST be represented by source schema
and sqlc query files. Generated sqlc output MUST be regenerated and MUST NOT be
hand-edited.

### FR-9: Scope discipline

The change MUST not add daemon debit processing, XML generation, import
persistence, frontend code, or unrelated business changes.

## Non-Functional Requirements

- SQL must be fixed, parameterized, and compatible with MySQL 8 and the
  existing sqlc configuration.
- Tenant and source ownership predicates must be present in every API/publisher
  lookup and monitor query.
- Existing bill, credit, onboarding, outbox, envelope, and monitor behavior
  must remain wire-compatible except for additive debit/source fields.
- Focused tests must run without live MySQL, NATS, or ZATCA.
- Live MySQL limitations around migration execution, unique constraints,
  transaction isolation, and monitor joins must be reported explicitly.

## Acceptance Criteria

1. A replay-safe migration and schema source define the debit-note source row,
   parent linkage, tenant/branch/sequence/EGS context, stable identity, and
   daemon output metadata.
2. The authenticated debit API creates the source, pending submission, and
   outbox row atomically and rejects unauthorized or duplicate source requests.
3. `EnqueueDebit` loads and verifies durable source context from the caller's
   transaction and emits a stable event with source, sequence, EGS, and tenant
   context.
4. The generic submission schema and monitor SQL expose debit source identity
   and outcome fields without cross-tenant leakage.
5. Focused red regressions fail at `b602c05` and pass after implementation;
   existing outbox/envelope/monitor tests remain green.
6. `sqlc generate`, `gofmt`, build, vet, focused tests, and `go test ./...`
   are run, with any live MySQL limitation recorded.
7. No daemon debit XML/processing or import persistence appears in the diff.
8. One isolated local commit is created; nothing is pushed and no PR is
   created.

## Assumptions

- The existing `merchant_id` from JWT session is the tenant boundary used by
  backend APIs and monitor queries.
- The parent bill supplies branch and human invoice sequence; debit notes do
  not allocate a second human invoice-number stream.
- `branch_zatca_config.csr_computer_number` is the canonical EGS identity when
  populated; the existing tenant/branch fallback is valid before onboarding.
- One debit note per parent bill is sufficient for this bounded domain contract,
  matching the current credit-note uniqueness model.
- The monitor's current parent-bill join remains the compatibility anchor for
  debit submissions.

## Risks, Unknowns, and Open Questions

- Live migration behavior against populated production schemas and existing
  duplicate correction rows cannot be proven by sqlmock; deployment must run a
  migration rehearsal on a representative MySQL copy.
- The later daemon must be updated to write generic document-kind/source
  submission rows; T024 only supplies the backend contract and pending row.
- Existing historical `zatca_submission` rows may lack branch/EGS metadata;
  migration backfill uses nullable EGS values where no branch identity exists
  and monitor output preserves null rather than inventing provider data.
- The current daemon's debit implementation is intentionally not changed,
  including its XML and status-update behavior.

## Observable Success Measures

- A committed debit API request leaves exactly one source row, one pending
  generic submission identity, and one deduplicated outbox identity.
- The published debit payload contains the same stable key/source ID on every
  retry and includes sequence and EGS context.
- A monitor response can distinguish a debit source from its parent invoice and
  display its pending/provider outcome for the owning tenant only.
