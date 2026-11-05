# Sale Update Identity-Constraint Classification Specification

## Context

Sale finalization writes `(pos_id, bill_request_id)` under a database unique
constraint. Two concurrent requests can both pass the pre-write lookup; the
losing update must use the existing replay/conflict handling. Other unique
constraints are unrelated to that race and must remain database failures.

## Required behavior

1. A sale update error is treated as a lost request-identity race only when it
   is a duplicate violation of the sale request-identity constraint.
2. The existing replay behavior remains available when the winning request can
   be read after rollback.
3. The existing duplicate response remains available when no winning request
   can be read.
4. A duplicate violation for another key follows the existing internal
   database-error response and does not perform a request-identity replay
   lookup.

## Verification

- A focused handler regression injects a duplicate for an unrelated key and
  verifies the generic `500` database-error response rather than a conflict.
- Existing race tests inject the named request-identity key and verify replay
  and conflict behavior.
- Focused handler tests and the backend test suite pass.
