# Implementation Plan: Bill Credit-Source Preview

## Summary

Add one authenticated read-only endpoint for the full, provable remainder of a final bill's credit. Reuse the note writer's line/history queries and calculation helpers. Do not change the existing POST credit operation.

## Technical Context

- **Language/runtime:** Go; existing Gin HTTP API.
- **Persistence:** MySQL through sqlc-generated query methods.
- **Authentication/authorization:** Existing JWT middleware and bill-store access checks.
- **Data contract:** IDs as JSON integers; decimal amount/quantity/price/VAT as fixed-precision strings.
- **Testing:** Existing handler-package sqlmock patterns.
- **Schema changes:** None.
- **Unknowns:** None blocking. Error status for unprovable source follows local 409 conflict convention.

## Constitution Check

- The target worktree had no pre-existing Spec Kit constitution or Spec Kit initialization. No project-wide principles were available to load; this feature plan is scoped to the user's explicit compatibility, security, and worktree constraints.
- **Gate:** Pass. The design is additive, authenticated, read-only, fail-closed, and leaves POST credit authoritative.
- **Recheck after design:** Pass. No migration, new dependency, unrelated file, commit, or push is planned.

## Phase 0 — Research

- Confirmed route/auth group, existing store-access helper, bill state/branch/total query, note-line/history queries, accounting helpers, and sqlmock patterns.
- Resolved design decisions and alternatives in [research.md](research.md).

## Phase 1 — Design

- [Design](design.md) defines the handler flow, consistency, failures, tests, and requirement trace.
- [Data model](data-model.md) records the source bill, lines, history, and response fields.
- [API contract](contracts/credit-source.md) records the route and wire response.
- [Quickstart](quickstart.md) records focused validation commands.

## Implementation sequence

1. Add the GET handler that parses the ID, checks store access, reads bill state/branch and credit history in a no-write transaction, and proves remaining amount/lines.
2. Register the route in the authenticated group without changing the existing POST.
3. Add sqlmock tests for no history, partial history, fully credited, and unprovable/missing data/status.
4. Document the route and stale-preview semantics in the README.
5. Run gofmt, targeted tests, package tests, convergence, and self-review.

## Risks and mitigations

- **Stale preview:** Make explicit in the API docs; POST credit revalidates under its existing transaction.
- **Legacy or inconsistent rows:** Refuse instead of returning partial/defaulted values; compare the complete candidate line total with `creditRemaining`.
- **Concurrent credit write:** Use the same bill lock and transactional history reads as the writer.
- **Accounting drift from rounding/discount allocation:** Reuse `creditLines`/`checkNoteTotal`; reject if exact equality does not hold.

## Rollback

Remove the added GET route, handler, tests, and README section. No data migration or rollback of the existing credit POST is needed.
