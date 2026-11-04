# Requirements: Bill Credit-Source Preview

## Problem

The frontend needs a read-only way to load the remaining creditable contents of a bill. Existing bill and credit-note detail routes do not provide the source bill-product IDs and VAT required to build a credit request.

## Goals and scope

- Provide an authenticated source-preview route for a bill.
- Return only the bill's provable remaining credit amount and line quantities, with each source bill line's ID, price, and VAT.
- Refuse the preview whenever bill eligibility, store access, source metadata, credit history, or remaining values cannot be established.
- Keep credit creation as the authoritative operation; the preview must not reserve or mutate credit.

## Out of scope

- Changing credit-note creation or its transaction/revalidation behavior.
- Modifying credit-note detail, bill-detail, sale, or debit-note behavior.
- Adding frontend changes, persistence, or a new crediting workflow.

## User story

As an authenticated user authorized for a bill's store, I want to load the bill's remaining creditable value and source lines so the frontend can prepare a valid credit request without guessing IDs or accounting values.

## Functional requirements

- **FR-1:** The service exposes an authenticated read-only route for a bill's credit source.
- **FR-2:** A successful response contains `bill_id`, `branch_id`, `amount`, and `lines`; each line contains `bill_product_id`, `quantity`, `price`, and `vat`. Decimal values have a stable, lossless JSON representation.
- **FR-3:** The response represents the full remaining credit only: prior credited quantities are deducted per bill line and prior credit-note amounts are deducted from the bill total.
- **FR-4:** The service refuses missing, deleted, draft, or otherwise ineligible bills and users without access to the bill's store.
- **FR-5:** The service refuses bills without a valid branch, VAT-bearing bill lines, or credit lines needed to prove a non-empty remaining credit.
- **FR-6:** The service refuses a fully credited bill or any inconsistent/ambiguous history that prevents proving the remaining amount and line quantities.
- **FR-7:** The route is a preview only. Credit creation remains authoritative and revalidates the request transactionally; preview success does not guarantee later credit creation.
- **FR-8:** Existing credit creation behavior and wire contract remain unchanged.

## Non-functional requirements

- **NFR-1:** The route is read-only and performs no writes or reservation.
- **NFR-2:** Existing authentication and store-authorization conventions are preserved.
- **NFR-3:** Errors do not return success-shaped data or fabricated values.
- **NFR-4:** Implementation remains limited to the requested backend worktree and directly related endpoint, tests, and API documentation.

## Acceptance criteria

- **AC-1:** A bill with no prior credits returns its exact source lines and full creditable amount.
- **AC-2:** A partially credited bill returns exact remaining per-line quantities and remaining amount.
- **AC-3:** A fully credited bill is refused.
- **AC-4:** Missing, draft/deleted, unauthorized, or unverifiable bill metadata/status is refused without a success response.
- **AC-5:** Focused handler tests use the repository's SQL-mock conventions and prove the cases above.
- **AC-6:** Formatting and relevant Go tests pass; the existing credit POST handler is unchanged.

## Compatibility and success measures

- The endpoint is additive; existing routes and POST `/api/v2/bill/credit` keep their current behavior.
- For every accepted preview, all returned fields are derived from persisted bill and note data, and the sum of remaining line values agrees with the remaining amount under the existing credit calculation rules.
- For every rejected case, the response is an error and contains no apparently usable credit payload.

## Assumptions

- Existing note queries and helper logic define the authoritative interpretation of line quantities, discounts, VAT, and amounts.
- Existing authentication middleware supplies the authenticated user and store-access rules used by credit-note creation.
- Decimal strings are an acceptable stable representation for monetary and quantity fields.

## Constraints

- Target only `C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b6-notes-2`, branch `feat/b6-credit-debit-2`.
- Do not commit or push, do not edit the sale backend b5 worktree, and do not touch unrelated files.
- The GET result is a snapshot; concurrent credit creation may change availability before a later POST.

## Risks and unknowns (research resolved)

- The existing bill lock, store guard, history queries, accounting helpers, and sqlmock patterns satisfy the data and authorization needs; see `research.md`.
- The contract does not prescribe error status codes. The implementation uses local conventions: 404 for missing/deleted, 409 for draft/full/unprovable, and the existing store-access refusal for unauthorized callers.
- Preview data may become stale after the GET transaction ends. This is intentional; the existing POST remains authoritative and revalidates transactionally.
- No blocking unknowns remain.
