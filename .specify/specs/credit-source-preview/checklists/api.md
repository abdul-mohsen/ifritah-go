# API Requirements Quality Checklist: Bill Credit-Source Preview

**Purpose**: Review completeness and clarity of API requirements before implementation  
**Created**: 2026-11-04  
**Feature**: [spec.md](../spec.md)

> Reviewer-owned checklist: `[x]` means the reviewer approves the requirement-quality criterion, not that implementation is complete. This checklist is intentionally generated unchecked.

## Requirement Completeness

- [ ] CHK001 Are authentication and bill-store authorization requirements defined for the preview? [Completeness, Requirements FR-1/FR-4]
- [ ] CHK002 Are missing, deleted, draft, fully credited, and unprovable-bill cases all represented in the refusal requirements? [Coverage, Requirements FR-4/FR-5/FR-6]
- [ ] CHK003 Are missing branch, VAT, source lines, and mismatched credit-history cases included as explicit refusal conditions? [Coverage, Requirements FR-5/FR-6]
- [ ] CHK004 Are read-only behavior, no reservation, and POST revalidation defined as separate compatibility requirements? [Completeness, Requirements FR-7/NFR-1]

## Requirement Clarity

- [ ] CHK005 Are the response fields and decimal-string precision unambiguous for bill ID, branch ID, amount, source-line ID, quantity, price, and VAT? [Clarity, Requirements FR-2]
- [ ] CHK006 Is “full remaining credit” clearly defined as deducting prior credited quantities per source line and prior note amounts from bill total? [Clarity, Requirements FR-3]
- [ ] CHK007 Are the source of accounting rules and the expected handling of rounding/discount allocation explicit enough to prevent frontend recomputation? [Clarity, Assumptions]

## Requirement Consistency

- [ ] CHK008 Are successful response requirements consistent with the fail-closed requirement that missing or unprovable values never become defaults? [Consistency, Requirements FR-2/FR-5/FR-6]
- [ ] CHK009 Are the preview's snapshot semantics consistent across the acceptance scenarios and the POST revalidation requirement? [Consistency, Requirements FR-7]

## Acceptance Criteria Quality

- [ ] CHK010 Can success be objectively judged by exact equality between returned amount and the full remaining source-line values? [Measurability, Acceptance AC-1/AC-2]
- [ ] CHK011 Are refusal outcomes required to contain an error and no usable credit payload for every invalid or unverifiable case? [Acceptance Criteria, AC-3/AC-4]

## Notes

- Resolve reviewer concerns in `requirements.md`/`spec.md`; this checklist does not verify code behavior.
- `/speckit-implement` reads checklist state but does not change reviewer-owned markers.
