# Design: Backend #105 Line-Discount Persistence

## Summary

Carry the existing `BillProduct.Discount` value through the current sale-line write
operation. The line's discount column already exists, and the database's generated
line totals and bill aggregation already use it.

## Component boundaries and data flow

1. A sale request provides each line's `BillProduct.Discount`.
2. The handler's `addProductToBill` maps that decimal value into the generated
   `AddProductToBill` parameters.
3. The generated query inserts the value into the same bill-product row as the
   associated line.
4. Existing generated line totals and `RefreshBillTotals` consume the saved value
   without formula changes.

Append the discount column and placeholder to the existing insert so the established
argument order remains intact. Regenerate sqlc output; do not hand-edit generated
files.

## Persistence and migration

Use the existing `bill_product.discount` decimal column. No schema migration or
backfill is needed. New writes explicitly persist the supplied value; existing rows
and the database default retain their current behavior.

## Failure handling

The generated query returns insert errors through the existing handler path. Do not
change transaction boundaries, error responses, or rollback behavior.

## Security and performance

The change adds no new inputs, queries, or external effects. It binds one additional
decimal parameter to the existing insert; there is no material performance or
security impact.

## Test strategy

- Add a focused handler-package sqlmock test that invokes `addProductToBill` with a
  non-zero decimal discount and asserts the exact insert plus all bound arguments.
- Leave existing broad sale mocks and accounting queries unchanged unless required
  by the query signature.
- Run sqlc generation, gofmt on changed Go files, and focused handler tests.

## Rollout and rollback

Deploy as a normal backend code change; no database rollout step is required.
Previously saved lines whose discounts were omitted are not backfilled by this
change. Reverting the code restores the previous behavior for subsequent writes.

## Requirement traceability

| Design element | Requirements |
|---|---|
| Handler-to-query discount mapping and persistence | FR-001, FR-002, AC-001 |
| Existing decimal type and database column | NFR-001, NFR-002 |
| Unchanged generated totals and bill aggregation | FR-003, AC-003 |
| Zero-discount compatibility | FR-004 |
| Focused argument-binding regression | AC-002 |
| sqlc generation, gofmt, and tests | NFR-003, AC-004 |
