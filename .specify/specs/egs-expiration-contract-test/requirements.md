# Requirements: Deterministic EGS Reservation Contract Test

## Problem

`TestEGSReservationContract` has an expired-reservation takeover case that can fail in
the migration/chain contract test suite. The test must prove the intended lease and
owner rules without depending on wall-clock races, arbitrary sleeps, or state left by
another test process.

## Goals and scope

- Establish the failure mechanism for the expired-reservation takeover assertion.
- Make expiration, timing, and database state deterministic in the contract test.
- Preserve the production reservation protocol, including its owner and lease semantics.
- Keep the change limited to the EGS/goose contract test and directly related test
  setup/documentation.
- Run the focused contract tests and the relevant Go test suites.

## Out of scope

- Changing the production reservation protocol or migration schema unless the
  investigation proves an existing production defect.
- Replacing lease comparisons with arbitrary sleeps or broadening lease ownership.
- Changing unrelated bill, note, migration, or deployment behavior.
- Pushing branches, creating GitHub objects, or modifying another worktree.

## User story

As a maintainer of the EGS migration/chain contract, I want the expired-reservation
scenario to use controlled time and isolated database state so that the test reliably
checks the real lease contract rather than test timing or cross-test contamination.

## Functional requirements

- **FR-1:** The test deterministically represents a live reservation and an expired
  reservation using explicit test-controlled state.
- **FR-2:** The test asserts the intended behavior for a reservation whose lease has
  expired, including the correct document/token ownership rule.
- **FR-3:** The test does not alter production lease semantics to make the assertion pass.
- **FR-4:** Each contract-test run uses isolated database state and does not rely on
  rows or timestamps from another run.
- **FR-5:** The test continues to cover owner-guarded release/commit and preservation
  of the reserved ICV across the expiration scenario.

## Non-functional requirements

- **NFR-1:** No arbitrary sleep or polling delay is used to create expiration.
- **NFR-2:** Time comparisons have a stable precision and timezone representation
  appropriate for the MySQL `DATETIME` column.
- **NFR-3:** The fix is repeatable under the supported MySQL/goose test invocation.
- **NFR-4:** The diff is surgical and leaves unrelated worktree files unchanged.

## Acceptance criteria

- **AC-1:** The expired-reservation failure is reproduced or its mechanism is
  established from the current test/production history and recorded in research.
- **AC-2:** The contract test passes deterministically for the intended live and
  expired lease cases without a sleep.
- **AC-3:** Existing production owner/lease semantics remain represented by the test;
  an unrelated document cannot gain an ICV merely because a lease is expired when
  the protocol forbids that behavior.
- **AC-4:** Focused EGS/goose tests and the relevant Go tests pass, or any unavailable
  external MySQL prerequisite is explicitly recorded as a blocker.
- **AC-5:** A local commit is created in the requested worktree and its SHA is reported.
- **AC-6:** The session todo `fixing-egs-expiration-contract-test` is marked `done`
  only after the implementation and validation pass; otherwise it is marked
  `blocked`.

## Compatibility and success measures

- The migration schema and runtime lease contract are unchanged unless a confirmed
  production defect requires a narrowly scoped correction.
- Repeating the focused contract test does not depend on prior test rows, database
  contents, or elapsed wall-clock time.
- The final diff contains only the scoped feature artifacts and the deterministic
  test fix.

## Assumptions

- `GOOSE_PARITY_DSN` is the supported environment variable for the MySQL-backed
  contract test.
- The migration/chain contract is owned by the external SQLChain/EGS daemon after
  the backend chain package removal.
- The requested worktree is the only worktree this task may modify.

## Constraints

- Work only in
  `C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b6-notes-2`.
- Do not push or perform GitHub writes.
- Do not use `/tmp` or arbitrary sleeps.
- Preserve existing repository conventions and do not hand-edit generated `pkg/db/gen`
  code.

## Risks and unknowns

- The requested worktree may be behind the worktree that currently contains the
  contract test; the exact branch relationship must be resolved before editing.
- MySQL driver timezone/precision conversion may affect comparisons against
  `DATETIME`; research must distinguish a real driver issue from an incorrect test
  expectation.
- The test may intentionally encode an external-daemon rule that differs from the
  removed backend implementation; changing the assertion without evidence could
  weaken the contract.
