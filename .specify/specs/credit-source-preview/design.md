# Design: Bill Credit-Source Preview

## Technical context

- The backend is a Go/Gin API backed by MySQL and sqlc-generated queries.
- The target worktree already contains the authoritative credit-note calculations and history queries.
- No migration or generated SQL change is required.
- The feature is additive; credit creation, stock restoration, and note-event behavior remain unchanged.

## Interface

Add authenticated `GET /api/v2/bill/{id}/credit-source`.

Successful response:

```json
{
  "bill_id": 42,
  "branch_id": 7,
  "amount": "28.75",
  "lines": [
    {
      "bill_product_id": 5,
      "quantity": "1.000",
      "price": "10.00",
      "vat": "15.00"
    }
  ]
}
```

IDs are JSON integers. Amount, quantity, price, and VAT are fixed-precision decimal strings (two places for monetary/VAT values; three for quantity). The endpoint follows existing error conventions: invalid ID and store-access refusal are client errors, missing/deleted bill is 404, and draft/full/unprovable source is 409. Database failures remain 500.

## Data flow and calculation

1. Parse a positive bill ID.
2. Call `checkNoteStoreAccess` before exposing bill state or data.
3. Begin a transaction and read `LockBillForNote` so bill state, branch, store, and total are available under the same bill lock used by credit creation.
4. Reject deleted state, draft state, or absent/zero branch.
5. In that transaction, read `ListBillLinesForNote`, `ListCreditedLinesForBill`, and `ListCreditAmountsForBill`.
6. Require at least one source line; reject invalid/duplicate bill-line IDs, nonpositive source quantities, and missing VAT. Aggregate prior credited quantities by bill-product ID; reject nonpositive credited quantities, unknown line IDs, or quantities exceeding the bill line.
7. Reject invalid prior credit amounts, then compute the amount using `creditRemaining(bill.Total, amounts)`. A nonpositive amount means no credit remains.
8. For each bill line with positive remaining quantity, validate the values accepted by credit creation and build a `model.CreditNoteLine` using the persisted price and VAT.
9. Pass all candidate lines and the aggregate credited quantities to `creditLines`. Require `checkNoteTotal(remainingAmount, lines)` to succeed. A mismatch means the full remaining credit cannot be proven and is refused.
10. Return the validated amount and lines. Roll back the read-only transaction to release locks; no rows are written.

The existing POST still rechecks availability transactionally. A preview is not a reservation and can become stale as soon as the GET transaction ends.

## Components and boundaries

- `main.go`: registers the route in the existing authenticated route group.
- `pkg/handlers/credit.go`: owns ID parsing, store/final-state checks, source calculation, failure responses, and serialization.
- `pkg/handlers/credit_source_test.go`: focused sqlmock handler coverage.
- `README.md`: documents the new route and preview caveat.
- Existing note queries and helper implementations remain the single source of accounting rules.

## Failure handling

- Missing/deleted bill: 404.
- Draft bill: 409.
- Missing/invalid branch, VAT, lines, fully credited bill, invalid credit history, or any mismatch between remaining amount and remaining line totals: 409 with an error message and no success payload.
- Unauthorized store: preserve `checkBillStoreAccess`'s existing refusal.
- Query/transaction failure: 500; no partial response is written.
- No field is defaulted when its persisted source is absent or invalid.

## Security, consistency, and performance

- The route is behind the same JWT middleware as neighboring bill routes.
- Store access is checked before the transactional source data is read.
- The bill lock serializes the preview's history read against the existing credit writer, whose transaction locks the same bill first. History rows are read within that transaction.
- The GET does not insert, update, or reserve business data. The transaction is short and only exists for a consistent preview.
- Work is bounded by the bill's existing lines and credit history; no new query pattern or external dependency is added.

## Test strategy

- Assert the response shape and fixed decimal strings for a bill with no prior credits.
- Assert exact remaining quantities and amount after partial history.
- Assert full-credit refusal.
- Assert refusal for draft/deleted/missing bill, unauthorized store, missing branch/VAT/lines, and unprovable history.
- Run focused handler tests, then the handlers package tests; format all changed Go files.

## Rollout and rollback

- No schema or migration changes.
- The route is additive and can be rolled back by removing its route registration and handler/tests/docs; existing POST credit behavior and persisted data are unaffected.

## Requirement traceability

| Requirement | Design response |
|---|---|
| FR-1, FR-2 | Authenticated GET contract and stable decimal-string response. |
| FR-3 | Existing line/amount queries and helpers calculate the complete remaining value. |
| FR-4 | Store authorization and explicit bill state checks. |
| FR-5, FR-6 | Validate all source/history data; refuse when the complete amount cannot be reconciled. |
| FR-7 | No writes; transaction is a preview snapshot and POST revalidates. |
| FR-8 | POST implementation and contract are untouched. |
| NFR-1 to NFR-4 | Transaction contains only reads; existing authorization/errors are reused; change scope is limited to handler, route, tests, and docs. |
