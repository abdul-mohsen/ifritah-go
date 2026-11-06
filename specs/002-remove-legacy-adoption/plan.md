# Implementation Plan: Remove Legacy Database Adoption

## Technical Context

- **Language/runtime**: Go 1.25 module with a Go 1.26.5 toolchain.
- **Migration system**: Goose v3.24.3 with embedded SQL files under
  `pkg/db/goose`.
- **Database**: MySQL tenant databases; integration checks use a configured
  MySQL DSN when available.
- **CLI**: `cmd/migrate` accepts one command argument and owns the supported
  migration operations.
- **Constraints**: Keep migration versions `00001` through `00010`,
  `BaselineVersion = 1`, the baseline rollback guard, and tenant replay
  safety. Do not change unrelated image/build digest code.
- **Testing**: Focused migration package tests, then repository-wide Go tests,
  build, vet, and formatting checks.
- **Unknowns**: None unresolved after repository research.

## Constitution Check

No project constitution is present under `.specify/memory/constitution.md`.
The change therefore follows the repository and task constraints directly:

- Fresh-system-only policy is explicit and is preserved.
- No migration SQL or numbering changes are planned.
- Tenant privilege and rollback safety checks remain.
- No remote writes or GitHub operations are performed.

**Gate**: PASS.

## Phase 0: Research

Completed in `research.md`. The adoption surface is isolated to the CLI,
adoption implementation/tests, digest helper/data, and stale documentation.
No replacement compatibility mechanism is required.

## Phase 1: Design

Completed in `design.md`, `data-model.md`, and `quickstart.md`. This is a
behavior-removal change with no new persisted entities or external API
contracts.

## Implementation Waves

### Wave 1 - Remove unsupported surface

- Delete adoption implementation, digest helpers/data, and adoption-only tests.
- Remove the CLI command branch, helper, usage text, and command comment.
- Rewrite migration documentation/comments to describe fresh-system-only
  operation.

### Wave 2 - Validate preserved behavior

- Run focused migration tests.
- Run full test, build, vet, and formatting checks.
- Search for stale adoption references and verify migration filenames.
- Inspect the diff for SQL, numbering, and tenant safety preservation.

### Wave 3 - Converge and commit

- Run the convergence checklist against requirements/design/tasks.
- Update status and the todo row.
- Commit locally with the required co-author trailer.

## Rollback

If review finds an unintended migration or tenant-safety change, restore the
affected file from the local branch before committing. The deleted adoption
surface is intentionally not restored unless the requirements change.

## Requirement Traceability

| Requirement | Plan coverage |
|---|---|
| FR-001/FR-002 | Wave 1 removes the command and adoption-only artifacts. |
| FR-003 | Wave 2 validates the ten contiguous migration files. |
| FR-004/FR-005 | Wave 2 runs migration tests and checks `BaselineVersion`. |
| FR-006 | Wave 2 preserves and validates tenant replay coverage. |
| FR-007/FR-008 | Wave 1 rewrites migration docs only and leaves unrelated digests. |
| NFR-001 | Wave 2 runs all requested Go checks. |
| NFR-002/NFR-003 | All waves remain local and avoid SQL/remote changes. |
