# Research: Bill Credit-Source Preview

## Repository evidence

- `main.go` registers the credit routes in the authenticated `authorized` group. `GET credit_bill/:id` is the credit-note detail route; `GET bill/:id` is ordinary bill detail; `POST bill/credit` is the existing authoritative writer.
- `pkg/handlers/credit.go` checks source-bill store access before opening a transaction. The writer locks the bill, verifies its state and request branch, then checks source lines, prior credited quantities, prior note amounts, and the final amount.
- `pkg/handlers/note.go` defines the accounting behavior:
  - `creditLines` verifies each source line, price, VAT, and available quantity, then allocates the bill-line discount proportionally and calculates the line total with database-compatible rounding.
  - `checkNoteTotal` verifies an amount against the calculated line totals.
  - `creditRemaining` subtracts recorded note amounts from the bill total.
  - `lockNoteBill` treats state below zero as deleted/not found, state zero as draft, and a positive state as final.
- `pkg/db/queries/note.sql` provides `GetBillStoreID` and `LockBillForNote`; the latter returns bill state, branch, store, and total. `LockBillForNote` does not filter deleted/draft rows, so the handler must check state explicitly.
- `pkg/db/queries/credit.sql` provides `ListBillLinesForNote`, `ListCreditedLinesForBill`, and `ListCreditAmountsForBill`. The two credit-history queries use `FOR UPDATE`, matching the writer's transactional read pattern.
- Store authorization is centralized in `checkNoteStoreAccess` / `checkBillStoreAccess`. The latter compares the requested store to the caller's store IDs.
- `pkg/handlers/credit_test.go` and `sale_test.go` provide sqlmock setup and expectations for store access, bill state, bill lines, and credit history.
- Root `README.md` contains only the repository title and no existing API reference section.
- The target worktree was clean before feature artifacts were added. It had no pre-existing `.specify` configuration, constitution, feature directory, or extension hook configuration.

## Resolved unknowns and decisions

| Requirement | Finding / decision |
|---|---|
| FR-1, FR-4 | Add the source route to the existing authenticated route group. Reuse `checkNoteStoreAccess`; this preserves the existing caller/store authorization policy. |
| FR-4, FR-5 | Read the bill row through `LockBillForNote` and explicitly reject deleted, draft, or branch-less bills. Positive state follows existing credit behavior. |
| FR-2, FR-3, FR-5 | Use the existing bill-line and credit-history queries. Aggregate credited quantity per source line; reject nonpositive/unknown credited rows and over-credited quantities. Compute remaining amount with `creditRemaining`. |
| FR-3, FR-5 | Build one candidate line per source line with a positive remainder and pass the candidates through `creditLines`. Require `checkNoteTotal(remainingAmount, lines)` to succeed; otherwise the full remainder cannot be proven and must be refused. |
| FR-5 | Reject empty bill lines, missing VAT, invalid line identifiers/quantities, missing branch, invalid credit amounts, and mismatched credit-history references rather than omitting or defaulting data. |
| FR-7 | Run the existing locking reads in a transaction without writes so a concurrent credit writer cannot change the bill/history between the preview reads. The transaction is released after the GET; a later POST still revalidates authoritatively. |
| FR-2 | Serialize `amount`, `quantity`, `price`, and `vat` as fixed-precision decimal strings: amount/price/VAT to two places and quantity to three places. IDs remain JSON integers. |
| AC-5 | Extend the existing handlers package tests using its sqlmock conventions; no schema or generated-query change is needed. |
| Documentation | Add a short endpoint description to the otherwise title-only README, including the preview/non-reservation caveat. |

## Alternatives considered

- **Reusing `GET /api/v2/bill/:id`**: rejected because the returned bill detail does not expose bill-product IDs and VAT needed to construct a credit request.
- **Reusing `GET /api/v2/credit_bill/:id`**: rejected because it returns a credit-note detail, not the source bill's remaining credit.
- **Adding a new SQL query or changing schema**: unnecessary; the existing note-lock and history queries expose the required bill and credit data.
- **Reading without a transaction**: rejected because the history queries use `FOR UPDATE`, and a concurrent credit writer could otherwise make the bill/line/history reads inconsistent.
- **Returning an empty or partially reconstructed result on bad history**: rejected by FR-5/FR-6; errors are safer than success-shaped defaults.

## Remaining uncertainty

- The contract does not prescribe HTTP error status codes for unprovable data. The implementation will follow local conventions: 404 for missing/deleted bills, 409 for draft/full/unprovable sources, and the existing store-access refusal for unauthorized callers.
- Preview contents may become stale after the transaction ends. This is an intentional property of a read-only preview; POST revalidation remains the correctness boundary.
