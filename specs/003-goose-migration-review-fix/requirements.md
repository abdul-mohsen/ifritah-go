# Goose migration review fix requirements

## Problem

The atomic-sale backend branch has reviewer findings around the tenant goose migration
contract: migration numbering and the `00008` bill-counter migration, adoption/backfill
behavior for existing databases, fresh-database behavior, and the destructive rollback of
`branch_sequence` bill counters. The fix must be compatible with databases that may already
have applied numbered migrations.

## Goals

- Preserve the applied migration version contract unless repository evidence proves that a
  renumbering is safe.
- Make bill-counter initialization for existing bills explicit, safe, and idempotent.
- Make fresh-database and existing-database/adoption behavior explicit rather than relying on
  silent guessing.
- Ensure rolling back the bill-counter migration does not delete unrelated or already-existing
  `branch_sequence` rows.
- Add focused regression coverage for the migration file contract and the database behavior.
- Establish one owner for the EGS ICV reservation columns introduced by migration 00005.

## In scope

- Numbered files in `pkg/db/goose`, the migration runner/adoption boundary, and focused migration
  tests/documentation needed to describe the supported contract.
- Existing migration-test helpers and MySQL parity tests when they are the established test
  mechanism.

## Out of scope

- Renumbering or rewriting already-applied migrations without concrete repository evidence.
- Changing sales allocation semantics beyond the migration's counter initialization/rollback
  contract.
- Changes to unrelated legacy migration trees, deployment infrastructure, or GitHub state.

## Functional requirements

1. **Version stability:** The migration set must retain the existing numbered versions, including
   any intentionally applied `00008`, unless source/history evidence demonstrates that no
   database could have applied the affected versions.
2. **Counter backfill:** Applying the bill-counter migration to an existing schema with bills must
   seed each branch's `bill` counter from its highest existing non-null bill number and must be
   safe to rerun/adopt without lowering a counter.
3. **Fresh database:** A fresh database must have an explicit supported path that applies the
   baseline and subsequent migrations in order.
4. **Existing database/adoption:** An existing database must have an explicit, contract-correct
   adoption path. The code must not silently infer that an arbitrary schema is equivalent to the
   baseline.
5. **Non-destructive rollback:** Rolling back the bill-counter migration must not delete
   pre-existing `branch_sequence` rows or rows belonging to other scopes; it may remove only
   rows that the migration can prove it created.
6. **Regression tests:** Tests must fail against the current destructive/ambiguous behavior and
   pass after the fix, covering version shape, backfill, fresh-vs-existing/adoption handling,
   and rollback preservation.
7. **EGS reservation ownership:** The four reservation columns and their all-or-none check must
   have exactly one documented owner after `pkg/egschain` removal. The backend must not retain
   stale comments or an undocumented second owner.
8. **EGS protocol coverage:** If the columns remain because the external daemon owns them, tests
   or executable fixtures must cover the SQL contract for concurrent booking, same-owner
   re-delivery, release, commit, and stale-owner rejection where backend-side MySQL fixtures are
   appropriate. If the columns are unused, a later-numbered safe removal migration must remove
   them without renumbering applied migrations.

## Non-functional requirements

- Use the smallest safe change and preserve already-applied numbering.
- Keep migration SQL compatible with the supported MySQL versions and tenant-account
  privileges documented by the repository.
- Keep the migration runner deterministic and fail closed on unsupported database state.
- Preserve existing behavior outside the reviewed migration contract.

## Acceptance criteria

- AC-1: Actual goose files and migration tests document/verify the final contiguous version
  contract without deleting an applied version.
- AC-2: Existing bill rows produce correct per-branch counters, including null branch/sequence
  rows and pre-existing higher counters, with idempotent behavior.
- AC-3: Fresh and existing/adopted database paths are explicit and covered by tests or a
  documented executable contract.
- AC-4: Down behavior preserves unrelated/pre-existing `branch_sequence` rows and is covered by
  a regression test.
- AC-5: Focused migration tests, repository tests/build/vet/gofmt available for this package pass.
- AC-6: A local commit is created, no remote/GitHub write occurs, and remaining risks are
  reported.

## Assumptions, constraints, risks, and unknowns

- **Assumptions:** The migration files in `pkg/db/goose` are the current source of truth for the
  goose path; `00001_baseline.sql` defines `branch_sequence`; MySQL parity tests may be skipped
  without `GOOSE_PARITY_DSN`.
- **Constraints:** Work is limited to
  `C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b5-atomic-sale`; user requested no push
  or GitHub writes.
- **Risks:** A migration already applied in deployed tenant databases cannot safely be renamed or
  renumbered; deleting an old migration file can make goose unable to reconcile those databases.
  SQL behavior may differ across supported MySQL versions. Removing an externally-owned column
  contract without inspecting the daemon could break invoice processing.
- **Unknowns to resolve in research:** Which numbered versions have been applied or referenced
  by repository history/deployment tooling; whether adoption is implemented outside `cmd/migrate`;
  whether rollback can identify rows created by `00008` without extra metadata; and which tests
  can run without a live MySQL DSN; whether a current daemon SQL implementation still reads or
  writes the 00005 reservation columns.
