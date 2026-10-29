# Feature Specification: Transactional and Durable ZATCA Queue Publication

**Feature Branch**: `fix/zatca/c4-transactional-outbox`
**Created**: 2026-10-29
**Status**: Ready for implementation
**Input**: T007 transactional outbox request

## User Scenarios & Testing

### User Story 1 - Never lose a committed ZATCA request (Priority: P1)

As an operator, I need a bill, credit, debit, or onboarding request to remain
queued when NATS is unavailable so that committed business actions are
eventually processed.

**Independent Test**: Commit a business mutation while the publisher is
unavailable, then inspect the durable queue row and retry it later.

### User Story 2 - Avoid phantom downstream work (Priority: P1)

As a business owner, I need downstream ZATCA work to reference only committed
business rows.

**Independent Test**: Force a business transaction rollback and verify no
publication occurs.

### User Story 3 - Preserve tenant and authorization boundaries (Priority: P1)

As a tenant administrator, I need durable events to retain the same authenticated
tenant, branch, document, and authorization boundaries as the request.

**Independent Test**: Enqueue events from separate authenticated contexts and
verify identity and branch fields remain distinct.

## Functional Requirements

- **FR-001**: Business mutations and outbox events commit atomically.
- **FR-002**: Publication occurs only after commit.
- **FR-003**: Publication success and failure are durably recorded.
- **FR-004**: Due failures retry with the original event identity and payload.
- **FR-005**: Existing event types, subjects, and onboarding OTP behavior remain
  compatible.
- **FR-006**: Existing authentication, tenant, store, and branch boundaries
  remain enforced.
- **FR-007**: Migration is additive and preserves historical business data.

## Key Entities

- **Outbox event**: durable queue record tied to a business action.
- **Publication attempt**: one claim/publish outcome with retry metadata.
- **Existing business row**: bill, credit/debit note, or onboarding domain
  configuration that must commit with its outbox event.

## Success Criteria

- **SC-001**: Zero in-scope handler paths publish before their business commit.
- **SC-002**: Every committed in-scope request has one durable event row.
- **SC-003**: A failed publication remains visible with error and next retry
  metadata after the request returns.
- **SC-004**: A later drain publishes the same event without a new HTTP request.
- **SC-005**: Focused baseline regressions fail before the change and pass after
  it; repository validation commands pass afterward.

## Assumptions

- The existing database name is the event-contract tenant identity.
- The authenticated session ID remains the merchant ownership key.
- No historical outbox backfill is possible without reconstructing event
  payloads, so only new mutations are enqueued.

## Out of Scope

- Frontend, daemon, deployment, provider, or PR changes.
