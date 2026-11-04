# Feature Specification: Bill Credit-Source Preview

## User Scenarios & Testing

### User Story 1 — Prepare a credit request from an eligible bill (Priority: P1)

An authenticated user with access to a final bill can retrieve the bill's remaining creditable amount and source lines, including each source line's ID, remaining quantity, price, and VAT.

**Why this priority**: The frontend needs these persisted values to construct a valid credit request without guessing or recomputing accounting data.

**Acceptance scenarios**:

1. **Given** a final bill with no prior credits, **When** an authorized user requests its credit source, **Then** the response contains the full amount and all creditable lines.
2. **Given** a final bill with partial credit history, **When** an authorized user requests its credit source, **Then** each line contains only its remaining quantity and the response amount is the exact remaining amount.
3. **Given** a bill with no remaining credit, **When** an authorized user requests its credit source, **Then** the request is refused and no credit payload is returned.

### User Story 2 — Refuse unavailable or unprovable credit sources (Priority: P1)

An authorized caller receives an error instead of misleading credit data if the bill is missing, deleted, still a draft, inaccessible, or lacks data needed to prove its remaining credit.

**Acceptance scenarios**:

1. **Given** a missing, deleted, or draft bill, **When** its credit source is requested, **Then** the request is refused.
2. **Given** a bill whose store the caller cannot access, **When** its credit source is requested, **Then** the request is refused.
3. **Given** missing branch, VAT, bill lines, or inconsistent credit history, **When** the source is requested, **Then** the request is refused without success-shaped defaults.

### User Story 3 — Keep the preview non-authoritative (Priority: P1)

An authorized user can use the preview to prepare a request, while the credit operation remains responsible for confirming that the bill still has the requested credit available.

**Acceptance scenarios**:

1. **Given** a successful preview, **When** another credit is recorded before the prepared request is submitted, **Then** the credit operation rechecks availability and may reject the stale request.
2. **Given** any preview request, **When** it completes, **Then** it has not changed bill, credit-note, stock, or accounting data.

## Requirements

- **FR-1:** The service provides an authenticated read-only means to retrieve a bill's credit source.
- **FR-2:** A successful response identifies the bill and branch, exact remaining amount, and source lines with bill-product ID, remaining quantity, price, and VAT.
- **FR-3:** The remaining amount and quantities reflect all prior credits and agree under the existing credit calculation rules.
- **FR-4:** The service refuses missing, deleted, or draft bills and callers without access to the bill's store.
- **FR-5:** The service refuses missing branch, VAT, or line data, a fully credited bill, and any history or arithmetic mismatch that prevents proving the remaining credit.
- **FR-6:** Error responses do not include fabricated values or a success-shaped credit payload.
- **FR-7:** The preview does not reserve or mutate credit. The credit-creation operation remains authoritative and revalidates availability when submitted.
- **FR-8:** Existing credit creation behavior remains compatible and unchanged.

## Success Criteria

- **SC-1:** For every eligible bill with provable history, the returned amount and quantities equal the full remaining credit calculated from persisted bill and note data.
- **SC-2:** For every refused bill or history condition, the caller receives an error with no apparently usable credit payload.
- **SC-3:** Preview requests do not change persisted business data, and a later credit request still revalidates availability.

## Key Entities

- **Bill**: Final bill with a store, branch, total, and persisted product lines.
- **Credit source line**: A bill product line with its remaining quantity and original price/VAT.
- **Credit history**: Previously recorded credit-note amounts and quantities attributed to bill lines.

## Assumptions

- Existing credit rules define the authoritative handling of price, discounts, VAT, rounding, prior credited quantities, and prior note amounts.
- Decimal values may be represented as strings to preserve exactness.
- Existing authentication and store-access rules apply to the preview.

## Constraints

- This feature is additive and does not change credit creation or unrelated bill behavior.
- A returned preview is a snapshot, not a reservation; the authoritative credit operation revalidates it.
