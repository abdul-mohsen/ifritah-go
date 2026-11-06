# Requirements: Remove Legacy Database Adoption

## Problem

The backend still exposes a migration path for databases created before Goose
managed tenant schemas. The fresh-system-only policy requires tenant databases
to be created and migrated by the current migration set; the legacy adoption
path and its schema-digest machinery are no longer supported.

## Goals

- Remove the legacy/pre-Goose database adoption capability and all user-facing
  instructions for invoking it.
- Keep the tenant migration set numbered and contiguous from `00001` through
  `00010`.
- Preserve fresh-database migration behavior, tenant-account safety, and the
  existing rollback boundary at the baseline.
- Remove tests and generated/supporting data that exist only to validate
  adoption or compare legacy schemas.

## In Scope

- The migration package's adoption implementation, digest helpers, embedded
  digest data, and adoption-only tests.
- The `migrate adopt` command, command help, and command documentation.
- Comments and migration documentation that claim pre-Goose databases can be
  adopted.
- Focused migration tests and repository documentation needed to describe the
  fresh-system-only workflow.

## Out of Scope

- Renumbering, rewriting, or reordering migrations `00001` through `00010`.
- Changing the tenant schema, migration SQL, Goose provider configuration, or
  the existing `up`, `down`, `roundtrip`, `status`, and `version` behavior
  except where stale adoption wording must be removed.
- Changing tenant privileges or the tenant-account baseline replay check.
- Removing unrelated build/image digest terminology.
- Supporting upgrades from pre-Goose databases through another mechanism.

## User Scenarios and Acceptance Scenarios

### Scenario 1: Create a fresh tenant database

**Given** an empty existing tenant database, **when** an operator runs the
normal migration command, **then** the baseline and migrations through
`00010` are applied in order and the database reaches the latest version.

### Scenario 2: Inspect migration history

**Given** a database managed by Goose, **when** an operator runs status or
version, **then** the command reports the existing migration state without
exposing an adoption option.

### Scenario 3: Protect the baseline rollback boundary

**Given** a database at the baseline version, **when** an operator runs the
rollback command, **then** the command refuses to roll back below the baseline.

### Scenario 4: Replay the baseline with a tenant account

**Given** a tenant account with privileges only on its own database, **when**
the baseline is replayed, **then** it succeeds under the existing supported
privilege model.

## Functional Requirements

- **FR-001**: The migration command interface MUST NOT expose an `adopt`
  subcommand or invoke adoption logic.
- **FR-002**: The migration package MUST NOT contain the legacy adoption
  implementation, schema-digest comparison helpers, embedded baseline digest
  data, or adoption-only tests.
- **FR-003**: The embedded Goose migration files MUST remain exactly the
  contiguous versions `00001` through `00010`.
- **FR-004**: A fresh tenant database MUST still apply the migrations in order
  and reach the current target version.
- **FR-005**: The existing baseline rollback safety rule MUST remain: the
  baseline migration cannot be rolled back through the normal down command.
- **FR-006**: Existing tenant-account safety checks and baseline replay behavior
  MUST remain unchanged.
- **FR-007**: Migration documentation MUST describe the fresh-system-only
  workflow and MUST NOT instruct operators to adopt a pre-Goose database.
- **FR-008**: Unrelated uses of the word “digest” (for example, container
  image metadata) MUST remain unchanged.

## Non-Functional Requirements

- **NFR-001**: The change MUST compile and pass the repository's focused
  migration tests, full Go tests, build, vet, and formatting checks.
- **NFR-002**: The change MUST be limited to the requested backend worktree and
  MUST not perform pushes or GitHub writes.
- **NFR-003**: The change MUST preserve the existing public migration numbering
  and SQL behavior.

## Success Criteria

- **SC-001**: Search of migration code and documentation finds no
  `migrate adopt`, `Adopt`, or adoption-specific digest helper/reference.
- **SC-002**: Migration file validation confirms exactly ten contiguous files
  numbered `00001` through `00010`.
- **SC-003**: Fresh-system migration tests pass, including the existing
  baseline, rollback-boundary, tenant replay, and migration round-trip checks
  that are available in the repository.
- **SC-004**: `go test ./...`, `go build ./...`, `go vet ./...`, and `gofmt -l .`
  complete successfully (with no unformatted Go files).
- **SC-005**: A local commit contains only the issue's implementation,
  documentation, tests, and Spec Kit artifacts; no remote operation is made.

## Assumptions

- The current migration set is the source of truth for fresh tenant databases.
- Removing the adoption path is intentionally a breaking operational change
  for pre-Goose databases; no compatibility shim is required.
- Existing migration tests may skip MySQL-dependent scenarios when their DSN is
  absent; the validation report will distinguish skipped integration tests from
  failures.
- The root README is minimal; the migration README is the authoritative
  operator documentation for this change.

## Constraints

- Preserve the Goose numbering and migration SQL exactly unless a stale
  adoption comment must be removed.
- Preserve `BaselineVersion` and the existing refusal to roll back below it.
- Keep all work in `C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b5-atomic-sale`.
- Mark todo `removing-legacy-adoption` done only after validation and commit;
  mark it blocked with an explanation if required validation cannot be run.

## Risks

- Removing adoption-only files may leave stale references in command help,
  comments, CI documentation, or tests.
- A broad “digest” search can find unrelated image/build digests; only
  migration adoption digest machinery should be removed.
- Full integration tests may require an unavailable MySQL instance.

## Open Questions

- None. The issue explicitly requires fresh-system-only behavior and names the
  adoption implementation, command, digest helpers, tests, and docs to remove.
