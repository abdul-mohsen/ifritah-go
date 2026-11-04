# Feature Specification: Line-Discount Persistence

**Feature Branch**: `feat/atomic-sale`

**Created**: 2026-11-04

**Status**: Draft

**Input**: User description: "Implement the minimal backend #105 fix for the line-discount persistence blocker. Preserve the existing accounting formula, add focused regression coverage, regenerate sqlc output, and run gofmt and focused Go tests."

## User Scenarios & Testing

### User Story 1 - Preserve a Sale Line Discount (Priority: P1)

As a cashier recording a sale, I need the discount entered for each line to be
preserved so that the saved bill reflects the sale I submitted.

**Why this priority**: An omitted line discount makes saved accounting values
inaccurate.

**Independent Test**: Record a sale with a non-zero discount on one line and verify
that the saved line retains that amount and the bill reflects the existing
calculation.

**Acceptance Scenarios**:

1. **Given** a sale line with a non-zero discount, **When** the sale is saved,
   **Then** the saved line retains the submitted discount and totals use the
   established accounting rules.
2. **Given** a sale line with a zero discount, **When** the sale is saved,
   **Then** the saved line remains a zero-discount line.

## Edge Cases

- A sale contains multiple lines with different discounts; each saved line must
  retain its own submitted value.
- A sale contains both catalog and manually entered lines; each line's discount
  must remain associated with that line.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST preserve each sale line's submitted discount on the
  corresponding saved line.
- **FR-002**: The system MUST preserve a non-zero line discount rather than
  substituting zero.
- **FR-003**: The system MUST continue to calculate line and bill totals using the
  established accounting rules.
- **FR-004**: The system MUST continue to support zero-discount lines.

### Key Entities

- **Sale**: A bill and its submitted product lines.
- **Sale line**: A catalog or manually entered item with its own price, quantity,
  and discount.

## Success Criteria

### Measurable Outcomes

- **SC-001**: For a saved sale containing non-zero and zero-discount lines, every
  saved line retains the amount submitted for that line.
- **SC-002**: Saved bill totals continue to match the established accounting rules
  for the submitted line values.
- **SC-003**: Existing zero-discount sale scenarios remain successful.

## Assumptions

- The sale request already carries a discount for each line.
- The established accounting calculation remains authoritative; this change only
  needs to preserve the value entered for a line.
- Bill-level discount behavior and purchase-bill behavior are outside this feature.
