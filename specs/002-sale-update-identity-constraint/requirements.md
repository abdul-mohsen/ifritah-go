# Sale Update Identity-Constraint Classification Requirements

## Problem

The sale update path currently maps every duplicate database error to the
request-identity race response. An unrelated unique-constraint failure can
therefore be reported as a generic identity-conflict `409`, hiding the actual
database failure and giving callers the wrong diagnosis.

## Goals and scope

- Classify only the database constraint that enforces sale request identity as
  an identity conflict.
- Preserve the existing identity-conflict response for a genuine request
  identity race.
- Preserve the existing generic database-error response for unrelated unique
  constraints and other database failures.
- Add focused regression coverage for both branches.

## Out of scope

- Changing sale update transaction semantics, request-identity generation, or
  database schema.
- Changing responses for validation, authorization, stock, numbering, outbox,
  or other sale paths.
- Addressing any reviewer finding other than finding #3 for PR ifritah-go #105.
- GitHub operations, commits, pushes, or changes outside the designated
  worktree.

## User story

As an API caller updating a sale, I need a real request-identity conflict to be
reported as a conflict while unrelated uniqueness failures retain their actual
database-error behavior.

## Functional requirements

- FR-001: The sale update path recognizes the actual request-identity
  constraint as an identity conflict.
- FR-002: A duplicate error for an unrelated unique constraint is not classified
  as a request-identity conflict.
- FR-003: Existing identity-conflict behavior remains unchanged for a true
  request-identity conflict.
- FR-004: Regression tests exercise both the true identity-conflict case and an
  unrelated unique-constraint case.

## Non-functional and compatibility requirements

- NFR-001: The change is surgical and limited to the sale update duplicate-error
  classification and its focused tests.
- NFR-002: Existing database error wrapping and response behavior remain
  compatible except for the corrected classification of unrelated unique
  errors.
- NFR-003: Tests must not require GitHub access or external services.

## Acceptance criteria

- AC-001: A true request-identity duplicate produces the existing identity
  conflict response.
- AC-002: An unrelated unique-constraint error does not produce the identity
  conflict response and retains the generic database-error path.
- AC-003: Focused sale-handler tests pass for both cases.
- AC-004: Relevant backend tests, formatting, and diff checks pass.
- AC-005: No source changes address other reviewer findings; no GitHub write,
  commit, or push is performed.

## Assumptions, constraints, risks, and unknowns

- Assumption: The repository already has a stable database constraint or error
  identifier for sale request identity; research must locate the authoritative
  name and current error representation.
- Constraint: Work only in
  `C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b5-atomic-sale`.
- Risk: Matching only a vendor-specific error string could be brittle if the
  repository already exposes a safer constraint identifier; research must use
  existing repository conventions.
- Unknown: The exact update handler, database helper, constraint name, and test
  seam must be confirmed from source and existing tests before design.
- Open question: Whether the existing test suite uses a SQL mock, a database
  fixture, or another error-injection mechanism for this path.
