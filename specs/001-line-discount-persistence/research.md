# Research: Backend #105 Line-Discount Persistence

## Evidence and findings

| Requirement | Repository evidence | Finding |
|---|---|---|
| FR-001, FR-002 | `pkg/model/bill.go:72-80` | `BillProduct.Discount` is a `decimal.Decimal` request field with JSON name `discount` and required validation. |
| FR-001, FR-002 | `pkg/handlers/bill.go:196-212` | `addProductToBill` writes each sale line but omits `Discount` when constructing `db.AddProductToBillParams`. |
| FR-001, FR-002 | `pkg/db/queries/bill.sql:162-163` | `AddProductToBill` inserts name, product ID, price, quantity, bill ID, and part name, but omits the existing discount column. |
| FR-003 | `pkg/db/goose/00001_baseline.sql:415-430` | `bill_product.discount` is `DECIMAL(12,2) NOT NULL DEFAULT 0.00`; generated totals already subtract it from the line's pre-discount total and derive VAT and inclusive total from that result. |
| FR-003 | `pkg/db/queries/bill.sql:168-175` | `RefreshBillTotals` aggregates generated line totals and sums line discounts; its formula does not need to change. |
| NFR-001, NFR-003 | `sqlc.yaml` | Generated Go code is emitted to `pkg/db/gen`; `*.discount` is mapped to `shopspring/decimal.Decimal`. |
| AC-002 | `pkg/handlers/sale_test.go:140-144` | Existing sale-line SQL mock checks the insert call but not its arguments, so it cannot detect an omitted or zero discount. |

## Resolved unknowns

- The narrowest focused regression location is the handler package: call
  `addProductToBill` with a non-zero line discount and require the expected SQL
  statement and arguments in sqlmock. This covers the request-model-to-generated
  query binding that caused the blocker without changing unrelated sale mocks.
- `sqlc` is available as v1.30.0, so generation can be run directly.
- The repository ignores `pkg/db/gen/*`, and its Go and validation workflows run
  `sqlc generate`; generated output is therefore regenerated locally for validation
  rather than force-added to Git.
- The bill-product column and formula already meet the persistence and accounting
  requirements; no schema migration or accounting change is needed.
- The root `README.md` contains only its project heading and no relevant sale/API
  behavior guidance. No user-facing documentation change applies to this internal
  persistence correction.

## Alternatives considered

- Changing the bill-total formula or adding a column migration was rejected: the
  schema already stores the line discount and calculates totals from it.
- Relying on the existing sale-line mock was rejected: it does not assert bound
  arguments and would pass while the discount is omitted.
- A database integration test was not selected as the sole regression check because
  the handler's missing argument is the failing boundary and the focused handler
  mock can assert that bound value deterministically.

## Remaining uncertainty

None affecting scope or design. The focused test will verify the generated insert
statement and its non-zero discount argument.
