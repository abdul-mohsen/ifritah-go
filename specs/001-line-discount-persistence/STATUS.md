# STATUS

## Completed

- Requirements, user-facing spec, quality checklist, research, design, plan, and
  dependency-ordered tasks are ready and consistent.
- T1: Confirmed the target worktree is clean on `feat/atomic-sale` at
  `5d067c452335f103252bb44425758aed31639cae`.
- Repository evidence confirms the existing line-discount column and accounting
  formula are sufficient; no migration is needed.
- Confirmed sqlc v1.30.0 is available.
- T2: Added the discount placeholder to the existing insert and ran sqlc generation.
- T3: Bound `BillProduct.Discount` in the handler and added a focused SQL argument
  regression test.
- T4: `rtk sqlc generate` and `rtk gofmt -w` completed successfully; the focused
  handler test plus two existing sale-write tests passed (3 tests, 1 package).
- `rtk gofmt -l` reported no unformatted changed Go files; `rtk git diff --check`
  passed.
- T5: Self-review confirmed the query/handler/test cover AC-001/AC-002, the
  existing generated totals and bill aggregation are unchanged (AC-003), and the
  work remains scoped to this worktree (AC-005).
- The root README contains only a title, so no user-facing README guidance applies
  to this internal persistence fix.

## In Progress

- None.

## Blocked

- None.

## Backlog

- None.

## Risks

- The generated insert and handler must bind the decimal discount in the same column
  position; the focused mock will assert the exact argument.
- Previously saved lines whose discounts were omitted are not repaired by this
  change; historical backfill is out of scope.
- The repository ignores `pkg/db/gen/*`; CI regenerates sqlc output, and the local
  generated code was used to compile and run the focused tests.

## Next Actions

- No implementation work remains. Parent coordinates QA and PR updates; no commit
  or push has been made.
