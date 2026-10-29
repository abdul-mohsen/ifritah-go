# Feature Specification: Durable ZATCA Submission State

**Feature Branch**: `fix/zatca/c2-submission-state-backend`  
**Created**: 2026-10-29  
**Status**: Draft for implementation  
**Input**: T003 backend persistence/monitor task

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reconcile provider outcomes (Priority: P1)

As an operations user, I need each invoice submission to show the provider's
actual outcome so that accepted, warning, rejected, ambiguous, and terminal
failure cases are not conflated.

**Why this priority**: An incorrect outcome directly affects compliance
follow-up and operational decisions.

**Independent Test**: Insert a provider response through the persistence
contract and read the monitor result; the returned state/code/warning matches
the provider response.

**Acceptance Scenarios**:

1. **Given** a provider response with no warning, **When** it is recorded,
   **Then** the monitor returns `accepted` and its provider code.
2. **Given** a provider response with warning data, **When** it is recorded,
   **Then** the monitor returns `accepted_with_warning` and warning details.
3. **Given** a provider response that rejects the document, **When** it is
   recorded, **Then** the monitor returns `rejected` and the provider code.

### User Story 2 - Preserve retry evidence (Priority: P1)

As an operations user, I need retry and ambiguous outcomes to remain visible
so that a transport failure is not mistaken for provider rejection and no
attempt evidence is lost.

**Independent Test**: Record two attempts for one stable document and read the
monitor/attempt data; both attempts and the retry scheduling fields remain
available.

**Acceptance Scenarios**:

1. **Given** a timed-out provider request, **When** it is recorded, **Then**
   the current state is `ambiguous`, the attempt count is retained, and the
   next retry time is queryable.
2. **Given** an unrecoverable local/provider error, **When** it is recorded,
   **Then** the current state is `terminal_failure` with its provider/error
   code.

### User Story 3 - Retain XML evidence (Priority: P1)

As an operator or auditor, I need request and cleared/response XML references
for each attempt so that an outcome can be investigated without changing the
document identity.

**Independent Test**: Record artifact metadata for a request and a cleared
response, then query it by submission/attempt and tenant.

### User Story 4 - Keep tenants isolated (Priority: P1)

As a tenant administrator, I need monitor results scoped to my tenant so that
another tenant's invoice or provider data cannot appear in my dashboard.

**Independent Test**: Store equivalent identities/provider references for two
tenants and execute the monitor query for one tenant; only that tenant's row is
returned.

## Edge Cases

- A provider returns no code or warning; nullable fields remain empty rather
  than receiving fabricated values.
- A submission has never been attempted; retry and provider outcome timestamps
  remain null while state is `pending`.
- A retry has no next scheduled time because the record is terminal.
- A legacy row has only a numeric status; the migration preserves it without
  inventing a provider outcome.
- A document has multiple artifacts of the same kind across attempts; each
  artifact remains attributable to its attempt.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST maintain a stable tenant-scoped document identity
  linked to the source bill.
- **FR-002**: The system MUST expose the states `pending`, `accepted`,
  `accepted_with_warning`, `rejected`, `ambiguous`, and `terminal_failure`.
- **FR-003**: The system MUST retain provider code/status, warning/rejection
  data, and outcome timestamps when supplied.
- **FR-004**: The system MUST retain attempts, retry count, last attempt time,
  and next retry time without replacing prior attempt evidence.
- **FR-005**: The system MUST retain request and cleared/response XML paths or
  equivalent artifact metadata tied to the relevant attempt.
- **FR-006**: Monitor reads MUST return explicit provider outcome data and
  enforce tenant scope.
- **FR-007**: Applying the change MUST preserve existing invoice totals and
  legacy submission rows.

### Non-Functional Requirements

- Tenant filters are mandatory for all document-level monitor reads.
- Time fields use the established database timestamp type and timezone
  behavior.
- Lookup indexes support tenant/document, state, provider code/reference, and
  retry time.
- Generated database contracts are reproducible through the repository
  pipeline.

## Key Entities

- **ZATCA submission**: the stable tenant-scoped document record and current
  provider outcome.
- **Submission attempt**: one provider request/response try, including retry
  and timing data.
- **Submission artifact**: metadata/reference for request, cleared, response,
  or related XML evidence.
- **Provider outcome**: the provider state/code/warning/rejection facts
  associated with the current submission.

## Success Criteria *(mandatory)*

- **SC-001**: The focused regression test fails at the base commit because the
  provider outcome/status is absent or wrong, and passes after the change.
- **SC-002**: 100% of monitor query paths include the requested tenant scope.
- **SC-003**: All six operational states are representable and distinguishable
  in a monitor result.
- **SC-004**: A document with at least two attempts retains both attempt
  records and their artifact references.
- **SC-005**: Existing invoice monetary values are unchanged by migration.

## Assumptions

- `bill.merchant_id` is the tenant key for current invoice data.
- The existing submission table remains the compatibility anchor.
- Artifact references are application-managed paths/metadata and may omit XML
  body content.

## Out of Scope

- Submission workers, daemons, outbox publishing, and provider client behavior.
- Frontend routes, components, and display changes.
- Invoice signing, calculation, posting, or data backfill.
