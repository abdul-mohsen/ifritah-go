# Implementation Plan: Deterministic EGS Reservation Contract Test

## Summary

Add the missing EGS Goose contract test to the requested branch in a form that
matches the current daemon `SQLChain` lease protocol. Use a fixed clock, an explicit
expired timestamp, and a process-qualified fresh database. Update only the directly
related migration ownership comment and migration-contract assertion.

## Constitution check

- No project-wide Spec Kit constitution is present in this repository.
- The scoped plan is compatible with the user's requirements: test-only behavioral
  correction, no production lease weakening, no arbitrary sleeps, isolated worktree,
  local validation, and no push.
- Gate: pass.

## Implementation sequence

1. Add `pkg/db/goose/egs_chain_reservation_test.go` from the existing #105 contract
   test shape, replacing wall-clock time with a fixed UTC clock, qualifying the test
   database name by process, and checking lease liveness before different-owner
   rejection.
2. Update the `00005` migration comment to name `queue.SQLChain` as the runtime owner
   and add the migration test that protects that boundary.
3. Format changed Go files and run the focused Goose/EGS tests.
4. Run the Goose package tests and relevant repository Go checks; record any MySQL
   DSN limitation.
5. Run convergence/self-review, update status and chat evidence, update the session
   todo, and create the local commit.

## Risks and mitigations

- **Wrong owner semantics:** Compare the helper with the current daemon
  `queue.SQLChain.Book`; do not restore the removed backend same-document-only rule.
- **Timestamp conversion:** Use UTC whole-second values that fit MySQL `DATETIME` and
  pass the same explicit `now` through every helper call.
- **Cross-process database collision:** Include the test process ID in only this
  contract test's database name.
- **Missing MySQL:** Run compile/package tests without the DSN and record the skipped
  real-database validation as an environment limitation.

## Rollback

Remove the added contract test, migration comment/assertion, and scoped Spec Kit
artifacts. No schema or runtime rollback is required.
