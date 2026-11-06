# Feature Specification: Remove Legacy Database Adoption

**Feature Branch**: `feat/atomic-sale`

**Created**: 2026-11-06

**Status**: Ready for implementation

**Input**: User request to resolve issue #123 by removing legacy/pre-Goose
database adoption support, including the adoption command, digest helpers,
tests, and documentation, while preserving Goose migrations `00001` through
`00010`, tenant safety, and migration behavior.

## User Scenarios & Testing

### User Story 1 - Provision a fresh tenant (Priority: P1)

As an operator, I need a fresh tenant database to be built by the current
migration set so that schema state is deterministic and no legacy schema is
silently adopted.

**Independent Test**: Run the migration command against an empty tenant
database and verify that all ten migrations apply in order.

### User Story 2 - Operate a managed tenant (Priority: P1)

As an operator, I need the existing migration status, version, rollback, and
round-trip commands to retain their current behavior after the unsupported
adoption path is removed.

**Independent Test**: Run the migration package tests and verify the baseline
rollback boundary and contiguous migration versions.

### User Story 3 - Use least-privilege tenant credentials (Priority: P1)

As a tenant database account, I need the baseline to remain replayable with
privileges limited to my own database.

**Independent Test**: Replay the baseline using the existing tenant-account CI
check and verify it succeeds without elevated global privileges.

## Acceptance Scenarios

1. **Given** an empty tenant database, **when** `migrate up` runs, **then**
   migrations `00001` through `00010` apply in order.
2. **Given** the migration CLI, **when** an operator reads its help or invokes
   an unsupported command, **then** no `adopt` command is offered or dispatched.
3. **Given** a database at version 1, **when** `migrate down` runs, **then** it
   refuses to roll back below the baseline.
4. **Given** a tenant account with only database-local privileges, **when**
   `00001_baseline.sql` is replayed, **then** the existing replay check succeeds.
5. **Given** the repository migration documentation, **when** an operator
   follows it, **then** it describes fresh-system migration and contains no
   pre-Goose adoption instructions.

## Requirements

### Functional Requirements

- **FR-001**: The migration command interface MUST NOT expose an `adopt`
  subcommand.
- **FR-002**: Adoption implementation, schema-digest helpers, embedded digest
  data, and adoption-only tests MUST be removed.
- **FR-003**: The Goose migration set MUST remain exactly `00001` through
  `00010`, in order.
- **FR-004**: Fresh-system migration behavior MUST remain unchanged.
- **FR-005**: The baseline rollback boundary MUST remain unchanged.
- **FR-006**: Existing tenant safety and least-privilege baseline replay MUST
  remain unchanged.
- **FR-007**: Migration documentation MUST describe the fresh-system-only
  workflow and omit adoption instructions.
- **FR-008**: Unrelated build/image digest behavior MUST remain unchanged.

### Non-Functional Requirements

- **NFR-001**: Focused migration tests, full tests, build, vet, and formatting
  validation MUST pass.
- **NFR-002**: No remote write or push may be performed.

## Edge Cases

- A pre-Goose database is no longer automatically recognized or modified.
- A database at the baseline remains protected from rollback.
- A tenant account must still replay the baseline without global privileges.
- Search terms such as “digest” may match unrelated image metadata and must
  not trigger unrelated edits.

## Assumptions

- Fresh tenant provisioning is the supported path after this change.
- No replacement compatibility mechanism is required.
- MySQL-dependent tests may skip without a configured DSN; skipped integration
  coverage will be reported separately from failures.

## Success Criteria

- **SC-001**: No migration code or operator documentation contains an
  adoption command, `Adopt` implementation, or adoption-only digest helper.
- **SC-002**: Migration validation confirms ten contiguous versions numbered
  `00001` through `00010`.
- **SC-003**: Fresh migration, baseline safety, tenant replay, and existing
  migration tests pass.
- **SC-004**: Required Go test, build, vet, and formatting checks pass.
- **SC-005**: A local commit records only this scoped change and no remote
  operation is performed.
