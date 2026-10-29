# T014: Versioned ZATCA Event Idempotency Envelope

## Problem

The transactional outbox added by T007 persists and republishes the original
five-field ZATCA message. Its payload does not identify the durable outbox
record, the EGS, or the publication attempt, and direct publisher calls do not
provide a deterministic NATS message identity. A retry or process restart can
therefore be difficult to correlate and can produce duplicate downstream
delivery attempts without a stable message identity.

## Goals

- Publish a versioned ZATCA event envelope for every publisher path covered by
  the backend outbox.
- Carry enough tenant, branch, EGS, document, outbox, stable-key, and attempt
  context for downstream correlation and idempotent retry.
- Give each publication a deterministic NATS/JetStream message identity derived
  from the durable event identity.
- Allow the deployed daemon decoder that still reads the legacy top-level
  fields to continue processing events during migration.
- Keep the change limited to event payload and publisher identity behavior.

## In Scope

- Backend ZATCA publisher and outbox payload construction/publication.
- A versioned, JSON-encoded event envelope containing:
  tenant identity, branch identity, EGS identity, document type, source
  document ID, durable outbox ID, stable document key, and publication attempt
  metadata.
- Deterministic NATS message identity for first publication and retries.
- Compatibility fields required by the existing daemon decoder.
- Focused publisher regression tests covering envelope shape, compatibility,
  deterministic identity, and retry/attempt metadata.
- Backend documentation and feature artifacts for this change.

## Out of Scope

- Daemon consumer, decoder, reconciliation, or downstream business-state
  changes.
- Changes to invoice, credit-note, debit-note, onboarding, or submission
  business state.
- New database business tables or destructive migrations.
- NATS subject changes, provider-client changes, frontend changes, deployment
  changes, or pull requests.
- Replacing the existing daemon decoder in this backend task.

## User Scenarios

### Scenario 1: Versioned document publication

Given a bill, credit note, debit note, or onboarding event is enqueued, when
the backend publishes it, then the JSON payload includes a version marker and
the required tenant, branch, EGS, document, source, outbox, stable-key, and
attempt context.

### Scenario 2: Stable retry identity

Given the same durable outbox event is attempted more than once, when each
attempt is published, then NATS receives the same deterministic message
identity for that event rather than a random or time-based identity.

### Scenario 3: Legacy daemon coexistence

Given a daemon version that only decodes the existing five top-level fields,
when it receives the new envelope, then it can still read the document type,
document ID, branch ID, tenant/database name, and onboarding OTP exactly as
before.

### Scenario 4: Attempt observability

Given an outbox event is retried, when the backend publishes the event, then
the payload identifies the current attempt without mutating business state or
losing the stable document identity.

## Functional Requirements

### FR-1: Versioned envelope

Every backend ZATCA publication MUST use a versioned envelope. The version
MUST be explicit and distinguish the new contract from the legacy
five-field-only payload.

### FR-2: Required identity context

The envelope MUST carry tenant ID, branch ID, EGS ID, document type, source
document ID, durable outbox ID, and a stable document key. The values MUST be
derived from the existing authenticated/domain/outbox context and MUST NOT be
invented from untrusted client input.

### FR-3: Attempt metadata

The envelope MUST carry publication-attempt metadata sufficient to distinguish
the attempt number and its time/context. Retry attempts MUST retain the same
stable document key and durable event identity.

### FR-4: Deterministic NATS identity

The publisher MUST set the NATS/JetStream message identity from a stable,
durable event key. Re-publishing one outbox event MUST produce the same message
identity; different durable events MUST not intentionally share it.

### FR-5: Decoder compatibility

The new payload MUST retain the existing decoder's top-level compatibility
fields: document type, source document ID, branch ID, tenant/database name, and
optional onboarding OTP. Their meaning and subject derivation MUST remain
unchanged during migration.

### FR-6: No business-state change

The implementation MUST NOT add daemon consume/reconciliation behavior or
change business/submission state outside the event payload and publisher
identity.

### FR-7: Regression coverage

Focused publisher tests MUST fail against the T007 baseline because it emits a
legacy five-field/non-deterministic payload, and MUST pass after T014. Tests
MUST assert the required envelope fields, compatibility fields, deterministic
message identity, and retry stability.

## Non-Functional Requirements

- The existing NATS subject scheme MUST remain unchanged.
- The publisher MUST remain safe to use without a live NATS server in focused
  tests.
- JSON fields and message identity MUST be deterministic for equal durable
  input, except for explicitly varying attempt metadata where required.
- Existing T003/T007 tests and all repository tests MUST continue to pass.
- The implementation MUST be limited to this backend worktree and one
  isolated commit.

## Acceptance Criteria

1. A versioned envelope is emitted for bill, credit, debit, onboarding, and
   outbox-drained events.
2. The envelope contains tenant, branch, EGS, document type, source ID,
   outbox ID, stable document key, and attempt metadata.
3. The old daemon's five compatibility fields remain present and semantically
   equivalent.
4. The NATS publish call receives a deterministic message identity derived
   from durable event identity, including on retries.
5. Focused publisher regressions are red at commit `d4069f4` and green after
   implementation.
6. `sqlc generate`, `gofmt`, `go build ./...`, `go vet ./...`, focused tests,
   and `go test ./...` pass, with exact evidence recorded in `STATUS.md`.
7. No daemon, reconciliation, or business-state changes are present in the
   diff; only T014 backend code, tests, documentation, and feature artifacts
   are committed.

## Assumptions

- The existing outbox row ID is the durable outbox identity and is available
  when the drainer publishes a committed row.
- The existing branch ZATCA configuration's EGS serial/UUID is the source for
  the EGS identity when the backend has it available; if not, the existing
  branch/document context must provide a stable representation without a new
  business lookup.
- Existing daemon compatibility means the legacy fields remain top-level,
  rather than requiring a daemon rollout in this repository.
- A stable document key is the durable logical identity for a document event,
  while an attempt number may vary across retries.

## Constraints

- Base commit is `d4069f4` (T003+T007) on branch
  `fix/zatca/h2-event-idempotency-envelope`.
- Work only in `C:\ssda\chatGPT\worktrees\zatca-fix-t014-backend`.
- Do not push or create a pull request.
- Do not hand-edit generated sqlc output.
- Do not use live MySQL, NATS, or the daemon for focused regression tests.

## Risks and Unknowns

- The current T007 payload is built before the database assigns the outbox
  auto-increment ID, so the exact publication point for adding that ID must be
  resolved before design.
- The current backend schema exposes multiple EGS-related branch fields; the
  correct stable EGS identity and whether a lookup is already available must
  be confirmed from repository evidence.
- The currently deployed daemon decoder is outside this worktree; compatibility
  must be proven from the known legacy field contract and fixtures rather than
  by changing daemon code.
- Existing direct publisher APIs do not receive an outbox row or attempt
  number; their migration behavior must be defined without widening business
  scope.

## Open Questions

1. Which exact EGS field is the established daemon-facing identity?
2. Should outbox payload JSON be finalized before insert, or enriched only at
   committed publication time so the auto-increment row ID is available?
3. What attempt timestamp/number semantics are already represented by T007
   and should be exposed without changing persistence behavior?
