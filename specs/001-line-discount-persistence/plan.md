# Implementation Plan: Backend #105 Line-Discount Persistence

## Goal

Persist each sale line's existing discount input without changing the sale contract,
bill-product schema, or accounting formulas.

## Work sequence

1. **Extend the existing insert**: add the `discount` column and placeholder to
   `AddProductToBill`, then run sqlc generation for the target worktree.
2. **Map the input value**: pass `BillProduct.Discount` from `addProductToBill` to
   the generated parameter.
3. **Prove the binding**: add a focused handler-package sqlmock test with a
   non-zero discount and exact expected arguments.
4. **Validate and converge**: run gofmt and focused handler tests; inspect generated
   output, formula changes, and the final worktree diff.

These are sequential because generated parameters depend on the query and the
handler/test depend on the generated signature. No schema migration is planned.

## File boundaries

- Editable source: `pkg/db/queries/bill.sql`, `pkg/handlers/bill.go`, and one
  focused handler test file.
- Generated only by sqlc: `pkg/db/gen/bill.sql.go`.
- Do not change schema migrations, generated model formulas, existing sale mocks,
  or unrelated worktrees.

## Validation commands

- From the target worktree: `rtk sqlc generate` (using its `sqlc.yaml`).
- `rtk gofmt -w` on the changed handler/test Go files.
- `rtk go test ./pkg/handlers -run 'TestAddProductToBillPersistsLineDiscount|TestAddBillWritesTheSaleAndItsZatcaEventInOneTransaction|TestAddBillSavesADraftWithoutANumberOrAnEvent'`.
- `rtk git diff --check` and inspect status/diff to confirm scope and unchanged
  accounting formulas.

## Requirement traceability

- Persistence path: FR-001, FR-002, AC-001.
- Formula and compatibility: FR-003, FR-004, NFR-001, NFR-002, AC-003.
- Regression and tool validation: AC-002, AC-004.
- Worktree/branch constraints: AC-005.
