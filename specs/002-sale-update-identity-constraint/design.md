# Design

## Boundary

Only the error decision in `writeSale` is changed. The generic `IsDuplicate`
helper remains unchanged because other handlers intentionally classify any
duplicate key according to their own endpoint behavior.

## Classification

Add a sale-specific predicate that:

1. unwraps the error with `errors.As` to find `*mysql.MySQLError`;
2. requires MySQL duplicate number `1062`; and
3. requires the MySQL message to identify the existing
   `uq_bill_pos_request` constraint.

`writeSale` uses this predicate instead of the generic duplicate predicate only
for the `UpdateBillByID` failure. A matching error keeps the existing rollback
and `answerLostRace` flow. A non-matching error keeps the existing
`500 Internal Server Error` flow with the original error.

## Data and failure flow

```text
UpdateBillByID
  -> sale request-identity duplicate?
       yes -> rollback -> request lookup -> existing replay/conflict response
       no  -> existing 500 response with original database error
```

No query, migration, transaction, schema, API envelope, or outbox behavior
changes.

## Test strategy

- Update the two existing race tests to inject the actual
  `uq_bill_pos_request` key.
- Add a handler test that injects a duplicate for a different unique key,
  expects the existing `500` database-error response, and a direct predicate
  test that distinguishes both key names. The handler test intentionally
  provides no post-rollback request lookup expectation; the corrected path
  must not attempt that lookup.
- Keep all test setup in the existing `sqlmock` sale-handler harness.

## Rollout and rollback

The change is source-compatible and requires no migration or configuration.
Rollback is a single code/test revert; the database schema is unaffected.

## Requirement traceability

- FR-001/FR-003: named-key predicate plus existing race tests.
- FR-002: non-matching duplicate remains on the existing generic error branch.
- FR-004: focused unrelated-key regression and updated true-race regressions.
- NFR-001/NFR-002: one sale-specific predicate; generic duplicate behavior and
  all other handlers remain unchanged.
