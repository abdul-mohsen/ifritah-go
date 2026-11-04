# Backend #105 Line-Discount Persistence Requirements

## Problem

Sale request lines can include a discount, but the saved line and the bill totals do
not consistently reflect a non-zero line discount. This causes persisted accounting
values to diverge from the submitted sale.

## Goals and scope

- Preserve each submitted sale line's discount as that line's persisted discount.
- Ensure bill totals continue to derive from the existing line-total accounting
  columns and discount aggregation.
- Preserve behavior for lines whose discount is zero.

## Out of scope

- Changing bill-level discounts, purchase-bill discounts, or request validation.
- Changing the existing accounting formula or the `bill_product` schema.
- Altering sale transaction, stock, numbering, or outbox behavior.

## User story

As a cashier recording a sale, I need each line's submitted discount to be saved so
the persisted bill totals and discount amount represent the sale I entered.

## Functional requirements

- FR-001: A sale line's submitted discount is persisted on that same bill-product line.
- FR-002: A non-zero line discount is not replaced by the database default of zero.
- FR-003: The existing generated line-total formula and bill-total aggregation remain
  unchanged.
- FR-004: Zero-discount lines remain supported.

## Non-functional and compatibility requirements

- NFR-001: Keep the existing decimal representation and precision of line discounts.
- NFR-002: Keep existing sale API and transaction behavior; no migration is required
  if the current `bill_product.discount` column is sufficient.
- NFR-003: Generated sqlc files must be produced by the repository's sqlc generator.

## Acceptance criteria

- AC-001: The sale write path carries a non-zero `BillProduct.Discount` to the stored
  bill-product discount value.
- AC-002: A focused regression test fails if the non-zero discount is absent from the
  bound insert arguments.
- AC-003: Existing bill-product total formula and bill aggregation query are unchanged.
- AC-004: sqlc generation, gofmt, and focused Go tests complete successfully.
- AC-005: Only this worktree is changed; no commit or push is made.

## Assumptions, constraints, risks, and unknowns

- Assumption: The existing `bill_product.discount` column and its generated total
  columns are the accounting source of truth.
- Constraint: Work only in the clean `feat/atomic-sale` worktree at the supplied base
  SHA. Do not edit other worktrees, commit, or push.
- Risk: A missing or misordered SQL argument could silently persist zero or attach
  the value to the wrong field; the regression test must check the bound value.
- Research resolved the narrowest handler-test location and confirmed sqlc v1.30.0
  is available; no unresolved unknowns remain.
- Open questions: None; the request defines the desired behavior and explicitly
  preserves the current accounting formula.
